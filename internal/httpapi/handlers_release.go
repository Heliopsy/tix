// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/changelog"
	"github.com/heliopsy/tix/internal/release"
)

// ReleaseSource is what the release check knows; *release.Watch is one.
type ReleaseSource interface {
	State() release.State
}

// ReleaseBody is the running version, the newest published release and what
// the bundled changelog says: the browser's release screen as data.
type ReleaseBody struct {
	// Running is this binary's version, without a leading v.
	Running string `json:"running"`
	// Latest is the newest published release, empty until a check succeeds.
	Latest string `json:"latest"`
	// Status is newer, current, unknown when the versions cannot be
	// compared, failed when the feed could not be read, or pending when
	// nothing has been asked yet.
	Status string `json:"status"`
	// Newer reports whether Latest is ahead of Running.
	Newer bool `json:"newer"`
	// Checked reports whether a check has finished, whatever it answered.
	Checked bool `json:"checked"`
	// Failed reports whether the most recent check got no usable answer.
	Failed bool `json:"failed"`
	// Notes are the newest release's notes as Markdown with every link
	// taken out and the heading naming the release dropped. Empty unless
	// Newer.
	Notes string `json:"notes"`
	// Methods are the commands that upgrade each install method to Latest.
	// Empty unless Newer.
	Methods []ReleaseMethod `json:"methods"`
	// Changes is the bundled changelog around the running version.
	Changes ReleaseChanges `json:"changes"`
}

// ReleaseMethod is one way tix is installed and the command that upgrades it.
type ReleaseMethod struct {
	// ID is binary, container or go.
	ID string `json:"id"`
	// Name names the install method.
	Name string `json:"name"`
	// Help says when the method applies.
	Help string `json:"help"`
	// Command upgrades that install to Latest.
	Command string `json:"command"`
}

// ReleaseChanges is the "What's new" selection from the bundled changelog.
type ReleaseChanges struct {
	// Running is the running version's own entry, left out unless this build is
	// a tagged release the changelog has an entry for.
	Running *ReleaseEntry `json:"running,omitempty"`
	// Earlier are up to ten entries before Running, or the eleven newest
	// when Running is left out.
	Earlier []ReleaseEntry `json:"earlier"`
	// Untagged reports that the running version is not a plain
	// major.minor.patch, so the build is not a tagged release.
	Untagged bool `json:"untagged"`
}

// ReleaseEntry is one changelog entry.
type ReleaseEntry struct {
	// Version is the release, without a leading v.
	Version string `json:"version"`
	// Date is the release date, YYYY-MM-DD, or empty.
	Date string `json:"date"`
	// Notes is the entry as Markdown with every link taken out.
	Notes string `json:"notes"`
}

// handleRelease answers what the release screen shows, to any signed-in
// caller, from the same cached check the browser reads.
func (rt *Router) handleRelease(w http.ResponseWriter, r *http.Request) {
	if _, err := rt.cfg.Service.WhoAmI(r.Context()); err != nil {
		WriteError(w, err)
		return
	}
	var st release.State
	if rt.cfg.Releases != nil {
		st = rt.cfg.Releases.State()
	} else {
		st = (*release.Watch)(nil).State()
	}
	WriteJSON(w, http.StatusOK, newReleaseBody(st, changelog.Entries()))
}

// newReleaseBody shapes the state and the changelog for the wire.
func newReleaseBody(st release.State, entries []changelog.Entry) ReleaseBody {
	offer := st.Offer()
	body := ReleaseBody{Running: st.Running, Latest: st.Latest, Status: st.Status(),
		Newer: st.Newer, Checked: st.Checked, Failed: st.Failed, Notes: offer.Notes,
		Methods: make([]ReleaseMethod, 0, len(offer.Methods))}
	for _, m := range offer.Methods {
		body.Methods = append(body.Methods, ReleaseMethod(m))
	}
	c := release.NewChanges(st.Running, entries)
	body.Changes = ReleaseChanges{Untagged: c.Untagged, Earlier: make([]ReleaseEntry, 0, len(c.Earlier))}
	if c.Running != nil {
		e := ReleaseEntry(*c.Running)
		body.Changes.Running = &e
	}
	for _, e := range c.Earlier {
		body.Changes.Earlier = append(body.Changes.Earlier, ReleaseEntry(e))
	}
	return body
}
