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

type InstalledModel struct {
	AirlineName string
	Model       string
	Title       string
}

var airlineFixes = map[string]string{
	"SPEED BIRD": "SPEEDBIRD",
}

func AircraftConfig(filename string) ([]InstalledModel, error) {
	// config file reference: https://msdn.microsoft.com/en-us/library/cc526949.aspx

	id := filepath.Base(filepath.Dir(filename))
	if id == "" {
		return nil, errors.New("malformed filename: " + filename)
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	variants, err := ParseIni(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %v", filename, err)
	}

	atcModel := variants["general"]["atc_model"]
	if atcModel == "" {
		fmt.Println("Empty atc_model: ", filename)
		return nil, nil
	}

	var models []InstalledModel

	for n, section := range variants {
		if !strings.HasPrefix(n, "fltsim.") {
			continue
		}

		m := InstalledModel{
			Title:       section["title"],
			AirlineName: strings.ToUpper(section["atc_airline"]),
			Model:       atcModel,
		}

		if fix := airlineFixes[m.AirlineName]; fix != "" {
			m.AirlineName = fix
		}

		switch {
		case m.Title == "":
			fmt.Println("Empty title: ", filename)
		case m.AirlineName == "":
			// Happens all the time for stock models
			// fmt.Println("Empty atc_airline: ", filename)
		default:
			models = append(models, m)
		}
	}

	return models, nil
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
