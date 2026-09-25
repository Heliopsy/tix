// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// CursorParam and TrailParam are the two query parameters the pager owns.
//
// Keyset pagination only goes forward: a cursor names where the next page
// starts and carries nothing about where the current one did. The trail is
// how the browser gets a Previous control without the store growing a
// backwards keyset: the page remembers, in its own URL, the cursors it came
// through, and Previous pops the last one. Nothing about it reaches core, the
// store or the API, and a trail that is wrong or missing costs the reader a
// return to the first page rather than a wrong page of rows.
const (
	CursorParam = "cursor"
	TrailParam  = "trail"
)

// trailSeparator joins the cursors of a trail. A cursor is base64 in the
// URL-safe alphabet (core.Cursor.Encode), so a full stop never occurs inside
// one and needs no escaping of its own.
const trailSeparator = "."

// maxTrailDepth and maxTrailLength bound the trail, which arrives from the
// address bar and is therefore whatever anybody cares to type. Without a
// bound a link could carry an arbitrarily long parameter that every page of
// that listing then copies forward and lengthens. Forty pages back is further
// than anybody walks a keyset listing by hand, and the byte bound holds even
// if some future cursor encodes much more than today's.
const (
	maxTrailDepth  = 40
	maxTrailLength = 4096
)

// pager is the position control every keyset listing renders. One type and
// one partial, so the six listings cannot disagree about what the control is
// called, where it sits or what it says.
type pager struct {
	// Base is the listing's own path, and Params the filter and sort it is
	// showing. Neither carries a cursor: the pager writes those itself, which
	// is what keeps a paged URL shareable and a filter form's plain GET, which
	// submits neither, a reset back to the first page.
	Base   string
	Params url.Values

	// Cursor is where this page starts, empty on the first page. Next is
	// where the following page starts, empty on the last one.
	Cursor string
	Next   string

	// Trail is the cursors of the pages walked to reach this one, oldest
	// first, excluding the first page (which has no cursor) and this one.
	Trail []string

	// Count and Unit are the position indicator's second half: how many rows
	// this page actually carries, and what they are called.
	Count int
	Unit  string
}

// newPager reads the position out of a request and pairs it with the cursor
// the listing's own query returned. keep names the query parameters that
// describe the listing rather than the position, and they are the only ones
// carried into the pager's own links.
func newPager(r *http.Request, base, next string, count int, unit string, keep ...string) pager {
	query := r.URL.Query()
	params := url.Values{}
	for _, name := range keep {
		if v := query.Get(name); v != "" {
			params.Set(name, v)
		}
	}
	return pager{
		Base: base, Params: params, Count: count, Unit: unit,
		Cursor: query.Get(CursorParam), Next: next, Trail: trailFrom(r),
	}
}

// trailFrom reads the cursor trail a request carries, and returns nothing at
// all for a trail this build will not walk.
//
// The whole value is rejected rather than repaired, for the same reason
// safeNext returns a known-good destination rather than editing a suspect
// one: a half-kept trail would send Previous to a page the reader was never
// on, which is worse than sending them to the first page. Every entry has to
// decode as a cursor this build issued, so a trail of invented tokens cannot
// reach a query.
func trailFrom(r *http.Request) []string {
	raw := r.URL.Query().Get(TrailParam)
	if raw == "" || len(raw) > maxTrailLength {
		return nil
	}
	parts := strings.Split(raw, trailSeparator)
	if len(parts) > maxTrailDepth {
		return nil
	}
	for _, token := range parts {
		// DecodeCursor reads an empty token as the first page rather than as
		// an error, so an empty entry is refused here: it would otherwise
		// make Previous jump over every page between here and the start.
		if strings.TrimSpace(token) == "" {
			return nil
		}
		if _, err := core.DecodeCursor(token); err != nil {
			return nil
		}
	}
	return parts
}

// Show reports whether the control has anything to offer. A listing that fits
// on one page renders no pager rather than a pair of dead controls.
func (p pager) Show() bool { return p.HasPrev() || p.HasNext() }

// HasPrev reports whether there is a page before this one. The first page has
// no cursor, which is exactly what makes it the first page.
func (p pager) HasPrev() bool { return p.Cursor != "" }

// HasNext reports whether the listing returned a cursor to carry on from.
func (p pager) HasNext() bool { return p.Next != "" }

// Page is which page of the walk this is, counting from one. It is derived
// from the trail, so it is right for any URL this control produced; a link
// edited by hand to drop the trail loses the count along with the history it
// was recording.
func (p pager) Page() int {
	if p.Cursor == "" {
		return 1
	}
	return len(p.Trail) + 2
}

// PrevHref is the page before this one: the last cursor of the trail, with
// that cursor popped off, or the cursorless first page once the trail is
// empty.
func (p pager) PrevHref() string {
	if len(p.Trail) == 0 {
		return p.href("", nil)
	}
	last := len(p.Trail) - 1
	return p.href(p.Trail[last], p.Trail[:last])
}

// NextHref is the following page, with this page's cursor pushed onto the
// trail so Previous can come back to it.
func (p pager) NextHref() string {
	trail := p.Trail
	if p.Cursor != "" {
		trail = append(append(make([]string, 0, len(p.Trail)+1), p.Trail...), p.Cursor)
		if len(trail) > maxTrailDepth {
			// Past the bound the walk keeps working and only its memory is
			// shortened: the oldest entries go, since Previous is walked from
			// the newest end.
			trail = trail[len(trail)-maxTrailDepth:]
		}
	}
	return p.href(p.Next, trail)
}

// href renders one position as a link to this listing.
func (p pager) href(cursor string, trail []string) string {
	values := url.Values{}
	for name, vs := range p.Params {
		for _, v := range vs {
			values.Add(name, v)
		}
	}
	if cursor != "" {
		values.Set(CursorParam, cursor)
	}
	if len(trail) > 0 {
		values.Set(TrailParam, strings.Join(trail, trailSeparator))
	}
	if len(values) == 0 {
		return p.Base
	}
	return p.Base + "?" + values.Encode()
}
