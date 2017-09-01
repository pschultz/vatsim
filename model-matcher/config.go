package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Config map[string]map[string]string

func AircraftConfig(filename string) (Config, error) {
	// config file reference: https://msdn.microsoft.com/en-us/library/cc526949.aspx

	id := filepath.Base(filepath.Dir(filename))
	if id == "" {
		return nil, errors.New("Malformed filename: " + filename)
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	variants, err := ParseIni(f)
	if err != nil {
		return nil, fmt.Errorf("Parse %s: %v", filename, err)
	}

	cfg := make(Config)
	atcModel := variants["general"]["atc_model"]

	for n, section := range variants {
		if !strings.HasPrefix(n, "fltsim.") {
			continue
		}
		delete(section, "model")
		delete(section, "atc_parking_codes")
		delete(section, "atc_parking_types")
		delete(section, "ui_manufacturer")
		delete(section, "atc_heavy")
		if atcModel != "" {
			section["atc_model"] = atcModel
		}

		cfg[id+"_"+n[7:]] = section
	}

	return cfg, nil
}

func ParseIni(r io.Reader) (Config, error) {
	cfg := make(Config)
	scanner := bufio.NewScanner(r)

	var section, key, value string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if n := strings.Index(line, "//"); n >= 0 {
			line = strings.TrimSpace(line[:n])
		}
		if line == "" {
			continue
		}
		switch line[0] {
		case '[':
			if !strings.HasSuffix(line, "]") {
				return nil, errors.New("missing section end char: " + line)
			}
			section = strings.ToLower(line[1 : len(line)-1])
			key = ""
			value = ""
			if cfg[section] == nil {
				cfg[section] = make(map[string]string)
			}
			continue
		case ';', '#', '/', '<':
			continue
		}
		if section == "" {
			continue
		}
		if n := strings.IndexByte(line, '='); n > 0 {
			key = strings.ToLower(strings.TrimSpace(line[:n]))
			value = strings.TrimSpace(line[n+1:])
			cfg[section][key] = value
			continue
		}
		cfg[section][key] += "\n" + line
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return cfg, nil
}
