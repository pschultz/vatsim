// +build !es

package main

import (
	"fmt"
	"log"
	"unicode"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/analysis/token/ngram"
	"github.com/blevesearch/bleve/analysis/tokenizer/character"
	"github.com/blevesearch/bleve/registry"
)

func init() {
	registry.RegisterTokenizer("words", func(config map[string]interface{}, cache *registry.Cache) (analysis.Tokenizer, error) {
		return character.NewCharacterTokenizer(func(r rune) bool {
			return !unicode.IsSpace(r) && r != '_'
		}), nil
	})
}

func CreateIndex() (bleve.Index, error) {
	mapping := bleve.NewIndexMapping()

	err := mapping.AddCustomTokenFilter("icao-ngram", map[string]interface{}{
		"type": ngram.Name,
		"min":  3.0,
		"max":  4.0,
	})
	if err != nil {
		return nil, err
	}

	err = mapping.AddCustomAnalyzer("icao", map[string]interface{}{
		"type":      custom.Name,
		"tokenizer": "words",
		"token_filters": []string{
			"icao-ngram",
		},
	})
	if err != nil {
		return nil, err
	}

	icaoMapping := bleve.NewTextFieldMapping()
	icaoMapping.Analyzer = "icao"

	keywordMapping := bleve.NewTextFieldMapping()
	keywordMapping.Analyzer = keyword.Name

	woaiMapping := bleve.NewDocumentMapping()
	woaiMapping.AddFieldMappingsAt("title", icaoMapping)
	woaiMapping.AddFieldMappingsAt("sim", icaoMapping)
	woaiMapping.AddFieldMappingsAt("atc_airline", keywordMapping)
	woaiMapping.AddFieldMappingsAt("atc_model", keywordMapping)
	woaiMapping.AddFieldMappingsAt("ui_type", icaoMapping)

	mapping.AddDocumentMapping("woai", woaiMapping)

	if false {
		ts, _ := mapping.AnalyzeText("icao", []byte("TFS A330-200 PW Nordvind Airlines VP-BYV"))
		for _, t := range ts {
			fmt.Println(string(t.Term))
		}
	}
	//dump(mapping)

	return bleve.NewMemOnly(mapping)
}

func Index(index bleve.Index, typ, id string, doc map[string]string) error {
	if doc["atc_airline"] != "LUFTHANSA" {
		return nil
	}

	doc["_type"] = typ
	return index.Index(id, doc)
}

func Search(index bleve.Index, ac Model) ([]string, error) {
	// The airline *must* match, otherwise we give up immediately. We don't
	// care too much if the model is lightly off, but the paint must be
	// correct. So we first lookup all the IDs for the airline.

	if ac.ALKeyword == "" {
		return nil, fmt.Errorf("missing airline keyword: %v", ac.AL)
	}

	alQuery := bleve.NewTermQuery(ac.ALKeyword)
	alQuery.FieldVal = "atc_airline"

	search := bleve.NewSearchRequest(alQuery)
	search.Size = 1 << 30
	searchResults, err := index.Search(search)
	if err != nil {
		return nil, err
	}
	if searchResults.Total == 0 {
		return nil, nil
	}

	idQuery := bleve.NewDocIDQuery(nil)
	idQuery.IDs = make([]string, 0, searchResults.Total)
	for _, h := range searchResults.Hits {
		idQuery.IDs = append(idQuery.IDs, h.ID)
	}

	// Next, check if there is a perfect match in "atc_model".

	acQuery := bleve.NewTermQuery(ac.AC)
	acQuery.FieldVal = "atc_model"

	query := bleve.NewBooleanQuery()
	query.AddMust(idQuery)
	query.AddMust(acQuery)

	search = bleve.NewSearchRequest(query)
	search.Size = 1
	search.Fields = []string{"title"}
	searchResults, err = index.Search(search)
	if err != nil {
		return nil, err
	}
	if searchResults.Total == 1 {
		return titles(searchResults), nil
	}

	// Still no luck. Now search in _all and also try the fulltext terms.

	acQuery = bleve.NewTermQuery(ac.AC)

	query = bleve.NewBooleanQuery()
	query.AddMust(idQuery)
	query.AddShould(acQuery)
	for _, t := range ac.Terms {
		query.AddShould(bleve.NewMatchQuery(t))
	}
	query.SetMinShould(1.0)

	search = bleve.NewSearchRequest(query)
	search.Size = 1
	search.Fields = []string{"title"}
	searchResults, err = index.Search(search)
	if err != nil {
		return nil, err
	}

	return titles(searchResults), nil
}

func titles(searchResults *bleve.SearchResult) []string {
	var titles []string
	for _, h := range searchResults.Hits {
		switch x := h.Fields["title"].(type) {
		case string:
			titles = append(titles, x)
		case []string:
			titles = append(titles, x...)
		default:
			panic(fmt.Sprintf("unexpected type for 'title': %T", h.Fields["title"]))
		}
	}
	return titles
}

func DumpAll(index bleve.Index) {
	query := bleve.NewMatchAllQuery()
	search := bleve.NewSearchRequest(query)
	search.Fields = []string{"*"}
	search.Size = 1 << 30
	searchResults, err := index.Search(search)
	if err != nil {
		log.Fatal(err)
	}
	dump(searchResults)
}
