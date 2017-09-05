package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"io"
	"regexp"
	"strings"
)

type Database struct {
	Wanted       map[string]map[string]bool // Airline to Aircraft to Urgent
	AirlineNames map[string]string
	AircraftAlts []map[string]bool
}

func (d *Database) UnmarshalJSON(b []byte) error {
	d.Wanted = make(map[string]map[string]bool)

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
	x := make(map[string][]string, len(d.Wanted))
	for al, acs := range d.Wanted {
		for ac := range acs {
			x[al] = append(x[al], ac)
		}
	}
	return json.Marshal(x)
}

func (d *Database) ReadVPilotModelData(r io.Reader) error {
	d.AircraftAlts = make([]map[string]bool, 0, 60)
	var m struct {
		XMLName          xml.Name `xml:"ModelMatchingData"`
		SimilarTypeCodes struct {
			String []string `xml:"string"`
		}
	}

	if err := xml.NewDecoder(r).Decode(&m); err != nil {
		return err
	}

	for _, s := range m.SimilarTypeCodes.String {
		codes := strings.Fields(s)
		if len(codes) < 2 {
			continue
		}
		set := make(map[string]bool, len(codes))
		for _, x := range codes {
			if x == "CRJ2" {
				set["CRJ200"] = true
			}
			set[x] = true
		}
		d.AircraftAlts = append(d.AircraftAlts, set)
	}
	return nil
}

func (d *Database) ReadEuroScopeICAO(r io.Reader) error {
	if d.AirlineNames == nil {
		d.AirlineNames = make(map[string]string, 10e3)
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		switch len(fields) {
		case 3:
			d.AirlineNames[fields[0]] = fields[2]
		default:
			// Probably not an ICAO_Airlines file
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
	case d.Wanted == nil:
		d.Wanted = make(map[string]map[string]bool)
		fallthrough
	case d.Wanted[airline] == nil:
		d.Wanted[airline] = make(map[string]bool)
	case d.Wanted[airline][x.Aircraft]:
		// already present and Urgent
		return
	}

	urgent := x.Origin == "EDDT" || x.Destination == "EDDT"
	d.Wanted[airline][x.Aircraft] = urgent
}

func (d *Database) All() map[string]map[string]bool {
	x := make(map[string]map[string]bool, len(d.Wanted))

	for al, acs := range d.Wanted {
		x[al] = make(map[string]bool, len(acs))
		seen := make(map[string]bool, len(acs))
		for ac, urgent := range acs {
			ac = parseAircraft(ac)
			switch {
			case ac == "":
			case seen[ac]:
			default:
				seen[ac] = true
				x[al][ac] = urgent
			}
		}
	}

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
