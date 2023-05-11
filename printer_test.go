package exiftool

import (
	"io"
	"strings"
	"testing"
)

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

func Test_printer_print(t *testing.T) {
	var buf strings.Builder

	printer := printer{w: nopCloser{&buf}}

	printer.print("abc", "def")
	if got, want := buf.String(), "abc\ndef\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	buf.Reset()

	printer.print("123")
	if got, want := buf.String(), "123\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	buf.Reset()

	printer.print("xyzw", "rgba", "stpq")
	if got, want := buf.String(), "xyzw\nrgba\nstpq\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	buf.Reset()
}
