// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// The saved filter routes. Opening one is a GET that lands on the task
// screen; the rest are the forms of the filter bar's Save and of the three
// dots beside each filter in the sidebar.
const (
	RouteFilters      = "/filters"
	RouteFilterOpen   = "/filters/{filter}"
	RouteFilterEdit   = "/filters/{filter}/edit"
	RouteFilterMove   = "/filters/{filter}/move"
	RouteFilterDelete = "/filters/{filter}/delete"
)

// savedNav is the sidebar's list of saved filters: the reader's own, then
// the shared ones, with what each one's menu needs to post and return.
type savedNav struct {
	Items []savedItem
	CSRF  string
	Here  string
	// MyWork marks the My work entry as the page being shown.
	MyWork bool
}

// savedItem is one filter in the sidebar.
type savedItem struct {
	core.SavedFilter
	Href    string
	Current bool
	// Up and Down report whether the filter has one of the reader's own
	// filters above or below it, which is the only move offered.
	Up   bool
	Down bool
}

// DOMID is a fragment safe for an element id.
func (s savedItem) DOMID() string { return "sf-" + slug(s.ID) }

// filterOpenHref is where a saved filter is opened from.
func filterOpenHref(id string) string {
	return strings.Replace(RouteFilterOpen, "{filter}", url.PathEscape(id), 1)
}

// filterTasksHref is the task screen answering a saved filter's expression
// with its sort.
func filterTasksHref(f core.SavedFilter) string {
	q := url.Values{}
	if f.Query != "" {
		q.Set("q", f.Query)
	}
	if f.Sort != "" {
		q.Set("sort", f.Sort)
	}
	if len(q) == 0 {
		return RouteTasks
	}
	return RouteTasks + "?" + q.Encode()
}

// buildSavedNav lists the reader's saved filters for the sidebar. A reader
// the service will not answer gets an empty list rather than a broken page.
func (h *handler) buildSavedNav(r *http.Request) savedNav {
	nav := savedNav{CSRF: csrfFrom(r), Here: here(r), MyWork: r.URL.Path == RouteMine}
	if _, ok := core.ActorFrom(r.Context()); !ok || h.svc == nil {
		return nav
	}
	filters, err := h.svc.ListSavedFilters(r.Context())
	if err != nil {
		return nav
	}
	current := r.URL.Path == RouteTasks
	q, sort := strings.TrimSpace(r.URL.Query().Get("q")), r.URL.Query().Get("sort")
	if sort == core.SortUrgency {
		sort = ""
	}
	for i, f := range filters {
		item := savedItem{SavedFilter: f, Href: filterOpenHref(f.ID)}
		fsort := f.Sort
		if fsort == core.SortUrgency {
			fsort = ""
		}
		item.Current = current && q == f.Query && sort == fsort && r.URL.Query().Get(DueParam) == ""
		item.Up = f.Mine && i > 0 && filters[i-1].Mine
		item.Down = f.Mine && i+1 < len(filters) && filters[i+1].Mine
		nav.Items = append(nav.Items, item)
	}
	return nav
}

// filterRoutes are the saved filter forms and the route that opens one.
func (h *handler) filterRoutes() []route {
	return []route{
		post(RouteFilters, h.createFilter, "CreateSavedFilter"),
		get(RouteFilterOpen, "tasks.html", h.openFilter, "GetSavedFilter"),
		post(RouteFilterEdit, h.editFilter, "UpdateSavedFilter"),
		post(RouteFilterMove, h.moveFilter, "OrderSavedFilters", "ListSavedFilters"),
		post(RouteFilterDelete, h.deleteFilter, "DeleteSavedFilter"),
	}
}

// createFilter saves what the filter bar is showing under a name, and opens
// it. The deadline control is folded into the expression the way the task
// screen folds it, so the saved filter answers the question on screen.
func (h *handler) createFilter(w http.ResponseWriter, r *http.Request) error {
	expr := field(r, "q")
	if due := field(r, DueParam); due != "" {
		expr = strings.TrimSpace("due:" + due + " " + expr)
	}
	sort := field(r, "sort")
	if sort == core.SortUrgency {
		sort = ""
	}
	f, err := h.svc.CreateSavedFilter(r.Context(), core.SavedFilterInput{
		Name: field(r, "name"), Query: expr, View: field(r, "view"), Sort: sort,
		Shared: field(r, "shared") == "1",
	})
	if err != nil {
		return err
	}
	h.applyFilterView(w, r, *f)
	redirect(w, r, filterTasksHref(*f), "filter saved")
	return nil
}

// openFilter lands on the task screen a saved filter describes. Its view is
// this browser's view preference, the one the View switch sets, so it is
// written before the redirect; a filter that names no view keeps the one the
// reader has.
func (h *handler) openFilter(w http.ResponseWriter, r *http.Request) error {
	f, err := h.svc.GetSavedFilter(r.Context(), r.PathValue("filter"))
	if err != nil {
		return err
	}
	h.applyFilterView(w, r, *f)
	// #nosec G710 -- the target is built from the task route and an encoded
	// query, never from input that could name another origin.
	http.Redirect(w, r, filterTasksHref(*f), http.StatusSeeOther)
	return nil
}

// applyFilterView sets the view switch to the view a filter names.
func (h *handler) applyFilterView(w http.ResponseWriter, r *http.Request, f core.SavedFilter) {
	if f.View == "" {
		return
	}
	value, age := f.View, cookieYear
	if value == TaskViewList {
		value, age = "", -1
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: TaskViewCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
}

// editFilter renames, shares or unshares a filter, changing only the fields
// the form posted.
func (h *handler) editFilter(w http.ResponseWriter, r *http.Request) error {
	next := safeNext(field(r, "next"))
	var in core.SavedFilterUpdate
	if _, ok := r.PostForm["name"]; ok {
		name := field(r, "name")
		in.Name = &name
	}
	if _, ok := r.PostForm["shared"]; ok {
		shared := field(r, "shared") == "1"
		in.Shared = &shared
	}
	if _, err := h.svc.UpdateSavedFilter(r.Context(), r.PathValue("filter"), in); err != nil {
		return err
	}
	redirect(w, r, next, "filter saved")
	return nil
}

// moveFilter swaps one of the reader's own filters with its neighbour and
// stores the whole order.
func (h *handler) moveFilter(w http.ResponseWriter, r *http.Request) error {
	filters, err := h.svc.ListSavedFilters(r.Context())
	if err != nil {
		return err
	}
	var own []string
	for _, f := range filters {
		if f.Mine {
			own = append(own, f.ID)
		}
	}
	dir := map[string]string{"up": "left", "down": "right"}[field(r, "dir")]
	if dir == "" {
		return core.Invalid("move %q: use up or down", field(r, "dir"))
	}
	order, err := movedOrder(own, r.PathValue("filter"), dir)
	if err != nil {
		return err
	}
	if _, err := h.svc.OrderSavedFilters(r.Context(), core.SavedFilterOrderInput{Filters: order}); err != nil {
		return err
	}
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// deleteFilter removes a filter. The tasks it selected are untouched.
func (h *handler) deleteFilter(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteSavedFilter(r.Context(), r.PathValue("filter")); err != nil {
		return err
	}
	redirect(w, r, safeNext(field(r, "next")), "filter removed")
	return nil
}

// RecentOwner names whose recently viewed list this browser shows: a digest
// of the tenant and the actor, so two readers sharing a browser keep apart
// lists and the page carries no identifier it would not otherwise show.
func (v view) RecentOwner() string {
	if v.Actor == nil {
		return ""
	}
	sum := sha256.Sum256([]byte(v.Actor.TenantID + "\x00" + v.Actor.ID))
	return hex.EncodeToString(sum[:8])
}
