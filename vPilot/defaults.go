package vPilot

var DefaultRuleDirectory = "vPilot Files/Model Matching Rule Sets"
var DefaultModelMatchingFile = "vPilot/ModelMatchingData.xml.gz"

func LoadDefaults() (*RuleSets, error) {
	if err := LoadModelMatchingFile(DefaultModelMatchingFile); err != nil {
		return nil, err
	}

	return ReadRuleSets(DefaultRuleDirectory)
}
