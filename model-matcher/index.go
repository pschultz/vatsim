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
	woaiMapping.AddFieldMappingsAt("atc_airline", icaoMapping)
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
	doc["_type"] = typ
	return index.Index(id, doc)
}

func Search(index bleve.Index, ac Model) ([]string, error) {
	query := bleve.NewBooleanQuery()
	query.AddMust(bleve.NewTermQuery(ac.AL))
	query.AddMust(bleve.NewTermQuery(ac.AC))
	if len(ac.Terms) > 0 {
		for _, t := range ac.Terms {
			query.AddShould(bleve.NewMatchQuery(t))
		}
		query.SetMinShould(0.0)
	}
	search := bleve.NewSearchRequest(query)
	search.Size = 1 << 30
	search.Fields = []string{"title"}
	searchResults, err := index.Search(search)
	if err != nil {
		return nil, err
	}

	var titles []string
	for _, h := range searchResults.Hits {
		switch x := h.Fields["title"].(type) {
		case string:
			titles = append(titles, x)
		case []string:
			titles = append(titles, x...)
		default:
			return nil, fmt.Errorf("unexpected type for 'title': %T", h.Fields["title"])
		}
	}
	return titles, nil
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
