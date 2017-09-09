package main

import (
	"flag"
	"log"
	"os"

	"github.com/pschultz/vatsim/EuroScope"
	"github.com/pschultz/vatsim/fsx"
	"github.com/pschultz/vatsim/vPilot"
)

func main() {
	flag.Parse()

	if err := euroscope.LoadDefaults(); err != nil {
		log.Fatal(err)
	}

	mm, err := vPilot.LoadDefaults()
	if err != nil {
		log.Fatal(err)
	}

	for _, filename := range flag.Args() {
		f, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}

		atcModel, atcAirline, titles, err := fsx.InspectConfig(f)
		f.Close()
		if err != nil {
			log.Fatal(err)
		}

		rule := vPilot.Rule{
			AircraftCode: atcModel,
			AirlineCode:  euroscope.AirlineCode(atcAirline),
			Title:        titles[0],
		}

		mm.Add(rule)
	}

	vPilot.WriteRuleSets(mm)
}
