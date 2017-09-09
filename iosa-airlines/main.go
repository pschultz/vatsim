package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

type Operator struct {
	OperatorName  string
	OperatorCode  string
	TelephonyName string
	CountryName   string
	CountryCode   string
}

func main() {
	apiKey := os.Getenv("ICAO_API_KEY")

	ops, err := listIOSA(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	buckets := make([][]string, 0, len(ops)/10)
	var codes []string
	for _, op := range ops {
		codes = append(codes, op.OperatorCode)
		if len(codes) == 10 {
			buckets = append(buckets, codes)
			codes = nil
		}
	}
	buckets = append(buckets, codes)

	for _, b := range buckets {
		ops, err := lookupICAO(apiKey, b)
		if err != nil {
			log.Fatal(err)
		}

		for _, op := range ops {
			fmt.Printf("%s\t%s - %s\t%s\n",
				op.OperatorCode, op.OperatorName, op.CountryName, op.TelephonyName,
			)
		}
	}
}

var apiBaseURL = "https://v4p4sz5ijk.execute-api.us-east-1.amazonaws.com/anbdata"

func listIOSA(apiKey string) ([]Operator, error) {
	q := make(url.Values)
	q.Set("api_key", apiKey)
	q.Set("format", "json")

	u := apiBaseURL + "/airlines/designators/iosa-registry-list?" + q.Encode()
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return decode(res)
}

func lookupICAO(apiKey string, codes []string) ([]Operator, error) {
	q := make(url.Values)
	q.Set("api_key", apiKey)
	q.Set("format", "json")
	q.Set("states", "")
	q.Set("operators", strings.Join(codes, ","))

	u := apiBaseURL + "/airlines/designators/code-list?" + q.Encode()
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return decode(res)
}

func decode(res *http.Response) ([]Operator, error) {
	defer res.Body.Close()
	defer io.Copy(ioutil.Discard, res.Body)

	if res.StatusCode/100 != 2 {
		return nil, errors.New(res.Status)
	}

	var ops []Operator
	if err := json.NewDecoder(res.Body).Decode(&ops); err != nil {
		return nil, err
	}

	sort.Slice(ops, func(i, j int) bool {
		return ops[i].OperatorCode < ops[j].OperatorCode
	})
	return ops, nil
}
