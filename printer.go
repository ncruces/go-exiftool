package exiftool

import "io"

type printer struct {
	w   io.WriteCloser
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
	buf := make([]byte, 0, n)
	for _, l := range lines {
		buf = append(buf, l...)
		buf = append(buf, '\n')
	}

	_, p.err = p.w.Write(buf)
	return p.err
}

func (p *printer) close() error {
	err := p.w.Close()
	if p.err == nil {
		p.err = err
	}
	return p.err
}
