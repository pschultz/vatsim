package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
)

func main() {
	q := make(url.Values)
	q.Set("dataSource", "metars")
	q.Set("format", "xml")
	q.Set("hoursBeforeNow", "1")
	q.Set("requestType", "retrieve")
	q.Set("stationString", "EDDT")

	res, err := http.Get("https://aviationweather.gov/adds/dataserver_current/httpparam?" + q.Encode())
	if err != nil {
		log.Fatal(err)
	}

	var m struct {
		XMLName xml.Name `xml:"response"`
		Data    struct {
			Metar []struct {
				Raw       string  `xml:"raw_text"`
				WindDir   float64 `xml:"wind_dir_degrees"`
				WindSpeed float64 `xml:"wind_speed_kt"`
			} `xml:"METAR"`
		} `xml:"data"`
	}

	if err := xml.NewDecoder(res.Body).Decode(&m); err != nil {
		log.Fatal(err)
	}

	if len(m.Data.Metar) < 1 {
		log.Fatal("empty metar")
	}

	metar := m.Data.Metar[len(m.Data.Metar)-1]
	tailwind := math.Cos((metar.WindDir-260.0)*math.Pi/180.0) * metar.WindSpeed * -1

	fmt.Println(metar.Raw)
	fmt.Printf("Tailwind for 26: %.2f\n", tailwind)
	if tailwind <= 5.0 {
		fmt.Println("In use: 26 L/R")
	} else {
		fmt.Println("In use: 08 L/R")
	}
}
