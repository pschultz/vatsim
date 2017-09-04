package main

import (
	"bufio"
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strings"
)

type Database struct {
	models        map[string]map[string]bool
	euroScopeICAO map[string][]string
}

type Model struct {
	AC        string // Aircraft ICAO
	AL        string // Airline ICAO
	ALKeyword string

	Terms []string // Terms for fulltext search

	Urgent bool // To or from EDDT
}

func (d *Database) UnmarshalJSON(b []byte) error {
	d.models = make(map[string]map[string]bool)

	x := make(map[string][]string)
	if err := json.Unmarshal(b, &x); err != nil {
		return err
	}

	for al, acs := range x {
		for _, ac := range acs {
			d.Add(APIStation{Callsign: al, Aircraft: ac})
		}
	}

	return nil
}

func (d *Database) MarshalJSON() ([]byte, error) {
	x := make(map[string][]string, len(d.models))
	for al, acs := range d.models {
		for ac := range acs {
			x[al] = append(x[al], ac)
		}
	}
	return json.Marshal(x)
}

func (d *Database) ReadEuroScopeICAO(r io.Reader) error {
	if d.euroScopeICAO == nil {
		d.euroScopeICAO = make(map[string][]string)
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		switch len(fields) {
		case 4:
			// Aircraft. Not sure what the second column is; ignore.
			d.euroScopeICAO[fields[0]] = fields[2:]
		case 3:
			// Airline. Grab only the keyword.
			d.euroScopeICAO[fields[0]] = fields[2:]
		}
	}

	return scanner.Err()
}

type APIStation struct {
	Callsign string
	Aircraft string

	Destination string
	Origin      string
}

func (d *Database) Add(x APIStation) {
	if len(x.Aircraft) < 3 {
		return
	}

	airline := parseCallsign(x.Callsign)
	if airline == "" {
		return
	}

	switch {
	case d.models[airline] == nil:
		d.models[airline] = make(map[string]bool)
	case d.models[airline][x.Aircraft]:
		// already present and Urgent
		return
	}

	urgent := x.Origin == "EDDT" || x.Destination == "EDDT"
	d.models[airline][x.Aircraft] = urgent
}

func (d *Database) All() []Model {
	x := make([]Model, 0, len(d.models))
	for al, acs := range d.models {
		seen := make(map[string]bool, len(acs))
		for ac, urgent := range acs {
			ac = parseAircraft(ac)
			if ac == "" || seen[ac] {
				continue
			}
			seen[ac] = true
			keywords := d.euroScopeICAO[al]
			if len(keywords) == 0 {
				continue
			}

			x = append(x, Model{
				AC:        ac,
				AL:        al,
				ALKeyword: keywords[0],
				Terms:     d.euroScopeICAO[ac],

				Urgent: urgent,
			})
		}
	}

	sort.Slice(x, func(i, j int) bool {
		switch {
		case x[i].AL < x[j].AL:
			return true
		case x[i].AL > x[j].AL:
			return false
		case x[i].AC < x[j].AC:
			return true
		case x[i].AC > x[j].AC:
			return false
		default:
			return false
		}
	})
	return x
}

var callsignPattern = regexp.MustCompile(`^[A-Z]{3}`)
var aircraftPattern = regexp.MustCompile(`^(?:[MHT]/)?([A-Z][A-Z0-9]{1,3})(?:/[A-Z])?$`)

func parseCallsign(callsign string) (airlineCode string) {
	return callsignPattern.FindString(callsign)
}

func parseAircraft(s string) string {
	if x := aircraftPattern.FindStringSubmatch(s); len(x) > 1 {
		return x[1]
	}
	return ""
}
