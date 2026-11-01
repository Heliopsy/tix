// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// The calendar feed on the settings screen. Every control is a plain form
// that opts out of hx-boost, like the rest of the screen, and the address is
// answered as a screen rather than a redirect, because a redirect would carry
// the secret in a URL and so into the access log and the browser history.
const (
	RouteCalendarCreate = "/settings/calendar"
	RouteCalendarRotate = "/settings/calendar/rotate"
	RouteCalendarFilter = "/settings/calendar/filter"
	RouteCalendarRevoke = "/settings/calendar/revoke"

	calendarTemplate = "calendarfeed.html"
	calendarSection  = "calendar"
)

// calendarRoutes are the four feed controls of the settings screen.
func (h *handler) calendarRoutes() []route {
	return []route{
		post(RouteCalendarCreate, h.createCalendarFeed, "CreateCalendarFeed"),
		post(RouteCalendarRotate, h.rotateCalendarFeed, "RotateCalendarFeed"),
		post(RouteCalendarFilter, h.setCalendarFilter, "SetCalendarFeedFilter"),
		post(RouteCalendarRevoke, h.revokeCalendarFeed, "RevokeCalendarFeed"),
	}
}

// calendarView is the settings section. Feed is nil when the reader has none;
// Available is false when they may not hold one at all. Filter and Error are
// the submitted expression and why it was refused, kept on a refusal so the
// reader corrects their text rather than retyping it.
type calendarView struct {
	Available bool
	Feed      *core.CalendarFeed
	Filter    string
	Error     string
}

// Active reports whether the reader has a live feed.
func (v calendarView) Active() bool { return v.Feed != nil }

// calendarIssuedView is the one screen a feed address appears on.
type calendarIssuedView struct {
	URL     string
	Webcal  template.URL
	Rotated bool
	Filter  string
}

// calendarFor reads the reader's feed for the settings screen. Having none is
// a section with a Create button, and lacking the authority is a section that
// says so, never a failed screen.
func (h *handler) calendarFor(r *http.Request) (calendarView, error) {
	feed, err := h.svc.GetCalendarFeed(r.Context())
	switch {
	case err == nil:
		return calendarView{Available: true, Feed: feed, Filter: feed.Filter}, nil
	case core.IsKind(err, core.KindNotFound):
		return calendarView{Available: true}, nil
	case core.IsKind(err, core.KindForbidden):
		return calendarView{}, nil
	default:
		return calendarView{}, err
	}
}

// feedAddress is the absolute address of a feed secret as this request
// reached the deployment, and the same address under webcal:, which asks the
// operating system's calendar to subscribe.
func feedAddress(r *http.Request, token string) (string, string) {
	path := wire.CalendarFeedPath(token)
	return httpapi.SchemeFrom(r.Context()) + "://" + r.Host + path, "webcal://" + r.Host + path
}

// createCalendarFeed issues the reader a feed and shows its address once.
func (h *handler) createCalendarFeed(w http.ResponseWriter, r *http.Request) error {
	filter := field(r, "filter")
	feed, err := h.svc.CreateCalendarFeed(r.Context(), core.CalendarFeedInput{Filter: filter})
	if err != nil {
		return h.refuseCalendar(w, r, err, filter)
	}
	return h.renderCalendarIssued(w, r, feed, false)
}

// rotateCalendarFeed replaces the reader's feed secret and shows the new one.
func (h *handler) rotateCalendarFeed(w http.ResponseWriter, r *http.Request) error {
	feed, err := held(h.svc.RotateCalendarFeed(r.Context()))
	if err != nil {
		return err
	}
	return h.renderCalendarIssued(w, r, feed, true)
}

// setCalendarFilter changes what the reader's feed selects, keeping its address.
func (h *handler) setCalendarFilter(w http.ResponseWriter, r *http.Request) error {
	filter := field(r, "filter")
	if _, err := h.svc.SetCalendarFeedFilter(r.Context(), core.CalendarFeedInput{Filter: filter}); err != nil {
		return h.refuseCalendar(w, r, err, filter)
	}
	http.Redirect(w, r, savedNext("Calendar filter", calendarSection), http.StatusSeeOther)
	return nil
}

// revokeCalendarFeed stops the reader's feed.
func (h *handler) revokeCalendarFeed(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RevokeCalendarFeed(r.Context()); err != nil {
		return err
	}
	redirect(w, r, RouteSettings+"#"+calendarSection, "calendar feed revoked")
	return nil
}

// refuseCalendar draws the settings screen again with the refused filter in
// its field and the reason beside it. A refusal that is not about what was
// typed is the ordinary error screen.
func (h *handler) refuseCalendar(w http.ResponseWriter, r *http.Request, err error, filter string) error {
	kind := core.KindOf(err)
	if kind != core.KindInvalid && kind != core.KindNotFound {
		return err
	}
	view, readErr := h.calendarFor(r)
	if readErr != nil {
		return readErr
	}
	view.Filter, view.Error = filter, safeMessage(err, kind)
	return h.renderSettings(w, r, kind.HTTPStatus(), view)
}

// renderCalendarIssued shows a feed's address, the one time it can be shown.
func (h *handler) renderCalendarIssued(w http.ResponseWriter, r *http.Request, feed *core.IssuedCalendarFeed, rotated bool) error {
	addr, webcal := feedAddress(r, feed.Token)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// #nosec G203 -- the webcal address is this request's own host and a
	// route built from a secret in the token alphabet, so it carries no markup.
	return h.render(w, r, calendarTemplate, "Calendar feed", calendarIssuedView{
		URL: addr, Webcal: template.URL(webcal), Rotated: rotated, Filter: strings.TrimSpace(feed.Filter),
	})
}
