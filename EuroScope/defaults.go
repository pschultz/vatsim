package euroscope

var DefaultAirlinesFile = "EuroScope/IOSA_Airlines.txt"
var DefaultAircraftFile = "EuroScope/EDBB/ICAO_Aircraft.txt"

func LoadDefaults() error {
	if err := LoadAirlinesFile(DefaultAirlinesFile); err != nil {
		return err
	}
	if err := LoadAircraftFile(DefaultAircraftFile); err != nil {
		return err
	}
	return nil
}
