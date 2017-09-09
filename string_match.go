package vatsim

import (
	"sort"

	"github.com/agext/levenshtein"
)

func FindMatches(want string, have [][2]string) (exact bool, others [][2]string) {
	matches := make(similar, 0)
	for _, x := range have {
		k := levenshtein.Match(want, x[0], nil)
		if k == 1.0 {
			return true, nil
		}
		if k >= .8 {
			matches.Add(k, x)
		}
	}
	sort.Sort(matches)
	for _, m := range matches {
		others = append(others, m.value)
	}
	return exact, others
}

type similar []struct {
	score float64
	value [2]string
}

func (s similar) Len() int           { return len(s) }
func (s similar) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s similar) Less(i, j int) bool { return s[j].score < s[i].score }
func (s *similar) Add(score float64, value [2]string) {
	*s = append(*s, struct {
		score float64
		value [2]string
	}{score, value})
}
