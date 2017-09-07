package fsx

import "io"

type Package interface {
	Files() map[string]io.ReadCloser
}
