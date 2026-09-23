// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// A first-run trap: a store with zero users has no way to log in, and the
// sign-in screen used to give no hint that this was even the problem. The
// disclosure has to name the actual commands, work with scripting off, and
// never react to what was typed into the form above it.

func TestLoginPageOffersACantSignInDisclosure(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := body(t, f.as("").get("/login"))

	if !strings.Contains(page, `<details class="panel login-help">`) {
		t.Fatalf("the login page carries no can't-sign-in disclosure:\n%s", page)
	}
	if !strings.Contains(page, "tix user create") {
		t.Errorf("the disclosure does not show how to create the first user:\n%s", page)
	}
	if !strings.Contains(page, "tix user edit") || !strings.Contains(page, "tix user ls") {
		t.Errorf("the disclosure does not show how to find and reset an existing user:\n%s", page)
	}
}

// Nothing about the disclosure may depend on what the visitor submitted, or
// it becomes an oracle for whether an email is a real account.
func TestLoginPageDisclosureDoesNotReactToTheForm(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("")

	before := body(t, b.get("/login"))
	resp := b.postRaw("/login", url.Values{
		"csrf_token": {b.csrf()}, "email": {"nobody@example.com"}, "password": {"wrong-password"},
	})
	_ = resp.Body.Close()
	after := body(t, b.get("/login"))

	if extractDetails(before, "panel login-help") != extractDetails(after, "panel login-help") {
		t.Errorf("the disclosure changed after a failed sign-in attempt, which makes it an enumeration oracle")
	}
}

// A server started with WithTargetHint has to show the same target in every
// command, or an operator runs it against the wrong store.
func TestLoginPageDisclosureCarriesTheResolvedTarget(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	handler := web.Handler(f.svc, web.WithTargetHint("/tmp/claude-1000/example.db"), web.WithLogger(nil))

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	page := recorder.Body.String()
	for _, want := range []string{
		"tix user create you@example.com --role admin --password 'change-me' --db /tmp/claude-1000/example.db",
		"tix user ls --db /tmp/claude-1000/example.db",
		"tix user edit &lt;id&gt; --password 'change-me' --db /tmp/claude-1000/example.db",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the disclosure does not carry the resolved target:\nwant substring %q\ngot:\n%s", want, page)
		}
	}
}

// Without a target hint, the commands are shown bare rather than with an
// empty or misleading flag.
func TestLoginPageDisclosureOmitsTheTargetFlagWhenThereIsNone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := body(t, f.as("").get("/login"))

	if strings.Contains(page, "--db \n") || strings.Contains(page, "--db <") || strings.Contains(page, `--db ""`) {
		t.Errorf("the disclosure shows a --db flag with no value:\n%s", page)
	}
	if strings.Contains(page, "tix user create you@example.com --role admin --password 'change-me' --db") {
		t.Errorf("the disclosure appends a --db flag despite no target hint being configured:\n%s", page)
	}
}

func extractDetails(page, class string) string {
	marker := `<details class="` + class + `">`
	start := strings.Index(page, marker)
	if start < 0 {
		return ""
	}
	rest := page[start:]
	end := strings.Index(rest, "</details>")
	if end < 0 {
		return rest
	}
	return rest[:end]
}
