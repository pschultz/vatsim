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

var DefaultAirlinesFile = "EuroScope/EDBB/ICAO_Airlines.txt"
var DefaultAircraftFile = "EuroScope/EDBB/ICAO_Aircraft.txt"

var airlines = make(map[string]string, 6500) // name to code
var airlineNames []string                    // keys of airlines
var airlineFixes = make(map[string]string)   // answer cache

var aircraft = make(vatsim.Set, 2300)
var aircraftIDs []string                    // members of aircraft
var aircraftFixes = make(map[string]string) // answer cache

func AirlineCode(name string) string {
	return airlines[name]
}

func FindAircraft(code string) (exact bool, similar []string) {
	return vatsim.FindMatches(code, aircraftIDs)
}

func FindAirline(name string) (exact bool, others []string) {
	return vatsim.FindMatches(name, airlineNames)
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
	err := load(name, 4, func(fields []string) { aircraft.Add(fields[0]) })
	if err != nil {
		return err
	}
	aircraftIDs = aircraft.All()
	return nil
}

func LoadAirlinesFile(name string) error {
	err := load(name, 3, func(fields []string) { airlines[fields[2]] = fields[0] })
	if err != nil {
		return err
	}
	airlineNames = make([]string, 0, len(airlines))
	for i := range airlines {
		airlineNames = append(airlineNames, i)
	}
	return nil
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

func fix(ui Asker, qPrefix string, name string, cache map[string]string, choices func(string) (bool, []string)) string {
	name = strings.ToUpper(name)
	if m, ok := cache[name]; ok {
		return m
	}
	ok, others := choices(name)
	if ok {
		return name
	}
	fixed := ask(ui, fmt.Sprintf("%s %q not found", qPrefix, name), others)
	cache[name] = fixed
	return fixed
}

func ask(ui Asker, q string, choices []string) string {
	buf := bytes.NewBufferString(q)
	for i, c := range choices {
		fmt.Fprintf(buf, "\n[%d] %s", i+1, c)
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
			return choices[0]
		}
		goto ask
	}

	k, err := strconv.Atoi(answer)
	if err == nil && k <= len(choices) {
		return choices[k-1]
	}

	return answer
}
