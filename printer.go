package exiftool

import "io"

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
		p.buf = append(p.buf, make([]byte, n-len(p.buf))...)
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
