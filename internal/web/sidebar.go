// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// SidebarCookie holds how this browser arranged the sidebar: its width, its
// rail and which of its groups are folded. assets/sidebar.js writes it and the
// server reads it, so the first paint of every page, and of every boosted
// navigation, draws the sidebar the reader left rather than a default the
// script corrects a frame later.
const SidebarCookie = "tix_sidebar"

// The width bounds assets/sidebar.js enforces, repeated so a cookie written by
// hand cannot draw a sidebar the handle could never reach.
const (
	sidebarMinWidth = 180
	sidebarMaxWidth = 480
)

// SidebarGroups are the sidebar's foldable groups, in the order they are drawn.
var SidebarGroups = []string{"saved", "recent", "admin"}

// sidebarDefaultFold are the groups drawn folded until a reader opens them: the
// admin screens, visited far less than tasks. A cookie without a "fold" key
// means this default; an empty "fold=" means every group open. Kept in step
// with DEFAULT_FOLD in assets/sidebar.js.
var sidebarDefaultFold = []string{"admin"}

// sidebarLegacyAdmin are the two groups an earlier build drew where Admin is
// now. A cookie that folded both folds Admin; one that left either open
// opens it, so nobody who had opened Configure or Data finds it shut.
var sidebarLegacyAdmin = []string{"configure", "data"}

func defaultFold() map[string]bool {
	fold := map[string]bool{}
	for _, g := range sidebarDefaultFold {
		fold[g] = true
	}
	return fold
}

// sidebarState is the sidebar as this browser left it.
type sidebarState struct {
	// Width is the dragged width in pixels, zero for the stylesheet's own.
	Width int
	// Rail is whether the desktop column is collapsed to icons.
	Rail bool
	fold map[string]bool
}

// Folded reports whether a group is drawn folded.
func (s sidebarState) Folded(group string) bool { return s.fold[group] }

// sidebarOf reads the sidebar cookie. Anything it does not understand is the
// default, so a damaged cookie costs an arrangement and never a page.
func sidebarOf(r *http.Request) sidebarState {
	state := sidebarState{fold: defaultFold()}
	c, err := r.Cookie(SidebarCookie)
	if err != nil {
		return state
	}
	values, err := url.ParseQuery(c.Value)
	if err != nil {
		return state
	}
	if w, err := strconv.Atoi(values.Get("w")); err == nil && w > 0 {
		state.Width = min(sidebarMaxWidth, max(sidebarMinWidth, w))
	}
	state.Rail = values.Get("rail") == "1"
	known := map[string]bool{}
	for _, g := range append(append([]string{}, SidebarGroups...), sidebarLegacyAdmin...) {
		known[g] = true
	}
	if _, set := values["fold"]; set {
		raw := values.Get("fold")
		fold := map[string]bool{}
		for _, g := range strings.Split(raw, ".") {
			if known[g] {
				fold[g] = true
			}
		}
		if fold[sidebarLegacyAdmin[0]] && fold[sidebarLegacyAdmin[1]] {
			fold["admin"] = true
		}
		for _, g := range sidebarLegacyAdmin {
			delete(fold, g)
		}
		// An empty list is a reader who opened every group, and so is a list
		// naming only one of the legacy admin groups. A list naming nothing
		// this version or the last one drew is damage, and gets the default.
		if raw == "" || len(fold) > 0 || namesAny(raw, sidebarLegacyAdmin) {
			state.fold = fold
		}
	}
	return state
}

// namesAny reports whether a dotted fold list names any of groups.
func namesAny(raw string, groups []string) bool {
	return slices.ContainsFunc(strings.Split(raw, "."), func(g string) bool { return slices.Contains(groups, g) })
}

// navGroupHead is what one foldable group's heading is drawn from.
type navGroupHead struct {
	Name   string
	Label  string
	Icon   string
	Folded bool
	// Sub marks a group under Tasks, drawn a step quieter.
	Sub bool
}

// newNavGroup builds a group's heading from the sidebar state.
func newNavGroup(side sidebarState, name, label, icon string) navGroupHead {
	return navGroupHead{
		Name: name, Label: label, Icon: icon, Folded: side.Folded(name),
		Sub: name == "saved" || name == "recent",
	}
}

// sidebarPinLimit is the most starred projects the sidebar draws. A list
// longer than this ends with Show all, which opens the projects screen
// filtered to the starred ones.
const sidebarPinLimit = 12

// pinnedProject is one project the reader starred, as the sidebar draws it.
type pinnedProject struct {
	Key     string
	Label   string
	Mark    iconMark
	Class   string
	Style   template.CSS
	Current bool
}

// pinnedProjects are the projects the reader starred, which is the star a
// project's lane carries too: one per-reader mark, so a project starred here
// leads the lanes and a lane starred there is listed here. An archived
// project is left out, and more reports that the reader starred more than the
// sidebar draws.
func (h *handler) pinnedProjects(r *http.Request) (out []pinnedProject, more bool) {
	if _, ok := core.ActorFrom(r.Context()); !ok {
		return nil, false
	}
	starred, err := h.svc.ListStarredProjects(r.Context())
	if err != nil {
		return nil, false
	}
	style := projectLabelOf(r)
	for _, p := range starred {
		if p.Archived() {
			continue
		}
		if len(out) == sidebarPinLimit {
			return out, true
		}
		plain := p
		plain.Icon = ""
		out = append(out, pinnedProject{
			Key: p.Key, Label: projectLabel(style, plain), Mark: markOf(p, true),
			Class: projClass(p.Color), Style: projStyle(p.Color),
			Current: strings.EqualFold(r.URL.Path, strings.Replace(RouteProject, "{key}", p.Key, 1)),
		})
	}
	return out, false
}

// overdueCap is the largest count My work's badge spells out; past it the
// badge reads 99+.
const overdueCap = 99

// overdueCount is how many of the reader's open assigned tasks are past their
// deadline, which is what My work's badge shows: the part of the list that
// wants attention, rather than its length. It asks for one small page and
// counts it, so it costs what the overdue filter costs and never more than
// overdueCap+1 rows. A reader the service will not tell gets no badge.
func (h *handler) overdueCount(r *http.Request) int {
	if _, ok := core.ActorFrom(r.Context()); !ok {
		return 0
	}
	filter, err := ParseFilter(myWorkQuery + " due:overdue")
	if err != nil {
		return 0
	}
	open, err := h.unfinishedStatuses(r.Context())
	if err != nil || len(open) == 0 {
		return 0
	}
	filter.Statuses = open
	filter.Page.Limit = overdueCap + 1
	filter.OmitBody = true
	page, err := h.svc.ListTasks(r.Context(), filter)
	if err != nil {
		return 0
	}
	return len(page.Tasks)
}
