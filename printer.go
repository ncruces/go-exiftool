package exiftool

import "io"

// printer writes stay_open command lines to stdin, one string per line, reusing an internal buffer.
// After the first write error, print becomes a no-op returning that error; close preserves it.
type printer struct {
	w   io.WriteCloser
	buf []byte
	err error
}

func (p *printer) print(lines ...string) error {
	if p.err != nil {
		return p.err
	}

	n := len(lines)
	for _, l := range lines {
		n += len(l)
	}

	if cap(p.buf) < n {
		p.buf = make([]byte, n, n*2)
	} else {
		p.buf = p.buf[:n]
	}

	i := 0
	for _, l := range lines {
		i += copy(p.buf[i:], l)
		p.buf[i] = '\n'
		i++
	}

	_, p.err = p.w.Write(p.buf)
	return p.err
}

func (p *printer) close() error {
	err := p.w.Close()
	if p.err == nil {
		p.err = err
	}
	return p.err
}
