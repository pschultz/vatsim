package main

import (
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var FSXRoot = "/mnt/c/Program Files (x86)/Steam/steamapps/common/FSX"

func main() {
	if d := os.Getenv("FSX_ROOT"); d != "" {
		FSXRoot = d
	}

	watch := flag.Bool("watch", false, "Monitor the vatsim API and update the ruleset when never-seen-before aircraft appear.")
	flag.Parse()

	// for each model in the database, look

	db := &Database{} // maps Airline codes to sets of aircraft codes
	if f, err := os.Open("icao.json"); err != nil {
		log.Println(err)
	} else {
		json.NewDecoder(f).Decode(db)
		f.Close()
	}

	if f, err := os.Open("EuroScope/EDBB/ICAO_Aircraft.txt"); err != nil {
		log.Fatal(err)
	} else {
		db.ReadEuroScopeICAO(f)
		f.Close()
	}

	if f, err := os.Open("EuroScope/EDBB/ICAO_Airlines.txt"); err != nil {
		log.Fatal(err)
	} else {
		db.ReadEuroScopeICAO(f)
		f.Close()
	}

	if f, err := os.Open("model-matcher/ModelMatchingData.xml.gz"); err != nil {
		log.Fatal(err)
	} else {
		r, err := gzip.NewReader(f)
		if err != nil {
			log.Fatal(err)
		}
		db.ReadVPilotModelData(r)
		r.Close()
		f.Close()
	}

	// TODO: Figure out which charset works for the txt files. It's something
	// exotic. See for instance db.EuroScopeICAO["LAU"][0]. That's supposed to be
	// "Líneas Aéreas Suramericanas - Colombia"
	//fmt.Println(db.EuroScopeICAO["LAU"][0], []byte(db.EuroScopeICAO["LAU"][0][:8]))

	index := make(map[string]map[string][]string) // Airline to Aircraft to Titles
	/*
		index, err := CreateIndex()
		if err != nil {
			log.Fatal(err)
		}
	*/

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		if err := updateDB(db); err != nil {
			log.Println(err)
		}
		wg.Done()
	}()

	paths, err := filepath.Glob(filepath.Join(FSXRoot, "SimObjects/Airplanes/*/aircraft.cfg"))
	if err != nil {
		log.Fatal(err)
	}

	for _, path := range paths {
		models, err := AircraftConfig(path)
		if err != nil {
			log.Println(path, err)
			continue
		}
		for _, m := range models {
			if index[m.AirlineName] == nil {
				index[m.AirlineName] = make(map[string][]string)
			}
			index[m.AirlineName][m.Model] = append(index[m.AirlineName][m.Model], m.Title)
		}
	}

	wg.Wait()
	if err := saveMappings(index, db); err != nil {
		log.Println(err)
	}

	if !*watch {
		return
	}

	for range time.Tick(10 * time.Minute) {
		if err := updateDB(db); err != nil {
			log.Println(err)
			continue
		}
		if err := saveMappings(index, db); err != nil {
			log.Println(err)
		}
	}
}

func updateDB(db *Database) error {
	res, err := http.Get("http://api.vateud.net/online/pilots/ed.json")
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return errors.New(res.Status)
	}

	var m []APIStation

	json.NewDecoder(res.Body).Decode(&m)
	io.Copy(ioutil.Discard, res.Body)
	res.Body.Close()

	for _, x := range m {
		db.Add(x)
	}

	b, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile("icao.json", b, 0644)
}

type RuleSet struct {
	XMLName xml.Name `xml:"ModelMatchRuleSet"`
	Rules   []Rule
}

type Rule struct {
	XMLName    xml.Name `xml:"ModelMatchRule"`
	AL         string   `xml:"CallsignPrefix,attr"`
	AC         string   `xml:"TypeCode,attr"`
	Model      string   `xml:"ModelName,attr"`
	Substitute bool     `xml:"Substitute,attr,omitempty"`
}

func saveMappings(index map[string]map[string][]string, db *Database) error {
	rss := buildMapping2(index, db)

	for airline, rs := range rss {
		b, err := xml.MarshalIndent(rs, "", "  ")
		if err != nil {
			log.Fatal(err)
		}

		fname := filepath.Join("vPilot Files/Model Matching Rule Sets", airline+".vrm")
		if err := ioutil.WriteFile(fname, append([]byte(xml.Header), b...), 0644); err != nil {
			return err
		}
	}

	return nil
}

func buildMapping2(index map[string]map[string][]string, db *Database) map[string]*RuleSet {
	rss := make(map[string]*RuleSet)

	for al, acs := range db.All() {
		alName := db.AirlineNames[al]
		if alName == "" {
			log.Printf("No name for %s\n", al)
			continue
		}

		if len(index[alName]) == 0 {
			log.Printf("No models for %s %s; want %+v\n", al, alName, acs)
			continue
		}

		rs := &RuleSet{}
		rss[alName] = rs

		for ac, urgent := range acs {
			candidates := []string{ac}
			for _, alts := range db.AircraftAlts {
				if !alts[ac] {
					continue
				}
				for alt := range alts {
					candidates = append(candidates, alt)
				}
			}

			for i, c := range candidates {
				if titles := index[alName][c]; len(titles) > 0 {
					rs.Rules = append(rs.Rules, Rule{
						AL:         al,
						AC:         ac,
						Model:      strings.Join(titles, "//"),
						Substitute: i > 0,
					})
					goto next_model
				}
			}
			if urgent {
				log.Printf("No match: %s %s %s\n", al, alName, ac)
			}
		next_model:
		}

	}

	return rss
}

func dump(y interface{}) {
	b, err := json.MarshalIndent(y, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(b))
}
