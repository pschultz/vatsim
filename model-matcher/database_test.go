package main

import "testing"

func TestParseCallsign(t *testing.T) {
	cases := []struct {
		given, want string
	}{
		{"", ""},
		{"X", ""},
		{"AXX373", "AXX"},
		{"CLX22F", "CLX"},
		{"DLH543", "DLH"},
		{"DLH8279", "DLH"},
		{"DLH89H", "DLH"},
		{"ELY354", "ELY"},
		{"GAF003", "GAF"},
		{"GEC8222B", "GEC"},
		{"GEC8396", "GEC"},
		{"GMI6311", "GMI"},
		{"GTI156", "GTI"},
		{"LHA3453", "LHA"},
		{"RYR9986", "RYR"},
		{"RYR9JK", "RYR"},
		{"SCG037", "SCG"},
		{"SXD34V", "SXD"},
		{"THY22D", "THY"},
		{"TUA463", "TUA"},
		{"TUI2810", "TUI"},
		{"TVF9624", "TVF"},
		{"UAE44", "UAE"},
		{"UAL961", "UAL"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.given, func(t *testing.T) {
			t.Parallel()

			if want, got := tc.want, parseCallsign(tc.given); want != got {
				t.Errorf("parseCallsign(%q) == %q, want %q", tc.given, got, want)
			}
		})
	}
}

func TestParseAircraft(t *testing.T) {
	cases := []struct {
		given, want string
	}{
		// http://www.flugzeuginfo.net/table_accodes_en.php
		{"738/L", ""},
		{"772/A", ""},
		{"A318", "A318"},
		{"A318/L", "A318"},
		{"A319", "A319"},
		{"A319/L", "A319"},
		{"A320", "A320"},
		{"A320/G", "A320"},
		{"A320/L", "A320"},
		{"A320/W", "A320"},
		{"A321", "A321"},
		{"A321/X", "A321"},
		{"B60T", "B60T"},
		{"B733/L", "B733"},
		{"B737", "B737"},
		{"B737/L", "B737"},
		{"B738", "B738"},
		{"B738/F", "B738"},
		{"B738/L", "B738"},
		{"B738/O", "B738"},
		{"B738/Q", "B738"},
		{"B738/R", "B738"},
		{"B738/S", "B738"},
		{"B738/W", "B738"},
		{"B738/X", "B738"},
		{"B739", "B739"},
		{"B739/L", "B739"},
		{"B744", "B744"},
		{"B744/H", "B744"},
		{"B747/L", "B747"},
		{"B752/L", "B752"},
		{"B763/L", "B763"},
		{"B772", "B772"},
		{"B773", "B773"},
		{"B777", "B777"},
		{"B777F", ""},
		{"B77F", "B77F"},
		{"B77W", "B77W"},
		{"B77W/L", "B77W"},
		{"C172", "C172"},
		{"CRJ7/L", "CRJ7"},
		{"CRJ9/F", "CRJ9"},
		{"CRJ9/L", "CRJ9"},
		{"DA20", "DA20"},
		{"DH8D", "DH8D"},
		{"DH8D/G", "DH8D"},
		{"DH8D/L", "DH8D"},
		{"GL5T", "GL5T"},
		{"H/738", ""},
		{"H/A319", "A319"},
		{"H/A320", "A320"},
		{"H/A332/L", "A332"},
		{"H/A332/X", "A332"},
		{"H/A333", "A333"},
		{"H/A359/L", "A359"},
		{"H/B738", "B738"},
		{"H/B738/M", "B738"},
		{"H/B738/Q", "B738"},
		{"H/B744/", ""},
		{"H/B744", "B744"},
		{"H/B744/L", "B744"},
		{"H/B744/S", "B744"},
		{"H/B748/L", "B748"},
		{"H/B77F", "B77F"},
		{"H/B77L", "B77L"},
		{"H/B77L/L", "B77L"},
		{"H/B77L/X", "B77L"},
		{"H/B77W", "B77W"},
		{"H/B77W/H", "B77W"},
		{"H/B77W/L", "B77W"},
		{"H/B789/L", "B789"},
		{"H/H/L", ""},
		{"JS41/G", "JS41"},
		{"M/A320/E", "A320"},
		{"M/A321/L", "A321"},
		{"MD82/F", "MD82"},
		{"M/CRJ2/Q", "CRJ2"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.given, func(t *testing.T) {
			t.Parallel()

			if want, got := tc.want, parseAircraft(tc.given); want != got {
				t.Logf("%q %+v", tc.given, aircraftPattern.FindStringSubmatch(tc.given))
				t.Errorf("parseAircraft(%q) == %q, want %q", tc.given, got, want)
			}
		})
	}
}
