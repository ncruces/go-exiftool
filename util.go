package exiftool

import "io"

func fprintln(w io.Writer, s string) (int, error) {
	w.Write([]byte(s))
	return w.Write([]byte{'\n'})
}
