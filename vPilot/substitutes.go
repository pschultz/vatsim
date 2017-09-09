package vPilot

import (
	"compress/gzip"
	"encoding/xml"
	"os"
	"strings"

	"github.com/pschultz/vatsim"
)

var modelSubs []vatsim.Set

func LoadModelMatchingFile(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer r.Close()

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
		set := vatsim.NewSet(codes...)
		if set.Has("CRJ2") {
			set.Add("CRJ200")
		}
		modelSubs = append(modelSubs, set)
	}

	return nil
}

func substitutes(code string) []string {
	for _, s := range modelSubs {
		if s.Has(code) {
			return s.All()
		}
	}
	return nil
}
