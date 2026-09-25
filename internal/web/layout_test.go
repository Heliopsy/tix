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

// selectorBefore returns the selector of the rule containing the declaration
// at index at, by walking back to the end of the previous rule.
func selectorBefore(sheet string, at int) string {
	head := sheet[:at]
	start := strings.LastIndexAny(head, "}{")
	if start < 0 {
		return head
	}
	// A "{" means we are inside the rule whose selector precedes it.
	if head[start] == '{' {
		head = head[:start]
		if prev := strings.LastIndexAny(head, "}{"); prev >= 0 {
			return strings.TrimSpace(head[prev+1:])
		}
		return strings.TrimSpace(head)
	}
	return strings.TrimSpace(head[start+1:])
}

// TestTheTickAnimatesOnlyTheRowThatWasActedOn pins the fix for a list that
// announced work nobody did.
//
// The pop and draw were attached to button.check.is-done, which is the state of
// being complete rather than the act of completing. Every render therefore
// replayed them on every already-ticked checkbox: opening the list, changing a
// filter or switching a column sent a wave of ticks down the page, and none of
// them corresponded to anything the reader had done. Scoping them to the row
// carrying .is-target leaves exactly one.
func TestTheTickAnimatesOnlyTheRowThatWasActedOn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").get("/assets/app.css")
	wantStatus(t, resp, http.StatusOK)
	sheet := body(t, resp)

	var found int
	for _, decl := range []string{"animation: pop ", "animation: draw "} {
		for at := 0; ; {
			i := strings.Index(sheet[at:], decl)
			if i < 0 {
				break
			}
			i += at
			at = i + len(decl)
			found++
			sel := selectorBefore(sheet, i)
			if !strings.Contains(sel, ".is-ticked") {
				t.Errorf("%q is declared under %q, which fires on every render of an\n"+
					"already-completed task; it belongs under .is-ticked, the mark that\n"+
					"clears itself, not .is-target, which holds until the reader moves on",
					decl, sel)
			}
		}
	}
	if found != 2 {
		t.Fatalf("found %d tick animation declarations, want the pop and the draw", found)
	}

	// The completed look itself must not depend on the animation, or a page
	// rendered without JavaScript loses its ticks entirely.
	done := strings.Index(sheet, "button.check.is-done {")
	if done < 0 {
		t.Fatal("no rule paints the completed state")
	}
	block := sheet[done : done+strings.Index(sheet[done:], "}")]
	if !strings.Contains(block, "background: var(--ok)") {
		t.Error("the completed state is not painted outside the animation")
	}
}
