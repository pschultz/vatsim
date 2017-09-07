package vatsim

import "sort"

type Set map[string]struct{}

func NewSet(xs ...string) Set {
	s := make(Set, len(xs))
	s.Add(xs...)
	return s
}

func (s Set) Add(xs ...string) {
	for _, x := range xs {
		s[x] = struct{}{}
	}
}

func (s Set) Has(x string) bool {
	_, ok := s[x]
	return ok
}

func (s Set) All() []string {
	members := make([]string, 0, len(s))
	for x := range s {
		members = append(members, x)
	}
	sort.Strings(members)
	return members
}
