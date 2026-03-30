package exiftool

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"
)

const boundary = "1854673209"

// processStub and cmdStub mimic a small part of [os.Process] for tests that simulate kill/release.
type processStub struct {
	server *Server
}

func (p *processStub) Kill() error {
	if p.server.stdinW != nil {
		return p.server.stdinW.Close()
	}
	return nil
}

func (p *processStub) Release() error {
	return nil
}

type cmdStub struct {
	Process *processStub
}

// Server keeps one ExifTool instance (embedded zeroperl WASM module) alive and
// exchanges commands using ExifTool's -stay_open protocol. [Server.Command] may be
// called from multiple goroutines; commands are serialized internally.
type Server struct {
	srvMtx     sync.Mutex
	cmdMtx     sync.Mutex
	runtime    wazero.Runtime
	compiled   wazero.CompiledModule
	rootFS     fs.FS
	args       []string
	done       bool
	cmd        *cmdStub
	restartErr error

	mod      api.Module
	stdin    printer
	stdout   *bufio.Scanner
	stderr   *bufio.Scanner
	stdinW   io.WriteCloser
	stdoutR  io.Closer
	stderrR  io.Closer
	evalDone chan struct{}
	evalErr  error
}

// NewServer compiles and starts ExifTool in stay_open mode. Arguments in commonArg
// are appended after built-in options (-stay_open, -@ -, -common_args, ready echo);
// package-level [Arg1] and [Config] are applied when non-empty. The server uses the
// same filesystem layout as [Command]: embedded Perl at "/" and the host working
// directory overlaid, with [os.TempDir] mounted read-write.
func NewServer(commonArg ...string) (*Server, error) {
	r, err := newRuntime(context.Background())
	if err != nil {
		return nil, err
	}
	compiled, err := r.CompileModule(context.Background(), wasmBinary)
	if err != nil {
		if closeErr := r.Close(context.Background()); closeErr != nil {
			return nil, fmt.Errorf("failed to compile wasm: %w (close error: %v)", err, closeErr)
		}
		return nil, err
	}

	e := &Server{
		runtime:  r,
		compiled: compiled,
		rootFS:   defaultRootFS(),
	}
	e.cmd = &cmdStub{Process: &processStub{server: e}}

	if Arg1 != "" {
		e.args = append(e.args, Arg1)
	}
	if Config != "" {
		e.args = append(e.args, "-config", Config)
	}
	e.args = append(e.args,
		"-stay_open", "true", "-@", "-",
		"-common_args",
		"-echo4", "{ready"+boundary+"}",
		"-charset", "filename=utf8",
	)
	e.args = append(e.args, commonArg...)

	if err := e.start(); err != nil {
		if closeErr := r.Close(context.Background()); closeErr != nil {
			return nil, fmt.Errorf("start failed: %w (close error: %v)", err, closeErr)
		}
		return nil, err
	}
	return e, nil
}

func closeAll(closers ...io.Closer) error {
	var errs []error
	for _, c := range closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func closeModules(mods ...api.Closer) error {
	var errs []error
	for _, m := range mods {
		if err := m.Close(context.Background()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (e *Server) start() error {
	e.mod = nil
	e.stdinW = nil
	e.stdoutR = nil
	e.stderrR = nil
	e.evalDone = nil
	e.evalErr = nil

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		_ = closeAll(stdinR, stdinW)
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = closeAll(stdinR, stdinW, stdoutR, stdoutW)
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	devFS := devNullFS{}
	fsConfig := wazero.NewFSConfig().
		WithFSMount(e.rootFS, "/").
		WithFSMount(devFS, "/dev").
		WithDirMount(os.TempDir(), os.TempDir())

	config := wazero.NewModuleConfig().
		WithStdin(stdinR).
		WithStdout(stdoutW).
		WithStderr(stderrW).
		WithName("zeroperl").
		WithArgs("zeroperl").
		WithStartFunctions().
		WithEnv("PERL5LIB", "/lib/5.42.0:/lib/5.42.0/wasm32-wasi").
		WithFSConfig(fsConfig)

	mod, err := e.runtime.InstantiateModule(context.Background(), e.compiled, config)
	if err != nil {
		_ = closeAll(stdinR, stdinW, stdoutR, stdoutW, stderrR, stderrW)
		return fmt.Errorf("wasm instantiate failed: %w", err)
	}

	cleanup := func() {
		_ = closeModules(mod)
		_ = closeAll(stdinR, stdinW, stdoutR, stdoutW, stderrR, stderrW)
	}

	initFn := mod.ExportedFunction("zeroperl_init")
	if initFn == nil {
		cleanup()
		return fmt.Errorf("zeroperl_init not found")
	}
	results, err := initFn.Call(context.Background())
	if err != nil {
		cleanup()
		return fmt.Errorf("zeroperl_init failed: %w", err)
	}
	if results[0] != 0 {
		cleanup()
		return fmt.Errorf("zeroperl_init returned %d", results[0])
	}

	e.mod = mod
	e.stdinW = stdinW
	e.stdoutR = stdoutR
	e.stderrR = stderrR
	e.stdin = printer{w: stdinW}
	e.stdout = bufio.NewScanner(stdoutR)
	e.stderr = bufio.NewScanner(stderrR)
	e.stdout.Split(splitReadyToken)
	e.stderr.Split(splitReadyToken)

	evalDone := make(chan struct{})
	e.evalDone = evalDone
	e.evalErr = nil

	go func() {
		defer close(evalDone)
		defer func() { _ = stdinW.Close() }()
		defer func() { _ = stdoutW.Close() }()
		defer func() { _ = stderrW.Close() }()
		defer func() { _ = stdinR.Close() }()

		mallocFn := mod.ExportedFunction("malloc")
		freeFn := mod.ExportedFunction("free")
		mem := mod.Memory()

		writeString := func(s string) (uint32, error) {
			b := append([]byte(s), 0)
			res, err := mallocFn.Call(context.Background(), uint64(len(b)))
			if err != nil {
				return 0, fmt.Errorf("malloc failed: %w", err)
			}
			ptr := uint32(res[0])
			if !mem.Write(ptr, b) {
				if _, freeErr := freeFn.Call(context.Background(), uint64(ptr)); freeErr != nil {
					return 0, fmt.Errorf("mem.Write failed at %d (free also failed: %w)", ptr, freeErr)
				}
				return 0, fmt.Errorf("mem.Write failed at %d", ptr)
			}
			return ptr, nil
		}

		wrapper := scriptPreamble + string(exiftoolScript)
		scriptPtr, err := writeString(wrapper)
		if err != nil {
			e.evalErr = fmt.Errorf("failed to allocate script memory: %w", err)
			return
		}
		defer func() {
			_, _ = freeFn.Call(context.Background(), uint64(scriptPtr))
		}()

		argPtrs := make([]uint32, len(e.args))
		for i, arg := range e.args {
			argPtrs[i], err = writeString(arg)
			if err != nil {
				e.evalErr = fmt.Errorf("failed to allocate arg memory: %w", err)
				for j := 0; j < i; j++ {
					_, _ = freeFn.Call(context.Background(), uint64(argPtrs[j]))
				}
				return
			}
			defer func(ptr uint32) {
				_, _ = freeFn.Call(context.Background(), uint64(ptr))
			}(argPtrs[i])
		}

		var argvPtr uint32
		if len(argPtrs) > 0 {
			res, err := mallocFn.Call(context.Background(), uint64(len(argPtrs)*4))
			if err != nil {
				e.evalErr = fmt.Errorf("failed to allocate argv: %w", err)
				return
			}
			argvPtr = uint32(res[0])
			defer func() {
				_, _ = freeFn.Call(context.Background(), uint64(argvPtr))
			}()
			for i, p := range argPtrs {
				if !mem.WriteUint32Le(argvPtr+uint32(i*4), p) {
					e.evalErr = fmt.Errorf("failed to write argv[%d]", i)
					return
				}
			}
		}

		evalFn := mod.ExportedFunction("zeroperl_eval")
		if evalFn == nil {
			e.evalErr = fmt.Errorf("zeroperl_eval not found")
			return
		}

		_, err = evalFn.Call(context.Background(), uint64(scriptPtr), uint64(1), uint64(len(e.args)), uint64(argvPtr))
		if err != nil {
			var exitErr *sys.ExitError
			if errors.As(err, &exitErr) {
				if exitErr.ExitCode() != 0 {
					e.evalErr = fmt.Errorf("exiftool exited with code %d", exitErr.ExitCode())
				}
			} else {
				e.evalErr = err
			}
		}
	}()

	return nil
}

func (e *Server) stop() {
	if e.stdinW != nil {
		_ = e.stdinW.Close()
	}
	if e.mod != nil {
		_ = e.mod.Close(context.Background())
	}
	if e.evalDone != nil {
		<-e.evalDone
	}
	if e.stdoutR != nil {
		_ = e.stdoutR.Close()
		e.stdoutR = nil
	}
	if e.stderrR != nil {
		_ = e.stderrR.Close()
		e.stderrR = nil
	}
}

func (e *Server) restart() {
	e.srvMtx.Lock()
	defer e.srvMtx.Unlock()
	if e.done {
		return
	}
	e.stop()
	e.restartErr = e.start()
}

// Command runs one ExifTool request: arguments are sent as lines on the server's
// stdin, then an -execute boundary. On success, returns a copy of stdout for that
// response (stderr is checked for errors). After [Shutdown] or [Close], Command
// returns an error and does not restart the server.
func (e *Server) Command(arg ...string) ([]byte, error) {
	e.cmdMtx.Lock()
	defer e.cmdMtx.Unlock()

	e.srvMtx.Lock()
	done := e.done
	e.srvMtx.Unlock()
	if done {
		return nil, errors.New("exiftool: server is closed")
	}

	if err := e.restartErr; err != nil {
		e.restartErr = nil
		return nil, fmt.Errorf("server had a previous restart error: %w", err)
	}

	if err := e.stdin.print(arg...); err != nil {
		e.restart()
		return nil, err
	}
	if err := e.stdin.print("-execute" + boundary); err != nil {
		e.restart()
		return nil, err
	}

	if !e.stdout.Scan() {
		err := e.stdout.Err()
		if err == nil {
			select {
			case <-e.evalDone:
				if e.evalErr != nil {
					err = e.evalErr
				} else {
					err = fmt.Errorf("exiftool: server closed unexpectedly: %w", io.EOF)
				}
			default:
				err = fmt.Errorf("exiftool: server closed unexpectedly: %w", io.EOF)
			}
		}
		e.restart()
		return nil, err
	}
	if !e.stderr.Scan() {
		err := e.stderr.Err()
		if err == nil {
			select {
			case <-e.evalDone:
				if e.evalErr != nil {
					err = e.evalErr
				} else {
					err = fmt.Errorf("exiftool: server closed unexpectedly: %w", io.EOF)
				}
			default:
				err = fmt.Errorf("exiftool: server closed unexpectedly: %w", io.EOF)
			}
		}
		e.restart()
		return nil, err
	}

	if len(e.stderr.Bytes()) > 0 {
		return nil, errors.New("exiftool: " + string(bytes.TrimSpace(e.stderr.Bytes())))
	}
	return append([]byte(nil), e.stdout.Bytes()...), nil
}

// finalize marks the server as done and cleans up readers and the runtime.
// It must be called with srvMtx held.
func (e *Server) finalize() error {
	e.done = true
	if e.stdoutR != nil {
		_ = e.stdoutR.Close()
		e.stdoutR = nil
	}
	if e.stderrR != nil {
		_ = e.stderrR.Close()
		e.stderrR = nil
	}
	return e.runtime.Close(context.Background())
}

// Close forcibly stops the server: closes stdin and the WASM module, waits for the
// long-running eval goroutine, then closes the wazero runtime. Idempotent; safe after
// [Shutdown]. A second Close returns nil.
func (e *Server) Close() error {
	e.srvMtx.Lock()
	defer e.srvMtx.Unlock()
	if e.done {
		return nil
	}
	if e.stdinW != nil {
		_ = e.stdinW.Close()
	}
	if e.mod != nil {
		_ = e.mod.Close(context.Background())
	}
	if e.evalDone != nil {
		<-e.evalDone
	}
	return e.finalize()
}

// Shutdown stops ExifTool cleanly by sending -stay_open false and closing stdin,
// waits for processing to finish, then releases resources like [Close]. Returns an
// error if the server was already closed. If [Close] runs concurrently, Shutdown
// may return nil after the other caller has finalized the runtime.
func (e *Server) Shutdown() error {
	e.cmdMtx.Lock()
	defer e.cmdMtx.Unlock()

	e.srvMtx.Lock()
	if e.done {
		e.srvMtx.Unlock()
		return errors.New("exiftool: server is closed")
	}
	e.srvMtx.Unlock()

	_ = e.stdin.print("-stay_open", "false")
	if err := e.stdin.close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	<-e.evalDone

	e.srvMtx.Lock()
	defer e.srvMtx.Unlock()
	if e.done {
		return nil
	}
	return e.finalize()
}

// splitReadyToken is a [bufio.SplitFunc] for ExifTool -stay_open output: each response
// ends with the ready marker from -echo4 (see boundary) and a newline.
func splitReadyToken(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.Index(data, []byte("{ready"+boundary+"}")); i >= 0 {
		if n := bytes.IndexByte(data[i:], '\n'); n >= 0 {
			if atEOF && len(data) == (n+i+1) {
				return n + i + 1, data[:i], bufio.ErrFinalToken
			} else {
				return n + i + 1, data[:i], nil
			}
		}
	}
	if atEOF {
		return 0, data, io.EOF
	}
	return 0, nil, nil
}
