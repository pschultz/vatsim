package vPilot

import (
	"encoding/xml"
	"io/ioutil"
	"os"
	"path/filepath"
)

type RuleSets struct {
	dirname string
	sets    []*RuleSet
}

type RuleSet struct {
	XMLName xml.Name `xml:"ModelMatchRuleSet"`

	AirlineCode string `xml:"CallsignPrefix,attr"`
	Rules       []Rule

	dirty bool `xml:"-"`
}

type Rule struct {
	XMLName xml.Name `xml:"ModelMatchRule"`

	AirlineCode  string `xml:"CallsignPrefix,attr"`
	AircraftCode string `xml:"TypeCode,attr"`
	Title        string `xml:"ModelName,attr"`
	Substitute   bool   `xml:"Substitute,attr,omitempty"`
}

func ReadRuleSets(dirname string) (*RuleSets, error) {
	names, err := filepath.Glob(filepath.Join(dirname, "*.vrm"))

	if err != nil {
		return nil, err
	}

	rs := &RuleSets{
		dirname: dirname,
		sets:    make([]*RuleSet, 0, len(names)),
	}

	for _, name := range names {
		set, err := loadRuleSet(name)
		if err != nil {
			return nil, err
		}
		if set != nil {
			rs.sets = append(rs.sets, set)
		}
	}

	return rs, nil
}

func loadRuleSet(name string) (*RuleSet, error) {
	rs := &RuleSet{}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := xml.NewDecoder(f).Decode(rs); err != nil {
		return nil, err
	}

	if rs.AirlineCode == "" {
		return nil, nil
	}

	return rs, nil
}

func WriteRuleSets(sets *RuleSets) error {
	for _, set := range sets.sets {
		if !set.dirty {
			continue
		}
		if err := writeRuleSet(sets.dirname, set); err != nil {
			return nil
		}
	}
	return nil
}

func writeRuleSet(dirname string, set *RuleSet) error {
	b, err := xml.MarshalIndent(set, "", "\t")
	if err != nil {
		return err
	}

	name := filepath.Join(dirname, set.AirlineCode+".vrm")
	if err := ioutil.WriteFile(name, b, 0644); err != nil {
		return err
	}

	set.dirty = false
	return nil
}

func (rs *RuleSets) Add(rule Rule) {
	if rule.AirlineCode == "" {
		return
	}

	for _, set := range rs.sets {
		if set.AirlineCode == rule.AirlineCode {
			set.Add(rule)
			return
		}
	}

	rs.sets = append(rs.sets, &RuleSet{
		AirlineCode: rule.AirlineCode,
		dirty:       true,
		Rules:       []Rule{rule},
	})
}

func (rs *RuleSet) Add(rule Rule) {
	rs.dirty = true

	for i, r := range rs.Rules {
		if r.AircraftCode == rule.AircraftCode {
			rs.Rules[i] = rule
			return
		}
	}

	rs.Rules = append(rs.Rules, rule)

	rule.Substitute = true
	for _, ac := range substitutes(rule.AircraftCode) {
		for _, r := range rs.Rules {
			if r.AircraftCode == ac {
				goto next
			}
		}
		rule.AircraftCode = ac
		rs.Rules = append(rs.Rules, rule)
	next:
	}
}
