// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"regexp"
	"strings"
	"testing"
)

var verticalMargin = regexp.MustCompile(`(?m)(^|;)\s*(margin|margin-top|margin-bottom|margin-block)\s*:([^;]*)`)

// headingSubject reports whether a selector's subject is part of a signed-in
// screen's page heading: the h1, its standfirst, the header strip, or the
// task's title bar.
func headingSubject(selector string) bool {
	if strings.Contains(selector, ".auth") {
		return false
	}
	for _, s := range subjects(selector) {
		for _, p := range []string{"h1", ".lede", ".pagehead", ".titlebar", ":is(.lede"} {
			if s == p || strings.HasPrefix(s, p+":") || strings.HasPrefix(s, p+".") || strings.HasPrefix(s, p+"[") || strings.HasPrefix(s, p+",") {
				return true
			}
		}
	}
	return false
}

// TestThePageHeadingKeepsOneRhythm pins the space under a page heading to
// two tokens.
//
// Every screen reached the same distance its own way: an h1 margin, a lede
// pulled up by -8px and pushed down by 18px, a header strip whose button was
// taller than its heading, an h2 whose top margin collapsed with the h1's. The
// gap from a heading to the first thing under it came out anywhere from 8 to
// 42px across twenty-odd screens. Any margin on the heading's parts is now
// either zero or written in --head-gap and --head-sub.
func TestThePageHeadingKeepsOneRhythm(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("alice").get("/assets/app.css"))
	rules := cssRules(sheet)

	seen := 0
	for _, r := range rules {
		if !headingSubject(r.selector) {
			continue
		}
		for _, m := range verticalMargin.FindAllStringSubmatch(r.body, -1) {
			seen++
			value := strings.TrimSpace(m[3])
			if value == "0" || strings.Contains(value, "var(--head-") {
				continue
			}
			t.Errorf("%s %s sets %s: %s, a heading margin outside the rhythm tokens", r.media, r.selector, m[2], value)
		}
	}
	if seen < 5 {
		t.Fatalf("found %d heading margins, the stylesheet lost the rules this guards", seen)
	}

	owns := false
	for _, r := range rules {
		if r.media != "" || !strings.HasPrefix(r.selector, ".wrap > :is(h1, .pagehead") {
			continue
		}
		if strings.Contains(r.selector, "+ :not(.lede, .meta)") && strings.Contains(r.body, "margin-top: 0") {
			owns = true
		}
	}
	if !owns {
		t.Error("nothing zeroes the top margin of what follows the heading, so an h2 there collapses its own margin with the heading's")
	}

	tools := ""
	for _, r := range rules {
		if r.selector == ".pagehead-tools" && r.media == "@media (min-width: 900px)" {
			tools = r.body
		}
	}
	if !strings.Contains(tools, "margin-block: calc(") || !strings.Contains(tools, "var(--control-h)") {
		t.Errorf("the header controls are not centred on the heading's line, so a header with a button is deeper than one without:\n%s", tools)
	}
}

var controlHeightDecl = regexp.MustCompile(`(?m)(^|;)\s*(height|min-height|max-height|line-height|padding|padding-top|padding-bottom|padding-block)\s*:([^;]*)`)

var controlSubject = regexp.MustCompile(`^(input|select|textarea|button|summary|a)([.:\[]|$)|\.(segment|clear|addmore-open|filterbtn|colmenu-open)\b`)

// stylesAControl reports whether any selector of a list styles a control
// rather than the row around it or a panel the row opens.
func stylesAControl(selector string) bool {
	for _, s := range subjects(selector) {
		if controlSubject.MatchString(s) {
			return true
		}
	}
	return false
}

// TestEveryControlInARowSharesOneHeight keeps the task screen's control rows
// at --control-h.
//
// The quick-add title box carried its own 11px padding and stood 49px tall
// beside a 36px select and button, and the Clear link sized itself from its
// padding and line height. The height of a control in these rows comes from
// the control rules and nowhere else.
func TestEveryControlInARowSharesOneHeight(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	sheet := body(t, b.get("/assets/app.css"))
	rules := cssRules(sheet)

	for _, sel := range []string{"input, select, textarea", "button", "a.secondary.clear"} {
		found := false
		for _, r := range rules {
			if r.selector == sel && r.media == "" {
				found = true
				if !strings.Contains(r.body, "min-height: var(--control-h);") {
					t.Errorf("%s does not take its height from --control-h:\n%s", sel, r.body)
				}
			}
		}
		if !found {
			t.Fatalf("the stylesheet has no %s rule", sel)
		}
	}

	seen := 0
	for _, r := range rules {
		if !strings.Contains(r.selector, ".quickadd") && !strings.Contains(r.selector, ".filterbar") && !strings.Contains(r.selector, ".pagehead-tools") {
			continue
		}
		if !stylesAControl(r.selector) {
			continue
		}
		seen++
		for _, m := range controlHeightDecl.FindAllStringSubmatch(r.body, -1) {
			value := strings.TrimSpace(m[3])
			switch m[2] {
			case "padding":
				if first := strings.Fields(value)[0]; first == "0" {
					continue
				}
			case "min-height", "height":
				if value == "var(--control-h)" || value == "var(--control-h-sm)" {
					continue
				}
			}
			t.Errorf("%s %s sets %s: %s, so a control in the row is a different height from its neighbours", r.media, r.selector, m[2], value)
		}
	}
	if seen < 4 {
		t.Fatalf("found %d rules for the control rows, the stylesheet lost them", seen)
	}

	page := b.page("/tasks")
	row := between(t, page, `class="quickadd">`, "</form>")
	if row == "" {
		t.Fatal("the task screen has no quick-add row")
	}
	open := tagWith(t, row, `popovertarget="addmore"`)
	for _, want := range []string{`type="button"`, `class="secondary addmore-open"`} {
		if !strings.Contains(open, want) {
			t.Errorf("Add with details is not a secondary button in the quick-add row (%s):\n%s", want, open)
		}
	}
	if !strings.Contains(row, "Add with details") {
		t.Errorf("the button in the quick-add row lost its name:\n%s", row)
	}
	if strings.Contains(page, `<details class="panel addmore">`) {
		t.Error("the task screen still draws Add with details as a disclosure on a strip of its own")
	}
	pop := between(t, page, `<div id="addmore" class="addpop" popover>`, "</form>")
	if !strings.Contains(pop, `action="/tasks"`) || !strings.Contains(pop, `name="body"`) {
		t.Errorf("the popover does not hold the fuller add form:\n%s", pop)
	}
}

// TestThePageMeasureIsOneRule keeps the content column's width cap the same
// on every screen, so its right edge does not move between them any more than
// its left edge does.
func TestThePageMeasureIsOneRule(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("alice").get("/assets/app.css"))

	base := 0
	for _, r := range cssRules(sheet) {
		wrap := false
		for _, s := range subjects(r.selector) {
			if s == ".wrap" || strings.HasPrefix(s, ".wrap:") {
				wrap = true
			}
		}
		if !wrap || (!strings.Contains(r.body, "max-width") && !strings.Contains(r.body, " width:")) {
			continue
		}
		if r.selector == ".wrap" && r.media == "" {
			base++
			if !strings.Contains(r.body, "max-width: var(--measure-page);") {
				t.Errorf("the content column is not capped by --measure-page:\n%s", r.body)
			}
			continue
		}
		t.Errorf("%s %s gives one screen a measure of its own:\n%s", r.media, r.selector, r.body)
	}
	if base != 1 {
		t.Fatalf("found %d base .wrap rules carrying the measure, want exactly one", base)
	}
}
