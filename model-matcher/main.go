package main

import (
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
	"time"

	"github.com/blevesearch/bleve"
)

var FSXRoot = "/mnt/c/Program Files (x86)/Steam/steamapps/common/FSX"

func main() {
	if d := os.Getenv("FSX_ROOT"); d != "" {
		FSXRoot = d
	}

	watch := flag.Bool("watch", false, "Monitor the vatsim API and update the ruleset when never-seen-before aircraft appear.")
	flag.Parse()

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

	// TODO: Figure out which charset works for the txt files. It's something
	// exotic. See for instance db.EuroScopeICAO["LAU"][0]. That's supposed to be
	// "Líneas Aéreas Suramericanas - Colombia"
	//fmt.Println(db.EuroScopeICAO["LAU"][0], []byte(db.EuroScopeICAO["LAU"][0][:8]))

	index, err := CreateIndex()
	if err != nil {
		log.Fatal(err)
	}

	paths, err := filepath.Glob(filepath.Join(FSXRoot, "SimObjects/Airplanes/*/aircraft.cfg"))
	if err != nil {
		log.Fatal(err)
	}

	for _, path := range paths {
		cfg, err := AircraftConfig(path)
		if err != nil {
			log.Println(path, err)
			continue
		}
		for id, doc := range cfg {
			if err := Index(index, "woai", id, doc); err != nil {
				log.Fatal(path, err)
			}
		}
	}

	//DumpAll(index)
	//os.Exit(0)

	if err := updateDB(db); err != nil {
		log.Println(err)
	}
	rs, err := buildMapping(index, db)
	if err != nil {
		log.Println(err)
	}

	if b, err := xml.MarshalIndent(rs, "", "  "); err != nil {
		log.Println(err)
	} else {
		fmt.Print(xml.Header)
		fmt.Println(string(b))
	}

	if !*watch {
		return
	}

	for range time.Tick(10 * time.Minute) {
		if err := updateDB(db); err != nil {
			log.Println(err)
			continue
		}
		rs, err = buildMapping(index, db)
		if err != nil {
			log.Println(err)
			continue
		}

		b, err := xml.MarshalIndent(rs, "", "  ")
		if err != nil {
			log.Println(err)
			continue
		}

		fmt.Print(xml.Header)
		fmt.Println(string(b))
	}
}

func updateDB(db *Database) error {
	res, err := http.Get("http://api.vateud.net/online/pilots/e.json")
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

func buildMapping(index bleve.Index, db *Database) (*RuleSet, error) {
	rs := &RuleSet{}

	for _, ac := range db.All() {
		switch ac.AL {
		case "DLH": //, "QTR", "BAW":
		default:
			continue
		}

		titles, err := Search(index, ac)
		if err != nil {
			return nil, err
		}

		if len(titles) == 0 {
			log.Printf("No match: %s %s %#v\n", ac.AL, ac.AC, ac.Terms)
			continue
		}

		substitute := false
		if substitute {
			titles = titles[:1]
		}
		rs.Rules = append(rs.Rules, Rule{
			AL:         ac.AL,
			AC:         ac.AC,
			Model:      strings.Join(titles, "//"),
			Substitute: substitute,
		})
	}

	return rs, nil
}

func search(index bleve.Index, ac Model) ([]string, bool, error) {
	titles, err := Search(index, ac)
	if err != nil {
		return nil, false, err
	}

	if len(titles) > 0 {
		return titles, false, err
	}

	if len(ac.AC) == 4 {
		switch ac.AC[:3] {
		case "B73", "B74", "B75", "B77", "B78":
			ac.AC = ac.AC[:3]
			title, _, err := search(index, ac)
			return title, true, err
		}
	}

	switch ac.AC {
	case "B73":
		ac.AC = "B75"
	case "B75":
		ac.AC = "B77"
	case "B77":
		ac.AC = "B78"
	default:
		return titles, false, nil
	}

	title, _, err := search(index, ac)
	return title, true, err
}

func dump(y interface{}) {
	b, err := json.MarshalIndent(y, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(b))
}
