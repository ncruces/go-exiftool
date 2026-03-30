package exiftool

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

type errorWriter struct {
	err error
}

func (e *errorWriter) Write(p []byte) (int, error) { return 0, e.err }
func (e *errorWriter) Close() error                { return e.err }

func Test_printer_print(t *testing.T) {
	var buf strings.Builder

	printer := printer{w: nopCloser{&buf}}

	if err := printer.print("abc", "def"); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "abc\ndef\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	buf.Reset()

	if err := printer.print("123"); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "123\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	buf.Reset()

	if err := printer.print("xyzw", "rgba", "stpq"); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "xyzw\nrgba\nstpq\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	buf.Reset()
}

func Test_printer_print_empty(t *testing.T) {
	var buf strings.Builder
	printer := printer{w: nopCloser{&buf}}

	if err := printer.print(""); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func Test_printer_print_single(t *testing.T) {
	var buf strings.Builder
	printer := printer{w: nopCloser{&buf}}

	if err := printer.print("hello"); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "hello\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func Test_printer_print_writeError(t *testing.T) {
	testErr := errors.New("write error")
	printer := printer{w: &errorWriter{err: testErr}}

	err := printer.print("test")
	if err == nil {
		t.Fatal("expected error")
	}
	if err != testErr {
		t.Errorf("got %v, want %v", err, testErr)
	}
}

func Test_printer_print_errorPersists(t *testing.T) {
	testErr := errors.New("write error")
	printer := printer{w: &errorWriter{err: testErr}}

	// First call sets the error
	err := printer.print("test")
	if err != testErr {
		t.Fatalf("first call: got %v, want %v", err, testErr)
	}

	// Subsequent calls should return the same error without writing
	err = printer.print("test2")
	if err != testErr {
		t.Errorf("second call: got %v, want %v", err, testErr)
	}
}

func Test_printer_close(t *testing.T) {
	var buf strings.Builder
	printer := printer{w: nopCloser{&buf}}

	err := printer.close()
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func Test_printer_close_error(t *testing.T) {
	testErr := errors.New("close error")
	printer := printer{w: &errorWriter{err: testErr}}

	err := printer.close()
	if err != testErr {
		t.Errorf("got %v, want %v", err, testErr)
	}
}

func Test_printer_close_preservesWriteError(t *testing.T) {
	writeErr := errors.New("write error")
	closeErr := errors.New("close error")
	// Use a writer that returns writeErr on Write and closeErr on Close
	w := &struct {
		io.Writer
		io.Closer
	}{
		Writer: &errorWriter{err: writeErr},
		Closer: &errorCloser{err: closeErr},
	}
	printer := printer{w: w}

	// First, trigger a write error
	if err := printer.print("test"); err != writeErr {
		t.Fatalf("print: got %v, want %v", err, writeErr)
	}

	// Close should preserve the write error, not the close error
	err := printer.close()
	if err != writeErr {
		t.Errorf("got %v, want %v (write error should take precedence)", err, writeErr)
	}
}

type errorCloser struct {
	err error
}

func (e *errorCloser) Close() error { return e.err }

func Test_printer_print_bufferGrowth(t *testing.T) {
	var buf strings.Builder
	printer := printer{w: nopCloser{&buf}}

	// Small initial write
	if err := printer.print("short"); err != nil {
		t.Fatal(err)
	}
	buf.Reset()

	// Large write should grow buffer
	longLine := strings.Repeat("x", 10000)
	if err := printer.print(longLine); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != longLine+"\n" {
		t.Errorf("large write: got %d bytes, want %d bytes", len(got), len(longLine)+1)
	}
	buf.Reset()

	// Subsequent small write should reuse buffer
	if err := printer.print("small"); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "small\n"; got != want {
		t.Errorf("after growth: got %q, want %q", got, want)
	}
}

func Test_printer_print_unicode(t *testing.T) {
	var buf strings.Builder
	printer := printer{w: nopCloser{&buf}}

	if err := printer.print("日本語", "Ünïcödé"); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "日本語\nÜnïcödé\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
