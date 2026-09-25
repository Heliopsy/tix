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
		{"one listing", "v2~tasks:status.ref", map[string][]string{"tasks": {"status", "ref"}}},
		{"declared order wins", "v2~tasks:ref.status",
			map[string][]string{"tasks": {"status", "ref"}}},
		{"two listings", "v2~tasks:status~users:name",
			map[string][]string{"tasks": {"status"}, "users": {"name"}}},
		{"hides nothing", "v2~tasks:-", map[string][]string{"tasks": {}}},
		{"unknown listing", "v2~planets:mars", map[string][]string{}},
		{"unknown column", "v2~tasks:mars", map[string][]string{}},
		{"version alone", "v2", map[string][]string{}},
		{"no separator", "v2~tasks", map[string][]string{}},
		{"rubbish", "%%%~::~", map[string][]string{}},
		{"oversized", "v2~tasks:status" + string(make([]byte, maxColumnsValue)), map[string][]string{}},
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

// A cookie written while the value recorded the columns SHOWN keeps meaning
// what it meant. Read as the current form it would mean the opposite, so the
// one thing that must never happen is a silent inversion.
func TestParseColumnsConvertsTheShownFormRatherThanInvertingIt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want map[string][]string
	}{
		{"one column refused", "tasks:status.priority.tags.project.ref",
			map[string][]string{"tasks": {"updated"}}},
		{"everything the reader could see", "tasks:status.priority.tags.project.updated.ref",
			map[string][]string{"tasks": {}}},
		{"only one kept", "tasks:ref",
			map[string][]string{"tasks": {"status", "priority", "tags", "project", "updated"}}},
		{"the reader asked for none", "tasks:-",
			map[string][]string{"tasks": {"status", "priority", "tags", "project", "updated", "ref"}}},
		{"two listings", "tasks:ref~users:name.role.state",
			map[string][]string{"tasks": {"status", "priority", "tags", "project", "updated"},
				"users": {}}},
		{"unknown listing", "planets:mars", map[string][]string{}},
		{"unknown column", "tasks:mars", map[string][]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseColumns(tc.raw)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseColumns(%q) = %v, want %v", tc.raw, got, tc.want)
			}
			for _, hidden := range got {
				for _, key := range hidden {
					if key == "assignee" {
						t.Fatalf("parseColumns(%q) hid a column that did not exist when it was written", tc.raw)
					}
				}
			}
		})
	}
}

// The columns the shown form could name are exactly the ones this build can
// still place, so the conversion above is a rename away from being wrong.
func TestTheShownFormVocabularyIsStillDeclared(t *testing.T) {
	t.Parallel()
	for page, vocabulary := range legacyColumns {
		cols, ok := columnSets[page]
		if !ok {
			continue
		}
		declared := map[string]bool{}
		for _, c := range cols {
			declared[c.Key] = true
		}
		for _, key := range vocabulary {
			if !declared[key] {
				t.Errorf("listing %q no longer declares %q, so a converted cookie silently drops it", page, key)
			}
		}
	}
}

func TestEncodeColumnsRoundTrips(t *testing.T) {
	t.Parallel()
	hidden := map[string][]string{"tasks": {"status", "ref"}, "users": {}}
	encoded := encodeColumns(hidden)
	if encoded != "v2~tasks:status.ref~users:-" {
		t.Fatalf("encodeColumns = %q", encoded)
	}
	if got := parseColumns(encoded); !reflect.DeepEqual(got, hidden) {
		t.Fatalf("round trip = %v, want %v", got, hidden)
	}
	if got := encodeColumns(map[string][]string{}); got != "" {
		t.Fatalf("encoding nothing = %q, want an empty value", got)
	}
}

// The cap has to hold the largest value the picker itself can produce, which
// is every listing with every column hidden. A value over it is discarded, so
// a reader who filled the picker in would silently lose the lot.
func TestTheWidestChoiceFitsTheCookieCap(t *testing.T) {
	t.Parallel()
	widest := map[string][]string{}
	for page, cols := range columnSets {
		keys := make([]string, 0, len(cols))
		for _, c := range cols {
			keys = append(keys, c.Key)
		}
		widest[page] = keys
	}
	value := encodeColumns(widest)
	if len(value) > maxColumnsValue {
		t.Fatalf("the widest choice is %d bytes, over the %d the reader is allowed: %s",
			len(value), maxColumnsValue, value)
	}
	if got := parseColumns(value); !reflect.DeepEqual(got, widest) {
		t.Fatalf("the widest choice does not survive a round trip: %v", got)
	}
	t.Logf("widest choice: %d bytes of %d", len(value), maxColumnsValue)
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
	chosen := resolveColumns(cols, []string{"status"}, true)
	if chosen["status"] || !chosen["ref"] {
		t.Fatalf("a hidden set was not applied: %v", chosen)
	}
	all := resolveColumns(cols, []string{}, true)
	for key, shown := range all {
		if !shown {
			t.Errorf("column %q is hidden although the reader hid none", key)
		}
	}
	if !all["updated"] {
		t.Errorf("hiding nothing did not show the column that is off by default")
	}
}

// The reason the cookie records what is hidden: a column this build gains
// later is shown to a browser whose cookie was written before it existed,
// whichever form that cookie is in, and without the reader touching it.
func TestAColumnAddedLaterIsShownWithoutTouchingTheCookie(t *testing.T) {
	t.Parallel()
	later := append(append([]Column{}, columnSets["tasks"]...),
		Column{Key: "phase", Label: "Phase", Default: true})

	for _, raw := range []string{
		encodeColumns(map[string][]string{"tasks": {"updated"}}),
		"tasks:status.priority.tags.project.ref",
	} {
		hidden, chose := parseColumns(raw)["tasks"]
		if !chose {
			t.Fatalf("cookie %q was discarded rather than read", raw)
		}
		shown := resolveColumns(later, hidden, chose)
		if !shown["phase"] {
			t.Errorf("cookie %q hides a column written after it: %v", raw, shown)
		}
		if shown["updated"] {
			t.Errorf("cookie %q lost the column the reader did put away", raw)
		}
		if !shown["status"] || !shown["ref"] {
			t.Errorf("cookie %q lost a column the reader kept", raw)
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
