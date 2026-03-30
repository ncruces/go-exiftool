package exiftool

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
)

// Command runs ExifTool once with the given arguments and returns combined stdout.
// stdin may be nil; when ExifTool is invoked with "-@ -", lines from stdin supply
// extra arguments. Host paths in arg are resolved relative to the process working
// directory (mounted at "/" inside the WASM sandbox together with embedded Perl libs;
// see package documentation).
//
// Command is equivalent to CommandContext with [context.Background].
func Command(stdin io.Reader, arg ...string) ([]byte, error) {
	return CommandContext(context.Background(), stdin, arg...)
}

// CommandContext is like [Command] but honors ctx for cancellation only before the
// WASM runtime is created; a context deadline or cancel during eval is not guaranteed
// to interrupt an in-flight ExifTool run.
//
// Non-empty stderr from ExifTool is returned as an error. Some failures may still
// return partial stdout together with an error.
func CommandContext(ctx context.Context, stdin io.Reader, arg ...string) (out []byte, err error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r, err := newRuntime(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := r.Close(ctx); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	compiled, err := r.CompileModule(ctx, wasmBinary)
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer
	args := commandArgs(arg)
	out, err = evalModule(ctx, r, compiled, defaultRootFS(), nil, []string{os.TempDir()}, stdin, &stdout, &stderr, args...)
	if err != nil {
		return out, err
	}
	if stderr.Len() > 0 {
		return out, errors.New("exiftool: " + stderr.String())
	}
	return out, nil
}
