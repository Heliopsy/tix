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

// formTag returns the opening tag of the form submitting to action, so an
// assertion about a form reads that form and not the whole page. Two guards in
// this package passed while the screen was broken because they searched the
// document for a string some other element happened to carry.
func formTag(t *testing.T, page, action string) string {
	t.Helper()
	at := strings.Index(page, `action="`+action+`"`)
	if at < 0 {
		t.Fatalf("no form submits to %s", action)
	}
	start := strings.LastIndex(page[:at], "<form")
	end := strings.Index(page[at:], ">")
	if start < 0 || end < 0 {
		t.Fatalf("the form submitting to %s is not a tag", action)
	}
	return page[start : at+end+1]
}

// declarations returns the body of the first rule whose selector list carries
// selector, so a stylesheet assertion reads the rule it names rather than
// finding the text somewhere else in the sheet.
func declarations(t *testing.T, sheet, selector string) string {
	t.Helper()
	at := strings.Index(sheet, selector)
	if at < 0 {
		t.Fatalf("the stylesheet has no %s rule", selector)
	}
	start := strings.Index(sheet[at:], "{")
	if start < 0 {
		t.Fatalf("the %s rule opens no block", selector)
	}
	start += at
	end := strings.Index(sheet[start:], "}")
	if end < 0 {
		t.Fatalf("the %s rule never closes", selector)
	}
	return sheet[start : start+end+1]
}

// TestTheStatisticsFilterIsARowNotTwoFullWidthDropdowns pins the repair of the
// Window and Project controls.
//
// They had a .filters rule of their own whose flex items were the selects
// themselves, so the width: 100% every form control carries measured the whole
// content column: two dropdowns a thousand pixels wide holding "14 days" and
// "Every project", stacked over the figures they filter and taking the top
// third of the screen. The task list already had a filter row that sizes a
// select to its content, so this one uses it.
func TestTheStatisticsFilterIsARowNotTwoFullWidthDropdowns(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	form := formTag(t, b.page("/stats"), "/stats")
	if !strings.Contains(form, "filterbar") {
		t.Errorf("the statistics filter is not the shared filter row: %s", form)
	}
	if !strings.Contains(form, `method="get"`) {
		t.Errorf("the statistics filter no longer submits without script: %s", form)
	}

	sheet := body(t, b.get("/assets/app.css"))
	if sized := declarations(t, sheet, ".filterbar select"); !strings.Contains(sized, "width: auto") {
		t.Errorf("a filter row's select is not sized to its content: %s", sized)
	}
	if row := declarations(t, sheet, ".statsfilter {"); !strings.Contains(row, "flex-wrap: wrap") {
		t.Errorf("the statistics filter cannot wrap, so it overflows a phone: %s", row)
	}
}
