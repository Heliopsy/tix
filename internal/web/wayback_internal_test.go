// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"
)

func TestIsScreenTellsAScreenFromAnAction(t *testing.T) {
	t.Parallel()
	h := Handler(nil).(*handler)
	for path, want := range map[string]bool{
		RouteRoot:              true,
		RouteSync:              true,
		"/projects/ops/delete": true,
		RouteSyncRun:           false,
		RouteSyncDelete:        false,
		"/nowhere":             false,
	} {
		if got := h.isScreen(path); got != want {
			t.Errorf("isScreen(%q) = %v, want %v", path, got, want)
		}
	}
}

var wildcard = regexp.MustCompile(`\{[^}]+\}`)

// Every action the browser posts to, when it fails, offers a way back that
// draws a screen. An action such as /sync/run has no screen of its own, and
// the error page used to send the reader straight back to it: a 404 with
// nothing on it but a link to the task list.
func TestAFailedActionLeadsBackToAScreen(t *testing.T) {
	t.Parallel()
	h := Handler(nil).(*handler)
	for _, rt := range h.routes() {
		if rt.Method != http.MethodPost {
			continue
		}
		path := wildcard.ReplaceAllString(rt.Pattern, "x")
		for _, referer := range []string{"", "http://tix/sync?flash=saved", "http://tix/sync/run"} {
			r := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: path}, Host: "tix",
				Header: http.Header{}}
			if referer != "" {
				r.Header.Set("Referer", referer)
			}
			back := h.wayBack(r)
			u, err := url.Parse(back)
			if err != nil || !h.isScreen(u.Path) || u.Path == RouteRoot {
				t.Errorf("a failed POST %s from %q leads back to %q, which draws no screen", path, referer, back)
				continue
			}
			if h.isScreen(path) && back != path {
				t.Errorf("a failed POST %s leads back to %q, not to its own screen", path, back)
			}
			if !h.isScreen(path) && referer == "http://tix/sync?flash=saved" && back != RouteSync {
				t.Errorf("a failed POST %s leads back to %q, not to the screen it was posted from", path, back)
			}
		}
	}
}
