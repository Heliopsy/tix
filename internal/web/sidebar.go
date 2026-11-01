// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
var SidebarGroups = []string{"saved", "recent", "configure", "data"}

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
	state := sidebarState{fold: map[string]bool{}}
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
	for _, g := range SidebarGroups {
		known[g] = true
	}
	for _, g := range strings.Split(values.Get("fold"), ".") {
		if known[g] {
			state.fold[g] = true
		}
	}
	return state
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
