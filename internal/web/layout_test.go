// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func TestEveryScreenDeclaresAPhoneViewport(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "viewport check")

	paths := []string{"/projects", "/projects/infra", "/projects/infra/fields", "/workflows",
		"/workflows/default", "/tasks", "/tasks/" + ref, "/activity", "/admin/tenant",
		"/admin/domains", "/admin/users", "/admin/tokens", "/admin/webhooks",
		"/transfer", "/sync"}
	for _, path := range paths {
		page := b.page(path)
		if !strings.Contains(page, `<meta name="viewport" content="width=device-width, initial-scale=1">`) {
			t.Errorf("screen %s declares no phone viewport", path)
		}
	}
}

func TestStylesheetEmitsNoHorizontalScrollLayout(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").get("/assets/app.css")
	wantStatus(t, resp, http.StatusOK)
	sheet := body(t, resp)

	for _, want := range []string{"overflow-x: hidden", "--gutter: 16px",
		"padding: 0 var(--gutter)", "@media (max-width: 639px)", "flex-wrap: wrap",
		"max-width: 100%"} {
		if !strings.Contains(sheet, want) {
			t.Errorf("the stylesheet is missing %q", want)
		}
	}
	if strings.Contains(sheet, "white-space: nowrap") {
		t.Errorf("the stylesheet forces text to stay on one line")
	}
}

// fixedWidth matches a declared width in pixels, which would break a narrow
// viewport.
var fixedWidth = regexp.MustCompile(`(?:^|[^-])width:\s*\d{3,}px`)

func TestNoScreenDeclaresAFixedPixelWidth(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "narrow viewport")

	for _, path := range []string{"/projects", "/projects/infra", "/tasks", "/tasks/" + ref,
		"/admin/webhooks", "/transfer"} {
		page := b.page(path)
		if fixedWidth.MatchString(page) {
			t.Errorf("screen %s declares a fixed pixel width", path)
		}
		if strings.Contains(page, "<table") && !strings.Contains(page, "table-layout") &&
			!strings.Contains(page, `class="scroller"`) {
			continue
		}
	}

	resp := b.get("/assets/app.css")
	sheet := body(t, resp)
	if !strings.Contains(sheet, "table-layout: fixed") {
		t.Fatalf("tables are not constrained to the viewport width")
	}
}

func TestBoardStacksItsColumnsAtPhoneWidth(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").get("/assets/app.css")
	sheet := body(t, resp)
	at := strings.Index(sheet, "@media (max-width: 639px)")
	if at < 0 {
		t.Fatal("the stylesheet has no phone-width breakpoint")
	}
	narrow := sheet[at:]
	for _, want := range []string{".board { display: block; }", "thead { display: none; }"} {
		if !strings.Contains(narrow, want) {
			t.Errorf("the phone-width rules are missing %q", want)
		}
	}
}
