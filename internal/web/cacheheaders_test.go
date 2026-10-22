// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"strings"
	"testing"
)

// TestEveryScreenForbidsStorage pins the fix for screens that carried no
// Cache-Control at all.
//
// A response holding one tenant's tasks named neither a freshness lifetime nor
// a validator, which let a browser cache it heuristically and hand back a task
// list that was already wrong, and let any shared cache, or the next person at
// the same machine after a logout, read an authenticated board out of storage.
//
// The screens are listed rather than discovered so that a new one added without
// a header is a failure here instead of a gap nobody looks for. Assets are
// deliberately absent: they are the opposite case and are covered by
// TestEveryAssetCarriesACacheValidator.
func TestEveryScreenForbidsStorage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	// The root is absent on purpose: it answers 303 to the task screen or to
	// the login form, and a 303 is already uncacheable unless a response says
	// otherwise. It renders nothing, so renderStatus never sees it.
	paths := []string{
		"/tasks",
		"/projects",
		"/workflows",
		"/activity",
		"/stats",
		"/settings",
		"/admin/tenant",
		"/admin/users",
		"/admin/tokens",
		"/admin/domains",
	}
	for _, path := range paths {
		resp := b.get(path)
		if resp.StatusCode >= 400 {
			t.Errorf("%s: status %d, so this test is not exercising a real screen",
				path, resp.StatusCode)
			continue
		}
		if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Errorf("%s carries Cache-Control %q, which does not forbid storing an authenticated page",
				path, cc)
		}
	}
}

// TestARefusedScreenAlsoForbidsStorage covers the non-200 renders.
//
// renderStatus is the one writer for every screen, including the error screen
// and a form redrawn after a refusal, and a refused form is exactly the
// response most likely to carry what the user just typed.
func TestARefusedScreenAlsoForbidsStorage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.get("/tasks/nosuchtask")
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("/tasks/nosuchtask returned 200, so this is not the refusal path")
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("a refused screen carries Cache-Control %q, want no-store", cc)
	}
}
