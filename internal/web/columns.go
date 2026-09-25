// SPDX-License-Identifier: AGPL-3.0-or-later

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

// ColumnsCookie carries which optional columns each listing leaves out. One
// cookie holds every listing, because a cookie per screen would multiply with
// the screens and eventually exceed what a browser will send back.
const ColumnsCookie = "tix_columns"

// maxColumnsValue bounds the cookie this package will read, so a value another
// program left behind cannot make parsing walk a large string.
const maxColumnsValue = 512

// emptyList marks a listing whose hidden set is empty, so "hide nothing" is
// distinguishable from "this browser has chosen nothing". They are different:
// the first shows every declared column, the second shows the declared
// defaults, and a column declared off by default is shown by one and not the
// other.
const emptyList = "-"

// columnsVersion is the first segment of the cookie, and says that what
// follows names the columns each listing HIDES.
//
// The cookie used to name the columns shown, which cannot distinguish "the
// reader turned this off" from "this column did not exist when the reader
// chose", so every column added later read as refused by every browser that
// had ever opened the picker. Recording what is hidden is how project
// visibility already works, and for the same reason.
//
// The marker covers the whole value rather than each column, because the
// inversion changes what the empty-list sentinel means as well as what a key
// means: read as the new form, an old "tasks:-" would show every column to a
// reader who asked for none. A per-column marker could not carry that.
// No listing is named "v2", so a value written in the old form can never
// begin with this segment.
const columnsVersion = "v2"

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
		{Key: "assignee", Label: "Assignee", Default: true},
		{Key: "project", Label: "Project", Default: true},
		{Key: "updated", Label: "Updated"},
		{Key: "ref", Label: "Reference", Default: true},
	},
	"users": {
		{Key: "name", Label: "Name", Default: true},
		{Key: "role", Label: "Role", Default: true},
		{Key: "state", Label: "State", Default: true},
	},
	"tokens": {
		{Key: "scopes", Label: "Scopes", Default: true},
		{Key: "expires", Label: "Expires", Default: true},
	},
	"sshkeys": {
		{Key: "label", Label: "Label", Default: true},
		{Key: "added", Label: "Added", Default: true},
		{Key: "used", Label: "Last used", Default: true},
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
//
// A column this browser has never had an opinion about is shown if it is
// declared on by default, whether the browser chose nothing at all or chose
// before the column existed.
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
	hidden := parseColumns(cookieValue(r, ColumnsCookie))
	out := make(columnPrefs, len(columnSets))
	for page, cols := range columnSets {
		off, ok := hidden[page]
		out[page] = resolveColumns(cols, off, ok)
	}
	return out
}

// resolveColumns shows every declared column except the hidden ones, falling
// back to the declared default when this browser has chosen nothing for the
// listing.
func resolveColumns(cols []Column, hidden []string, chose bool) map[string]bool {
	out := make(map[string]bool, len(cols))
	if !chose {
		for _, c := range cols {
			out[c.Key] = c.Default
		}
		return out
	}
	off := make(map[string]bool, len(hidden))
	for _, key := range hidden {
		off[key] = true
	}
	for _, c := range cols {
		out[c.Key] = !off[c.Key]
	}
	return out
}

// parseColumns reads the cookie into the columns each listing hides.
//
// The current form is "v2~page:col.col~page:-", where a listing's list names
// the columns it leaves out and "-" means it leaves out none. A listing absent
// from the value has not been chosen and keeps its declared defaults.
//
// A value without the version segment was written by a build that recorded
// the columns SHOWN, and is converted rather than read as it stands, since
// read as it stands it would mean the opposite. Only the listings and columns
// this build declares survive either way, so a stale, truncated or hand-edited
// value falls back to the default rather than failing the page or emptying a
// table.
func parseColumns(raw string) map[string][]string {
	out := map[string][]string{}
	if raw == "" || len(raw) > maxColumnsValue {
		return out
	}
	entries := strings.Split(raw, "~")
	if entries[0] != columnsVersion {
		return shownToHidden(entries)
	}
	for _, entry := range entries[1:] {
		page, list, found := strings.Cut(entry, ":")
		cols, known := columnSets[page]
		if !found || !known {
			continue
		}
		if list == emptyList {
			out[page] = []string{}
			continue
		}
		if keep := knownColumns(cols, strings.Split(list, ".")); len(keep) > 0 {
			out[page] = keep
		}
	}
	return out
}

// legacyColumns is the vocabulary each listing declared while the cookie
// recorded the columns shown. It is a frozen historical record, never appended
// to: a cookie in that form was written against these columns and can have
// held an opinion about no others, so anything this build declares beyond them
// is shown to such a browser rather than counted as refused. That is the whole
// point of the inversion, and applying it to the cookies already in readers'
// browsers is what keeps the first column added after the change visible to
// the readers who had customised most.
var legacyColumns = map[string][]string{
	"tasks":    {"status", "priority", "tags", "project", "updated", "ref"},
	"users":    {"name", "role", "state"},
	"tokens":   {"scopes", "expires"},
	"sshkeys":  {"label", "added", "used"},
	"webhooks": {"events", "active"},
	"domains":  {"verified", "cert"},
	"projects": {"name", "colour", "state", "screens", "manage"},
}

// shownToHidden converts entries written in the shown-set form, so a reader's
// existing choices survive the change to recording what is hidden instead of
// being inverted by it or thrown away.
func shownToHidden(entries []string) map[string][]string {
	out := map[string][]string{}
	for _, entry := range entries {
		page, list, found := strings.Cut(entry, ":")
		cols, known := columnSets[page]
		vocabulary, dated := legacyColumns[page]
		if !found || !known || !dated {
			continue
		}
		var shown []string
		if list != emptyList {
			if shown = knownColumns(cols, strings.Split(list, ".")); len(shown) == 0 {
				continue
			}
		}
		out[page] = hiddenColumns(withinVocabulary(cols, vocabulary), shown)
	}
	return out
}

// withinVocabulary keeps the columns a listing declared at a point in its
// history, in the order it declares them now.
func withinVocabulary(cols []Column, vocabulary []string) []Column {
	had := make(map[string]bool, len(vocabulary))
	for _, key := range vocabulary {
		had[key] = true
	}
	out := make([]Column, 0, len(vocabulary))
	for _, c := range cols {
		if had[c.Key] {
			out = append(out, c)
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

// hiddenColumns is every column the listing declares that the browser did not
// tick, which is what the cookie stores.
func hiddenColumns(cols []Column, shown []string) []string {
	on := make(map[string]bool, len(shown))
	for _, key := range shown {
		on[key] = true
	}
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if !on[c.Key] {
			out = append(out, c.Key)
		}
	}
	return out
}

// encodeColumns renders the hidden columns back into the cookie's form.
func encodeColumns(hidden map[string][]string) string {
	pages := make([]string, 0, len(hidden))
	for page := range hidden {
		pages = append(pages, page)
	}
	if len(pages) == 0 {
		return ""
	}
	sort.Strings(pages)
	parts := make([]string, 0, len(pages)+1)
	parts = append(parts, columnsVersion)
	for _, page := range pages {
		list := emptyList
		if len(hidden[page]) > 0 {
			list = strings.Join(hidden[page], ".")
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

// setColumns records which columns one listing leaves out for this browser.
// The form submits the columns to show and what is stored is everything else,
// so a column added after the choice was made is shown without being ticked.
// It is a
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
	hidden := parseColumns(cookieValue(r, ColumnsCookie))
	if checked(r, "reset") {
		delete(hidden, page)
	} else {
		hidden[page] = hiddenColumns(cols, knownColumns(cols, r.PostForm["column"]))
	}
	value := encodeColumns(hidden)
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
