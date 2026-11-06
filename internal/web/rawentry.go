// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"bytes"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// rawPrefix starts the id of the dialog showing one history row's raw audit
// entries, which ends in the sequence number of the row's latest entry.
const rawPrefix = "raw-"

// rawOpener is the Raw control on a history row: a link to the screen with
// that row's dialog drawn open, which with scripting fetches the dialog into
// the slot after the list and opens it as a modal one.
type rawOpener struct {
	ID    string
	Href  string
	Steps int
	// Dialog asks for the screen as the task dialog draws it, so the
	// fetched dialog carries the ids of the dialog it opens from.
	Dialog bool
}

// newRawOpener is the Raw control of one history row on the screen at here.
func newRawOpener(here, prefix string, g historyGroup, inDialog bool) rawOpener {
	id := prefix + rawPrefix + strconv.FormatInt(g.Latest().Entry.Seq, 10)
	return rawOpener{ID: id, Href: dialogHref(here, id), Steps: len(g.Hops), Dialog: inDialog}
}

// rawEntry is the dialog that shows one history row's audit entries as they
// were recorded: who, what, when and through which surface, then each
// entry's before and after snapshots, pretty-printed, with the fields that
// changed set side by side.
type rawEntry struct {
	ID          string
	Open        bool
	Back        string
	Verb        string
	Subject     string
	SubjectHref string
	At          time.Time
	Hops        []rawHop
	Steps       int
	// All is every entry of the row as one JSON document, what Copy all
	// as JSON copies.
	All string
}

// Action is what the row's latest entry recorded.
func (e *rawEntry) Action() string { return e.Hops[len(e.Hops)-1].Entry.Action }

// rawHop is one audit entry of a row as the dialog draws it.
type rawHop struct {
	Entry     core.AuditEntry
	ID        string
	Step      int
	Actor     string
	ActorHref string
	Before    string
	After     string
	Diff      []rawDiff
	Channel   viaView
}

// rawDiff is one field an entry changed: added, removed or changed, with its
// value on each side as compact JSON.
type rawDiff struct {
	Key    string
	Kind   string
	Before string
	After  string
}

// rawGroupID reports the sequence number a raw dialog id names, if it names
// one of a screen whose ids start with prefix.
func rawGroupID(open, prefix string) (int64, bool) {
	rest, ok := strings.CutPrefix(open, prefix+rawPrefix)
	if !ok {
		return 0, false
	}
	seq, err := strconv.ParseInt(rest, 10, 64)
	return seq, err == nil
}

// findRawGroup is the history row whose latest entry has this sequence
// number.
func findRawGroup(groups []historyGroup, seq int64) (historyGroup, bool) {
	for _, g := range groups {
		if len(g.Hops) > 0 && g.Latest().Entry.Seq == seq {
			return g, true
		}
	}
	return historyGroup{}, false
}

// newRawEntry builds the dialog for one history row, drawn open on the
// screen at here.
func newRawEntry(id, here string, g historyGroup, names actorNames, subject, subjectHref string) *rawEntry {
	latest := g.Latest().Entry
	out := &rawEntry{
		ID: id, Open: true, Back: withoutDialog(here), Verb: g.Verb,
		Subject: subject, SubjectHref: subjectHref, At: latest.OccurredAt,
		Hops: make([]rawHop, len(g.Hops)), Steps: len(g.Hops),
	}
	entries := make([]core.AuditEntry, len(g.Hops))
	for i, hop := range g.Hops {
		e := hop.Entry
		entries[i] = e
		out.Hops[i] = rawHop{
			Entry: e, ID: id + "-" + strconv.Itoa(i+1), Step: i + 1,
			Actor: names.Label(e.ActorID), ActorHref: actorFeedHref(e.ActorID),
			Before: prettyJSON(e.Before), After: prettyJSON(e.After),
			Diff: diffSnapshots(e.Before, e.After), Channel: hop.Channel(),
		}
	}
	var all any = entries
	if len(entries) == 1 {
		all = entries[0]
	}
	if raw, err := json.Marshal(all); err == nil {
		out.All = prettyJSON(raw)
	}
	return out
}

// actorFeedHref is the activity feed narrowed to one actor, or nothing for a
// change no actor made.
func actorFeedHref(id string) string {
	if id == "" {
		return ""
	}
	return RouteActivity + "?" + url.Values{"actor": {id}}.Encode()
}

// prettyJSON is a recorded snapshot indented for reading. Markup characters
// stay as written, since the template escapes them where they are drawn,
// and a snapshot that is not JSON is shown as it was stored.
func prettyJSON(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return string(raw)
	}
	return prettyValue(v)
}

// prettyValue is any value as indented JSON with markup characters unescaped.
func prettyValue(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return ""
	}
	return strings.TrimRight(buf.String(), "\n")
}

// compactValue is one field's value as a single line of JSON.
func compactValue(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return ""
	}
	return strings.TrimRight(buf.String(), "\n")
}

// diffSnapshots lists the top-level fields that differ between two object
// snapshots, by name. It is empty unless both sides are objects, since a
// creation or a removal has nothing to set beside its one snapshot.
func diffSnapshots(before, after json.RawMessage) []rawDiff {
	left, okLeft := decodeObject(before)
	right, okRight := decodeObject(after)
	if !okLeft || !okRight {
		return nil
	}
	keys := make([]string, 0, len(left)+len(right))
	for k := range left {
		keys = append(keys, k)
	}
	for k := range right {
		if _, ok := left[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var out []rawDiff
	for _, k := range keys {
		l, inLeft := left[k]
		r, inRight := right[k]
		switch {
		case !inLeft:
			out = append(out, rawDiff{Key: k, Kind: "added", After: compactValue(r)})
		case !inRight:
			out = append(out, rawDiff{Key: k, Kind: "removed", Before: compactValue(l)})
		case !equalJSON(l, r):
			out = append(out, rawDiff{Key: k, Kind: "changed", Before: compactValue(l), After: compactValue(r)})
		}
	}
	return out
}

// decodeObject decodes a snapshot that is a JSON object.
func decodeObject(raw json.RawMessage) (map[string]any, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil || out == nil {
		return nil, false
	}
	return out, true
}
