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

		metas, err := fsx.InspectConfig(f)
		f.Close()
		if err != nil {
			log.Fatal(err)
		}

		for _, m := range metas {
			mm.Add(vPilot.Rule{
				AircraftCode: m.ATCModel,
				AirlineCode:  euroscope.AirlineCode(m.ATCAirline),
				Title:        m.Title,
			})
		}
	}

	vPilot.WriteRuleSets(mm)
}
