// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"

	"github.com/heliopsy/tix/internal/changelog"
	"github.com/heliopsy/tix/internal/markdown"
	"github.com/heliopsy/tix/internal/release"
)

// upgrade is what the footer says about the running version.
type upgrade = release.State

// WithReleaseWatch shares the process's release check with the browser
// interface, so the screen and the API route draw the same answer from one
// check. Without it the handler keeps its own.
func WithReleaseWatch(w *release.Watch) Option {
	return func(h *handler) {
		if w != nil {
			h.releases = w
		}
	}
}

// releaseView is what the release screen draws beyond Upgrade, which render
// attaches to every page for the footer: the notes as HTML, and the commands
// that install the newest release.
type releaseView struct {
	// NotesHTML is the newest release's notes through the sanitising
	// Markdown renderer, empty when there are none.
	NotesHTML template.HTML
	// Methods are the upgrade commands, one per install method, empty
	// unless a newer release is on offer.
	Methods []release.Method
	// Changes is what the changelog bundled with this binary says.
	Changes changesView
}

// changesView is the "What's new" section, drawn from the bundled changelog.
type changesView struct {
	// Running is the running version's own entry, nil unless this build is
	// a tagged release the changelog has an entry for.
	Running *changeEntry
	// Earlier are the entries listed under it, or the newest entries when
	// Running is nil.
	Earlier []changeEntry
	// Untagged reports that the running version is not a plain
	// major.minor.patch, so the build is not a tagged release.
	Untagged bool
}

// changeEntry is one changelog entry ready to draw.
type changeEntry struct {
	Version string
	Date    string
	HTML    template.HTML
}

// newChangesView draws release.NewChanges through the sanitising Markdown
// renderer.
func newChangesView(running string, entries []changelog.Entry) changesView {
	c := release.NewChanges(running, entries)
	v := changesView{Untagged: c.Untagged}
	if c.Running != nil {
		drawn := drawEntry(*c.Running)
		v.Running = &drawn
	}
	for _, e := range c.Earlier {
		v.Earlier = append(v.Earlier, drawEntry(e))
	}
	return v
}

// drawEntry renders an entry the way the feed notes are rendered.
func drawEntry(e release.Entry) changeEntry {
	return changeEntry{Version: e.Version, Date: e.Date, HTML: markdown.Render(e.Notes, markdown.Options{})}
}

// newReleaseView derives the screen's own figures from the cached state.
func newReleaseView(up upgrade, entries []changelog.Entry) releaseView {
	offer := up.Offer()
	v := releaseView{Changes: newChangesView(up.Running, entries), Methods: offer.Methods}
	if offer.Notes != "" {
		v.NotesHTML = markdown.Render(offer.Notes, markdown.Options{})
	}
	return v
}
