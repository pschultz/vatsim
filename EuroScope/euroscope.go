package euroscope

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/pschultz/vatsim"
)

type Asker interface {
	Ask(string) (string, error)
}

var airlines = make(map[string]string, 6500) // name to code
var airlineNames [][2]string                 // keys of airlines
var airlineFixes = make(map[string]string)   // answer cache

var aircraft = make(vatsim.Set, 2300)
var aircraftIDs [][2]string                 // members of aircraft
var aircraftFixes = make(map[string]string) // answer cache

func AirlineCode(name string) string {
	return airlines[name]
}

func Airlines() map[string]string {
	return airlines
}

func FindAircraft(code string) (exact bool, similar [][2]string) {
	return vatsim.FindMatches(code, aircraftIDs)
}

func FindAirline(name string) (exact bool, similar [][2]string) {
	return vatsim.FindMatches(name, airlineNames)
	/*
		if exact {
			return
		}
		for i, s := range similar {
			if len(s[1]) < 3 {
				continue
			}

			code := s[1][:3]
			op, err := icao.LookupOperator(code)
			if err == nil {
				similar[i][1] += " ICAO API: " + op.TelephonyName
			}
		}
		return
	*/
}

func FixAircraft(ui Asker, id string) string {
	return fix(ui,
		"Aircraft", id,
		aircraftFixes, FindAircraft,
	)
}

func FixAirline(ui Asker, name string) string {
	return fix(ui,
		"Airline", name,
		airlineFixes, FindAirline,
	)
}

func LoadAircraftFile(name string) error {
	return load(name, 4, func(fields []string) {
		icao, hint1, hint2 := fields[0], fields[2], fields[3]
		if !aircraft.Has(icao) {
			aircraft.Add(icao)
			aircraftIDs = append(aircraftIDs, [2]string{icao, hint1 + " " + hint2})
		}
	})
}

func LoadAirlinesFile(name string) error {
	return load(name, 3, func(fields []string) {
		icao, hint, name := fields[0], fields[1], fields[2]
		if _, ok := airlines[name]; !ok {
			airlines[name] = icao
			airlineNames = append(airlineNames, [2]string{name, icao + " " + hint})
		}
	})
}

func load(name string, nFields int, fn func([]string)) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) != nFields {
			return fmt.Errorf("euroscope: unexpected number of fields: %d != %d", len(fields), nFields)
		}
		fn(fields)
	}
	return scanner.Err()
}

func fix(ui Asker, qPrefix string, name string, cache map[string]string, choices func(string) (bool, [][2]string)) string {
	name = strings.ToUpper(name)
	if m, ok := cache[name]; ok {
		return m
	}
	ok, others := choices(name)
	if ok {
		return name
	}
	others = append(others, [2]string{name, "original"})
	fixed := ask(ui, fmt.Sprintf("%s %q not found", qPrefix, name), others)
	cache[name] = fixed
	return fixed
}

func ask(ui Asker, q string, choices [][2]string) string {
	buf := bytes.NewBufferString(q)
	for i, c := range choices {
		fmt.Fprintf(buf, "\n[%d] %s (%s)", i+1, c[0], c[1])
	}
	buf.WriteByte('\n')
	if len(choices) > 0 {
		buf.WriteString("(default: 1)")
	}

ask:
	answer, err := ui.Ask(buf.String())
	if err != nil {
		log.Fatal(err)
	}
	if answer == "" {
		if len(choices) > 0 {
			return choices[0][0]
		}
		goto ask
	}

	k, err := strconv.Atoi(answer)
	if err == nil && k <= len(choices) {
		return choices[k-1][0]
	}

	return answer
}
