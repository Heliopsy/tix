// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"reflect"
	"testing"
)

func TestParseColumnsKeepsOnlyWhatIsDeclared(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want map[string][]string
	}{
		{"empty", "", map[string][]string{}},
		{"one listing", "tasks:status.ref", map[string][]string{"tasks": {"status", "ref"}}},
		{"declared order wins", "tasks:ref.status",
			map[string][]string{"tasks": {"status", "ref"}}},
		{"two listings", "tasks:status~users:name",
			map[string][]string{"tasks": {"status"}, "users": {"name"}}},
		{"explicit none", "tasks:-", map[string][]string{"tasks": {}}},
		{"unknown listing", "planets:mars", map[string][]string{}},
		{"unknown column", "tasks:mars", map[string][]string{}},
		{"no separator", "tasks", map[string][]string{}},
		{"rubbish", "%%%~::~", map[string][]string{}},
		{"oversized", "tasks:status" + string(make([]byte, maxColumnsValue)), map[string][]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := parseColumns(tc.raw); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseColumns(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestEncodeColumnsRoundTrips(t *testing.T) {
	t.Parallel()
	chosen := map[string][]string{"tasks": {"status", "ref"}, "users": {}}
	encoded := encodeColumns(chosen)
	if encoded != "tasks:status.ref~users:-" {
		t.Fatalf("encodeColumns = %q", encoded)
	}
	if got := parseColumns(encoded); !reflect.DeepEqual(got, chosen) {
		t.Fatalf("round trip = %v, want %v", got, chosen)
	}
	if got := encodeColumns(map[string][]string{}); got != "" {
		t.Fatalf("encoding nothing = %q, want an empty value", got)
	}
}

func TestResolveColumnsFallsBackToTheDeclaredDefault(t *testing.T) {
	t.Parallel()
	cols := columnSets["tasks"]
	unchosen := resolveColumns(cols, nil, false)
	for _, c := range cols {
		if unchosen[c.Key] != c.Default {
			t.Errorf("column %q defaults to %v, want %v", c.Key, unchosen[c.Key], c.Default)
		}
	}
	chosen := resolveColumns(cols, []string{"ref"}, true)
	if !chosen["ref"] || chosen["status"] {
		t.Fatalf("a chosen set was not applied: %v", chosen)
	}
	none := resolveColumns(cols, []string{}, true)
	for key, shown := range none {
		if shown {
			t.Errorf("column %q is shown although none were chosen", key)
		}
	}
}

// Every listing keeps one column outside its set, so no choice can leave a
// table with no cells in it, and the picker page name has to match a template.
func TestEveryColumnSetIsWellFormed(t *testing.T) {
	t.Parallel()
	for page, cols := range columnSets {
		if !HasTemplate(page + ".html") {
			t.Errorf("column set %q names no embedded template", page)
		}
		if len(cols) == 0 {
			t.Errorf("column set %q declares no columns", page)
		}
		seen := map[string]bool{}
		for _, c := range cols {
			if c.Key == "" || c.Label == "" {
				t.Errorf("column set %q has an unnamed column", page)
			}
			if seen[c.Key] {
				t.Errorf("column set %q declares %q twice", page, c.Key)
			}
			seen[c.Key] = true
		}
	}
	if columnPage("tasks.html") != "tasks" {
		t.Fatalf("columnPage does not name the listing a template renders")
	}
}
