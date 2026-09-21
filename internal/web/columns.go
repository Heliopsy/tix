package web

import (
	"net/http"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// Column is one optional column of a listing: the key the picker submits, the
// word the control is labelled with, and whether an untouched install shows it.
type Column struct {
	Key     string
	Label   string
	Default bool
}

// ColumnsCookie carries which optional columns each listing shows. One cookie
// holds every listing, because a cookie per screen would multiply with the
// screens and eventually exceed what a browser will send back.
const ColumnsCookie = "tix_columns"

// maxColumnsValue bounds the cookie this package will read, so a value another
// program left behind cannot make parsing walk a large string.
const maxColumnsValue = 512

// noColumns marks a listing whose optional columns were all switched off, so
// that choice survives a round trip instead of reading as an absent one.
const noColumns = "-"

// columnSets declares every listing whose columns can be chosen, in the order
// they render, and is the only place a default is written down. A listing not
// named here has no picker; a column not named here cannot be hidden. Each
// listing keeps one column outside this set -- the one naming the row and
// carrying its link -- so no choice can produce a listing with nothing in it
// or put a record out of reach.
var columnSets = map[string][]Column{
	"tasks": {
		{Key: "status", Label: "Status", Default: true},
		{Key: "priority", Label: "Priority", Default: true},
		{Key: "tags", Label: "Tags", Default: true},
		{Key: "list", Label: "List"},
		{Key: "updated", Label: "Updated"},
		{Key: "ref", Label: "Reference", Default: true},
	},
	"users": {
		{Key: "name", Label: "Name", Default: true},
		{Key: "state", Label: "State", Default: true},
	},
	"tokens": {
		{Key: "scopes", Label: "Scopes", Default: true},
		{Key: "expires", Label: "Expires", Default: true},
	},
	"webhooks": {
		{Key: "events", Label: "Events", Default: true},
		{Key: "active", Label: "Active", Default: true},
	},
	"domains": {
		{Key: "verified", Label: "Verified", Default: true},
		{Key: "cert", Label: "Certificate", Default: true},
	},
	"projects": {
		{Key: "name", Label: "Name", Default: true},
		{Key: "colour", Label: "Colour", Default: true},
		{Key: "state", Label: "State", Default: true},
		{Key: "screens", Label: "Screens", Default: true},
		{Key: "manage", Label: "Manage", Default: true},
	},
}

// columnPrefs is the resolved choice for every listing, with the declared
// default filled in wherever the browser has not chosen.
type columnPrefs map[string]map[string]bool

// Show reports whether a listing renders one of its optional columns.
func (p columnPrefs) Show(page, key string) bool { return p[page][key] }

// Configurable reports whether a screen offers a column picker.
func (p columnPrefs) Configurable(page string) bool {
	_, ok := columnSets[page]
	return ok
}

// Options returns the picker's checkboxes for one listing.
func (p columnPrefs) Options(page string) []columnOption {
	cols := columnSets[page]
	out := make([]columnOption, 0, len(cols))
	for _, c := range cols {
		out = append(out, columnOption{Column: c, Shown: p[page][c.Key]})
	}
	return out
}

// Span is how many cells a row spanning the whole table covers, given the
// columns that listing always renders.
func (p columnPrefs) Span(page string, fixed int) int {
	out := fixed
	for _, c := range columnSets[page] {
		if p[page][c.Key] {
			out++
		}
	}
	return out
}

// columnOption is one checkbox of the picker.
type columnOption struct {
	Column
	Shown bool
}

// columnPage names the column set a template belongs to.
func columnPage(template string) string {
	return strings.TrimSuffix(template, ".html")
}

// columnsOf resolves what this browser asked for, applying the declared
// default to every listing it has not chosen.
func columnsOf(r *http.Request) columnPrefs {
	chosen := parseColumns(cookieValue(r, ColumnsCookie))
	out := make(columnPrefs, len(columnSets))
	for page, cols := range columnSets {
		picked, ok := chosen[page]
		out[page] = resolveColumns(cols, picked, ok)
	}
	return out
}

// resolveColumns applies a chosen set, falling back to the declared default
// when this browser has chosen nothing for the listing.
func resolveColumns(cols []Column, chosen []string, chose bool) map[string]bool {
	out := make(map[string]bool, len(cols))
	if !chose {
		for _, c := range cols {
			out[c.Key] = c.Default
		}
		return out
	}
	want := make(map[string]bool, len(chosen))
	for _, key := range chosen {
		want[key] = true
	}
	for _, c := range cols {
		out[c.Key] = want[c.Key]
	}
	return out
}

// parseColumns reads the compact cookie form, "page:col.col~page:col", keeping
// only the listings and columns this build declares. Anything else is dropped,
// so a stale, truncated or hand-edited value falls back to the default rather
// than failing the page or emptying a table.
func parseColumns(raw string) map[string][]string {
	out := map[string][]string{}
	if raw == "" || len(raw) > maxColumnsValue {
		return out
	}
	for _, entry := range strings.Split(raw, "~") {
		page, list, found := strings.Cut(entry, ":")
		cols, known := columnSets[page]
		if !found || !known {
			continue
		}
		if list == noColumns {
			out[page] = []string{}
			continue
		}
		if keep := knownColumns(cols, strings.Split(list, ".")); len(keep) > 0 {
			out[page] = keep
		}
	}
	return out
}

// knownColumns keeps the named columns the listing declares, in declared order.
func knownColumns(cols []Column, names []string) []string {
	want := make(map[string]bool, len(names))
	for _, name := range names {
		want[strings.TrimSpace(name)] = true
	}
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if want[c.Key] {
			out = append(out, c.Key)
		}
	}
	return out
}

// encodeColumns renders the chosen listings back into the cookie's form.
func encodeColumns(chosen map[string][]string) string {
	pages := make([]string, 0, len(chosen))
	for page := range chosen {
		pages = append(pages, page)
	}
	sort.Strings(pages)
	parts := make([]string, 0, len(pages))
	for _, page := range pages {
		list := noColumns
		if len(chosen[page]) > 0 {
			list = strings.Join(chosen[page], ".")
		}
		parts = append(parts, page+":"+list)
	}
	return strings.Join(parts, "~")
}

// cookieValue reads one cookie, or an empty string when the browser sent none.
func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// setColumns records which columns one listing shows for this browser. It is a
// display choice, so it lives in a cookie beside the theme rather than in the
// tenant's data: two people sharing an account read a list differently, and
// hiding a column withholds nothing, since every value stays on the record's
// own screen.
func (h *handler) setColumns(w http.ResponseWriter, r *http.Request) error {
	page := field(r, "page")
	cols, ok := columnSets[page]
	if !ok {
		return core.Invalid("no listing named %q has columns to choose", page)
	}
	chosen := parseColumns(cookieValue(r, ColumnsCookie))
	if checked(r, "reset") {
		delete(chosen, page)
	} else {
		chosen[page] = knownColumns(cols, r.PostForm["column"])
	}
	value := encodeColumns(chosen)
	age := cookieYear
	if value == "" {
		age = -1
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: ColumnsCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}
