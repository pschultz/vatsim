package fsx

import "io"

func InspectConfig(r io.Reader) ([]Meta, error) {
	return (&Installer{}).loadConfig(r, 0, nil)
}
