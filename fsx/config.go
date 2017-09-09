package fsx

import "io"

func InspectConfig(r io.Reader) (atcModel, atcAirline string, titles []string, err error) {
	var meta meta
	meta, err = (&Installer{}).loadConfig(r, 0, nil)

	atcModel = meta.ATCModel
	atcAirline = meta.ATCAirline
	titles = meta.Titles
	return
}
