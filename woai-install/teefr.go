package main

import (
	"io"
	"os"
	"path/filepath"
)

type teeFileReader struct {
	r io.ReadCloser
	f *os.File
}

// TeeReader returns a Reader that writes to the named file what it reads from
// r. All reads from r performed through it are matched with corresponding
// writes. There is no internal buffering - the write must complete before the
// read completes. Any error encountered while writing is reported as a read
// error. The file is truncated before the first write and removed if an error
// occurs.
func TeeFileReader(r io.ReadCloser, fname string) (io.ReadCloser, error) {
	if err := os.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		return nil, err
	}

	f, err := os.Create(fname)
	if err != nil {
		return nil, err
	}

	return &teeFileReader{r, f}, nil
}

func (t *teeFileReader) Read(p []byte) (n int, err error) {
	n, err = t.r.Read(p)
	if n > 0 {
		if n, err := t.f.Write(p[:n]); err != nil {
			os.Remove(t.f.Name())
			return n, err
		}
	}
	return
}

func (t *teeFileReader) Close() error {
	e1 := t.r.Close()
	e2 := t.f.Close()

	if e1 != nil {
		return e1
	}
	return e2
}
