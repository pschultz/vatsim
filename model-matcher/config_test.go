package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseIni(t *testing.T) {
	cases := []struct {
		given string
		want  map[string]map[string]string
	}{
		{
			given: ``,
			want:  map[string]map[string]string{},
		},
		{
			given: `[general]
foo=bar`,
			want: map[string]map[string]string{
				"general": map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			given: `[general]
foo = bar`,
			want: map[string]map[string]string{
				"general": map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			given: `[general]
Foo = Bar`,
			want: map[string]map[string]string{
				"general": map[string]string{
					"foo": "Bar",
				},
			},
		},
		{
			given: `[general]
Foo = Bar
Baz`,
			want: map[string]map[string]string{
				"general": map[string]string{
					"foo": "Bar\nBaz",
				},
			},
		},
		{
			given: `[General]
foo=bar

[Specific]
foo=baz`,
			want: map[string]map[string]string{
				"general": map[string]string{
					"foo": "bar",
				},
				"specific": map[string]string{
					"foo": "baz",
				},
			},
		},
		{
			given: `[general] // comment
foo = bar`,
			want: map[string]map[string]string{
				"general": map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			given: `all content before the first section should be ignored
			
[general]
foo = bar`,
			want: map[string]map[string]string{
				"general": map[string]string{
					"foo": "bar",
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			got, err := ParseIni(strings.NewReader(tc.given))
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(map[string]map[string]string(got), tc.want) {
				t.Errorf("%+v != %+v", got, tc.want)
			}
		})
	}
}
