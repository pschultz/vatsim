package woai

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
)

type Package struct {
	files map[string]io.ReadCloser
}

func NewPackage(r io.Reader) (*Package, error) {
	p := &Package{
		files: make(map[string]io.ReadCloser),
	}

	if err := p.read(r); err != nil {
		for _, f := range p.files {
			f.Close()
		}
		return nil, err
	}

	return p, nil
}

func (p *Package) Files() map[string]io.ReadCloser {
	return p.files
}

func (p *Package) read(r io.Reader) error {
	zips, err := p.unwrap(r)
	if err != nil {
		return err
	}

	// Now parse the actual packages (which is most likely just one)
	for _, b := range zips {
		z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			return err
		}

		for _, f := range z.File {
			// Fixup filenames if necessary. All names must be relative to the
			// FSX root directory.
			name := filepath.Clean(strings.Replace(f.Name, `\`, "/", -1))
			iname := strings.ToLower(name)

			switch {
			case strings.HasPrefix(iname, "aircraft/"):
				name = name[len("aircraft/"):] // ignore and preserve casing
				name = filepath.Join("SimObjects/Airplanes", name)
			case strings.HasPrefix(iname, "texture/"),
				strings.HasPrefix(iname, "scenery/"),
				strings.HasPrefix(iname, "effects/"):
				// no-op
			case strings.HasSuffix(iname, ".txt"),
				strings.HasPrefix(iname, "addon scenery/"),
				iname == "avsim.diz",
				iname == "woai.cfg",
				iname == "version.ini":
				// ignore
				continue
			default:
				log.Println("woai: skipping file in package: ", name)
				continue
			}

			r, err := f.Open()
			if err != nil {
				return err
			}
			p.files[name] = r
		}
	}
	return nil
}

// unwrap unzips and decrypts all packages in r.
func (p *Package) unwrap(r io.Reader) ([][]byte, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}

	var zips [][]byte

	for _, f := range z.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".woai.zip") {
			continue
		}

		r, err := f.Open()
		if err != nil {
			return nil, err
		}

		b, err := ioutil.ReadAll(r)
		r.Close()
		if err != nil {
			return nil, err
		}

		z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			return nil, err
		}

		for _, f := range z.File {
			if !strings.HasSuffix(strings.ToLower(f.Name), ".woai.enc") {
				continue
			}

			r, err := f.Open()
			if err != nil {
				return nil, err
			}

			b, err := ioutil.ReadAll(r)
			r.Close()
			if err != nil {
				return nil, err
			}

			crypto.CryptBlocks(b, b)
			zips = append(zips, b)
		}
	}

	return zips, nil
}
