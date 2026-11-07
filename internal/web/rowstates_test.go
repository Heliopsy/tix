// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rowStates are the only rules that may style a table row, or a row of a list
// read as a table, by its pointer, focus, fragment or mark, each with the one
// body it may carry. Everything else that once tinted, ringed, barred or
// lifted a row is gone, and a new variant has to replace one of these.
var rowStates = map[string]string{
	`:root:not([data-hover="off"]) .rows > tbody > tr:hover`:  "background-color: var(--row-hover);",
	`:root:not([data-hover="off"]) .rows > li:hover`:          "background-color: var(--row-hover);",
	`.rows > tbody > tr:has(:focus-visible)`:                  "background-color: var(--row-hover);",
	`.rows > li:has(:focus-visible)`:                          "background-color: var(--row-hover);",
	`.rows > tbody > tr:is(:target, .is-target, .is-flashed)`: "animation: row-flash 1.6s ease-out;",
	`.rows > li:is(:target, .is-target, .is-flashed)`:         "animation: row-flash 1.6s ease-out;",
	`.is-target`:                       "animation: row-flash 1.6s ease-out;",
	`.is-flashed`:                      "animation: row-flash 1.6s ease-out;",
	`.rows > li:focus-visible`:         "outline: 2px solid var(--accent); outline-offset: -2px; background-color: var(--row-selected);",
	`.rows > tbody > tr:focus-visible`: "outline: 2px solid var(--accent); outline-offset: -2px; background-color: var(--row-selected);",
	`.rows > li.is-selected`:           "background-color: var(--row-selected);",
	`.rows > tbody > tr.is-selected`:   "background-color: var(--row-selected);",
}

// rowCompound is a compound selector naming a row: a table row, a list item,
// or one of the classes the row-shaped lists give their items.
var rowCompound = regexp.MustCompile(`^(tr|li)\b|\.(projrow|setrow|recrow|person|task|channelrow|inbox-item)(?:$|[.:\[])`)

// rowState is a pseudo-class or mark that styles a row for what is happening
// to it rather than what it is.
var rowState = regexp.MustCompile(`:hover|:focus-within|:focus-visible|:target|\.is-target\b|\.is-flashed\b|\.is-selected\b`)

// splitTop splits s on sep where sep is outside any parentheses or brackets,
// so a comma inside :is() does not end a selector and a space inside :has()
// is not a combinator.
func splitTop(s string, sep func(rune) bool) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch {
		case r == '(' || r == '[':
			depth++
		case r == ')' || r == ']':
			depth--
		case depth == 0 && sep(r):
			out = append(out, s[start:i])
			start = i + len(string(r))
		}
	}
	return append(out, s[start:])
}

// lastCompound is the compound a selector styles: the one after its last
// combinator, read outside any parentheses.
func lastCompound(sel string) string {
	parts := splitTop(sel, func(r rune) bool { return r == ' ' || r == '>' || r == '+' || r == '~' })
	for i := len(parts) - 1; i >= 0; i-- {
		if p := strings.TrimSpace(parts[i]); p != "" {
			return p
		}
	}
	return ""
}

// TestRowsTakeOnlyTheStandardStates is the one set of row states: every rule
// in the stylesheet whose subject is a row and which styles it on hover,
// focus, a fragment or a mark is one of rowStates, with its body, and no row
// is given a pointer cursor.
func TestRowsTakeOnlyTheStandardStates(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, rule := range cssRules(readSheet(t)) {
		if strings.HasPrefix(rule.media, "@keyframes") {
			continue
		}
		for _, sel := range splitTop(rule.selector, func(r rune) bool { return r == ',' }) {
			sel = strings.Join(strings.Fields(sel), " ")
			subject := lastCompound(sel)
			isRow := rowCompound.MatchString(subject)
			if isRow && strings.Contains(rule.body, "cursor") {
				t.Errorf("%q gives a row a cursor; a row that leads somewhere says so with its link", sel)
			}
			if !rowState.MatchString(subject) || (!isRow && !strings.HasPrefix(subject, ".is-")) {
				continue
			}
			want, ok := rowStates[sel]
			if !ok {
				t.Errorf("%q styles a row's state outside the standard set (internal/web/rowstates_test.go): %s",
					sel, strings.TrimSpace(rule.body))
				continue
			}
			seen[sel] = true
			if got := strings.Join(strings.Fields(rule.body), " "); got != want {
				t.Errorf("%q carries %q, want only %q", sel, got, want)
			}
		}
	}
	for sel := range rowStates {
		if !seen[sel] {
			t.Errorf("the stylesheet has no %q, one of the standard row states", sel)
		}
	}
}

// rowLists are the lists drawn as tables, by a class each one carries.
var rowLists = []string{"tasklist", "peoplelist", "reclist", "projlist", "rellist", "feed", "inbox-items", "channels"}

var (
	tableTag = regexp.MustCompile(`<table\b[^>]*>`)
	listTag  = regexp.MustCompile(`<ul\s+class="([^"]*)"`)
)

// TestEveryTableTakesTheStandardRowStates holds the templates to the same
// set: every table, and every list drawn as one, carries the class rows the
// states hang from, so none of them falls back to a look of its own.
func TestEveryTableTakesTheStandardRowStates(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("templates/*.html")
	if err != nil || len(files) < 20 {
		t.Fatalf("reading the templates: %v (%d files)", err, len(files))
	}
	tables, lists := 0, 0
	for _, file := range files {
		src, err := os.ReadFile(file) // #nosec G304 -- a template of this package
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(file)
		for _, tag := range tableTag.FindAllString(string(src), -1) {
			tables++
			if !hasClass(tag, "rows") {
				t.Errorf("%s: %s does not carry the class rows", name, tag)
			}
		}
		for _, m := range listTag.FindAllStringSubmatch(string(src), -1) {
			for _, list := range rowLists {
				if !hasClassIn(m[1], list) {
					continue
				}
				lists++
				if !hasClassIn(m[1], "rows") {
					t.Errorf("%s: <ul class=%q> is drawn as a table and does not carry the class rows", name, m[1])
				}
			}
		}
	}
	if tables < 15 || lists < 10 {
		t.Fatalf("read %d tables and %d lists, so this guard reads nothing", tables, lists)
	}
}

func hasClass(tag, class string) bool {
	m := regexp.MustCompile(`class="([^"]*)"`).FindStringSubmatch(tag)
	return m != nil && hasClassIn(m[1], class)
}

func hasClassIn(list, class string) bool {
	for _, c := range strings.Fields(list) {
		if c == class {
			return true
		}
	}
	return false
}

// A dialog button is found by the id it names, and a row carrying the same id
// came first: Manage on the user listing found the row, not the dialog, so the
// click fell through to the link, the address kept #user-ID and the row the
// fragment named stayed ringed after the dialog closed. Every button's id is
// its dialog's, and the first element with it.
func TestEveryDialogButtonNamesItsDialogFirst(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	created := b.post("/admin/users", url.Values{"email": {"rows@example.test"},
		"password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	openers := regexp.MustCompile(`data-dlg-open="([^"]+)"`)
	paths := []string{"/admin/users"}
	for _, tc := range createDialogs {
		paths = append(paths, tc.path)
	}
	checked := 0
	for _, path := range paths {
		page := b.page(path)
		for _, m := range openers.FindAllStringSubmatch(page, -1) {
			at := strings.Index(page, ` id="`+m[1]+`"`)
			if at < 0 {
				t.Errorf("%s: no element has the id %q its button opens", path, m[1])
				continue
			}
			checked++
			if open := strings.LastIndex(page[:at], "<"); !strings.HasPrefix(page[open:], "<dialog ") {
				t.Errorf("%s: the first element with id %q is %s, not its dialog", path, m[1], page[open:at+len(m[1])+6])
			}
		}
	}
	if checked < len(paths)+2 {
		t.Fatalf("checked %d buttons, so this guard reads nothing", checked)
	}
}
