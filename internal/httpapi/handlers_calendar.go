// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/ical"
	"github.com/heliopsy/tix/internal/wire"
)

// TaskPagePrefix starts the browser address of a task's page, which every feed
// event links to.
const TaskPagePrefix = "/tasks/"

// calendarCacheControl lets a client or a shared cache in front of the
// deployment reuse a feed for five minutes; private keeps it out of any cache
// serving more than one reader, because the address is a secret.
const calendarCacheControl = "private, max-age=300"

// registerCalendarRoutes binds the caller's feed management and the feed
// itself, which needs no credential.
func (rt *Router) registerCalendarRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteCalendar, rt.handleGetCalendarFeed)
	rt.mux.HandleFunc("POST "+wire.RouteCalendar, rt.handleCreateCalendarFeed)
	rt.mux.HandleFunc("PATCH "+wire.RouteCalendar, rt.handleSetCalendarFeedFilter)
	rt.mux.HandleFunc("DELETE "+wire.RouteCalendar, rt.handleRevokeCalendarFeed)
	rt.mux.HandleFunc("POST "+wire.RouteCalendarRotate, rt.handleRotateCalendarFeed)
	rt.mux.HandleFunc("GET "+wire.RouteCalendarFeed, rt.handleReadCalendarFeed)
}

// isCalendarFeedPath reports whether a request reads a feed, which is the one
// API route a calendar app reaches without a credential.
func isCalendarFeedPath(path string) bool {
	rest, ok := strings.CutPrefix(path, wire.CalendarFeedPrefix)
	return ok && rest != "" && !strings.Contains(rest, "/")
}

// origin is the scheme and host the request reached the deployment at.
func origin(r *http.Request) string {
	return SchemeFrom(r.Context()) + "://" + r.Host
}

// addressed fills in the feed's path and its absolute address.
func addressed(r *http.Request, f *core.IssuedCalendarFeed) *core.IssuedCalendarFeed {
	f.Path = wire.CalendarFeedPath(f.Token)
	f.URL = origin(r) + f.Path
	return f
}

// handleGetCalendarFeed returns the caller's feed, without its secret.
func (rt *Router) handleGetCalendarFeed(w http.ResponseWriter, r *http.Request) {
	feed, err := rt.cfg.Service.GetCalendarFeed(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, feed)
}

// handleCreateCalendarFeed issues the caller a feed and answers its address once.
func (rt *Router) handleCreateCalendarFeed(w http.ResponseWriter, r *http.Request) {
	var in core.CalendarFeedInput
	if !readOptionalJSON(w, r, &in) {
		return
	}
	feed, err := rt.cfg.Service.CreateCalendarFeed(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, addressed(r, feed))
}

// handleRotateCalendarFeed replaces the caller's feed secret.
func (rt *Router) handleRotateCalendarFeed(w http.ResponseWriter, r *http.Request) {
	feed, err := rt.cfg.Service.RotateCalendarFeed(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, addressed(r, feed))
}

// handleSetCalendarFeedFilter changes the caller's feed filter.
func (rt *Router) handleSetCalendarFeedFilter(w http.ResponseWriter, r *http.Request) {
	var in core.CalendarFeedInput
	if !readJSON(w, r, &in) {
		return
	}
	feed, err := rt.cfg.Service.SetCalendarFeedFilter(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, feed)
}

// handleRevokeCalendarFeed stops the caller's feed.
func (rt *Router) handleRevokeCalendarFeed(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RevokeCalendarFeed(r.Context()); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleReadCalendarFeed answers a feed as iCalendar, with a strong ETag over
// the body so a client polling an unchanged feed is told so in a few bytes.
func (rt *Router) handleReadCalendarFeed(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutSuffix(r.PathValue("file"), wire.CalendarFeedSuffix)
	if !ok {
		WriteError(w, core.NotFound("calendar feed not found"))
		return
	}
	entries, err := rt.cfg.Service.ReadCalendarFeed(r.Context(), token)
	if err != nil {
		WriteError(w, err)
		return
	}
	base := origin(r)
	body := ical.Render(*entries, ical.Options{
		Name:    "tix: " + entries.OwnerHandle,
		TaskURL: func(ref string) string { return base + TaskPagePrefix + url.PathEscape(ref) },
	})
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", calendarCacheControl)
	h.Set("Referrer-Policy", "no-referrer")
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set(wire.HeaderContentType, ical.ContentType)
	h.Set("Content-Disposition", `inline; filename="tix.ics"`)
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body) // #nosec G705 -- escaped iCalendar text, served as text/calendar with nosniff, never as a page
	}
}

// etagMatches reports whether an If-None-Match header names the entity tag,
// comparing weakly as RFC 9110 asks of If-None-Match.
func etagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}
