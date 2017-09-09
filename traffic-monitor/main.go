package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pschultz/vatsim"
	"github.com/pschultz/vatsim/vPilot"
)

var airlines map[string][]string

type DB []*Row

type Row struct {
	Callsign     string
	LastSeen     time.Time
	Aircraft     vatsim.Set
	Origins      vatsim.Set
	Destinations vatsim.Set
}

type Station struct {
	Role        string
	Callsign    string
	Aircraft    string
	Origin      string
	Destination string
}

func LoadDB() (*DB, error) {
	var rows []*Row

	f, err := os.Open("traffic.json.gz")
	switch {
	case os.IsNotExist(err):
		return &DB{}, nil
	case err != nil:
		return nil, err
	}
	defer f.Close()

	r, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if err := json.NewDecoder(r).Decode(&rows); err != nil {
		return nil, err
	}

	// Prune old rows
	current := make([]*Row, 0, len(rows))
	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	for _, row := range rows {
		if row.LastSeen.After(oneYearAgo) {
			current = append(current, row)
		}
	}

	db := DB(current)
	return &db, nil
}

func (db *DB) Save() error {
	buf := &bytes.Buffer{}

	w, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		return err
	}

	err = json.NewEncoder(w).Encode(db)
	w.Close()
	if err != nil {
		return err
	}

	f, err := os.Create("traffic.json.gz")
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, buf)
	return err
}

func (db *DB) Add(s Station) {
	for _, x := range *db {
		if x.Callsign == s.Callsign {
			x.Aircraft.Add(s.Aircraft)
			x.Origins.Add(s.Origin)
			x.Destinations.Add(s.Destination)
			x.LastSeen = time.Now().UTC()
			return
		}
	}
	*db = append(*db, &Row{
		Callsign:     s.Callsign,
		LastSeen:     time.Now().UTC(),
		Aircraft:     vatsim.NewSet(s.Aircraft),
		Origins:      vatsim.NewSet(s.Origin),
		Destinations: vatsim.NewSet(s.Destination),
	})
}

var aircraftPattern = regexp.MustCompile(`^(?:[MHT]/)?([A-Z][A-Z0-9]{1,3})(?:/[A-Z])?$`)

func main() {
	historic := flag.Bool("historic", false, "Show historic data instead of in-progress flights.")
	flag.Parse()

	mm, err := vPilot.LoadDefaults()
	if err != nil {
		log.Fatal(err)
	}

	if err := loadAirlines(); err != nil {
		log.Println(err)
	}

	db, err := LoadDB()
	if err != nil {
		log.Fatal(err)
	}

	if *historic {
		maxKeyLen := len("Airline/-craft")
		maxGlobal, maxLocal := 100000, 10000 // matches width of words "global" and "local"

		global := make(map[string]int)
		local := make(map[string]int)
		missing := make(map[string]bool)

		for _, row := range *db {
			airline := row.Callsign[:3]
			for ac := range row.Aircraft {

				key := airline + " " + ac
				if n := len(key); n > maxKeyLen {
					maxKeyLen = n
				}

				global[key] += 1
				if global[key] > maxGlobal {
					maxGlobal = global[key]
				}

				if row.Destinations.Has("EDDT") || row.Origins.Has("EDDT") {
					local[key] += 1
					if local[key] > maxLocal {
						maxLocal = local[key]
					}
				}

				if !mm.Has(airline, ac) {
					missing[key] = true
				}
			}
		}

		keys := make([]string, 0, len(global))
		for k := range global {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		format := fmt.Sprintf("%%-%ds  %%%dv  %%%dv  %%v\n",
			maxKeyLen,
			int(math.Log10(float64(maxGlobal)))+2,
			int(math.Log10(float64(maxLocal)))+2,
		)
		fmt.Printf(format, "", "Unique Callsigns", "", "")
		fmt.Printf(format, "Airline/-craft", "Global", "Local", "missing")
		for _, k := range keys {
			fmt.Printf(format, k, global[k], local[k], missing[k])
		}

		return
	}

	ticker := time.Tick(10 * time.Minute)
	for {
		stations, err := fetchStations()
		if err != nil {
			log.Println(err)
		} else {
			log.Println(len(stations), "pilots online")
		}

		for _, s := range stations {
			if s.Role != "PILOT" || len(s.Callsign) < 3 {
				continue
			}

			s.Aircraft = parseAircraft(s.Aircraft)
			if s.Aircraft == "" {
				continue
			}

			db.Add(s)

			if s.Origin != "EDDT" && s.Destination != "EDDT" {
				continue
			}

			if mm.Has(s.Callsign[:3], s.Aircraft) {
				continue
			}

			fmt.Printf("Missing: %-7s  %-6s  %4s to %4s  %v\n",
				s.Callsign, s.Aircraft,
				s.Origin, s.Destination,
				airlines[s.Callsign[:3]],
			)
		}

		if err := db.Save(); err != nil {
			log.Println(err)
		}

		<-ticker
		fmt.Println("")
	}
}

func loadAirlines() error {
	f, err := os.Open("EuroScope/IOSA_Airlines.txt")
	if err != nil {
		return err
	}
	defer f.Close()

	airlines = make(map[string][]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			log.Println("Weird IOSA line: %s")
			continue
		}
		airlines[fields[0]] = fields[1:]
	}
	return nil
}

func parseAircraft(s string) string {
	if x := aircraftPattern.FindStringSubmatch(s); len(x) > 1 {
		return x[1]
	}
	return ""
}

func fetchStations() ([]Station, error) {
	res, err := http.Get("http://api.vateud.net/online/pilots/ed.json")
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, err
	}

	var stations []Station
	if err := json.NewDecoder(res.Body).Decode(&stations); err != nil {
		return nil, err
	}

	return stations, nil
}
