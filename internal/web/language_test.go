// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"
)

// TestABrowserAskingForBulgarianGetsBulgarian follows one Accept-Language
// header through every place a page's language shows: the root's lang, the
// navigation, a flash, the table the scripts read, an info tip and the error
// screen. Each assertion reads the element it names, not the page.
func TestABrowserAskingForBulgarianGetsBulgarian(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.lang = "bg-BG,bg;q=0.9,en;q=0.5"

	page := b.page("/tasks?flash=task+created")
	if !strings.Contains(page, `<html lang="bg"`) {
		t.Errorf("root does not say lang=bg")
	}
	menu := nav(t, page)
	if !strings.Contains(menu, "<span>Задачи</span>") || strings.Contains(menu, "<span>Tasks</span>") {
		t.Errorf("navigation is not in Bulgarian:\n%s", menu)
	}
	if flash := between(t, page, `<p class="flash">`, "</p>"); !strings.Contains(flash, "задачата е създадена") {
		t.Errorf("flash = %q, want it translated", flash)
	}
	head := between(t, page, "<head>", "</head>")
	table := between(t, head, `<script id="tix-i18n" type="application/json">`, "</script>")
	if !strings.Contains(table, `"Collapse sidebar":"Свий страничната лента"`) {
		t.Errorf("script table = %s", table)
	}
	tip := between(t, page, `<details class="field-info tip">`, "</details>")
	if !strings.Contains(tip, `aria-label="За „Изгледи“"`) || !strings.Contains(tip, "Едни и същи задачи") {
		t.Errorf("view tip is not in Bulgarian:\n%s", tip)
	}

	resp := b.get("/tasks/nope-404")
	failure := between(t, body(t, resp), `<section class="failure">`, "</section>")
	if !strings.Contains(failure, "<h1>Не е намерено</h1>") {
		t.Errorf("error heading is not in Bulgarian:\n%s", failure)
	}
}

// TestABrowserAskingForNothingGetsEnglish is the other half: no header, or one
// naming only languages this build does not draw, is English with no script
// table at all.
func TestABrowserAskingForNothingGetsEnglish(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, lang := range []string{"", "de-DE,fr;q=0.8"} {
		b := f.as("alice")
		b.lang = lang
		page := b.page("/tasks")
		if !strings.Contains(page, `<html lang="en"`) {
			t.Errorf("%q: root does not say lang=en", lang)
		}
		if !strings.Contains(nav(t, page), "<span>Tasks</span>") {
			t.Errorf("%q: navigation is not in English", lang)
		}
		if strings.Contains(page, `id="tix-i18n"`) {
			t.Errorf("%q: an English page carries a script table", lang)
		}
		tip := between(t, page, `<details class="field-info tip">`, "</details>")
		if !strings.Contains(tip, `aria-label="About Views"`) {
			t.Errorf("%q: view tip = %s", lang, tip)
		}
	}
}
