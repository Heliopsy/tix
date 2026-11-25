// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/version"
)

// bodyTag is the page's opening <body> tag and nothing else, so an attribute
// found elsewhere on the page does not count.
var bodyTag = regexp.MustCompile(`<body[^>]*>`)

// TestBodyCarriesTheBuild guards what assets/deploy.js compares with
// /healthz: the release that drew the page, on <body>, and the script that
// does the comparing loaded after decide.js.
func TestBodyCarriesTheBuild(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/tasks")

	tag := bodyTag.FindString(page)
	if tag == "" {
		t.Fatal("the page has no <body> tag")
	}
	want := `data-build="` + version.Version + `"`
	if !strings.Contains(tag, want) {
		t.Errorf("<body> is %s, want it to carry %s", tag, want)
	}

	decide := strings.Index(page, `src="/assets/decide.js"`)
	deploy := strings.Index(page, `src="/assets/deploy.js"`)
	if deploy < 0 || deploy < decide {
		t.Errorf("deploy.js is not loaded after decide.js (decide at %d, deploy at %d)", decide, deploy)
	}

	resp := b.get("/assets/deploy.js")
	wantStatus(t, resp, http.StatusOK)
	if !strings.Contains(body(t, resp), "reloadBlocked") {
		t.Error("the served deploy.js is not the asset the suite covers")
	}
}
