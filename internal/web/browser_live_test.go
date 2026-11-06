// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/testenv"
	"github.com/heliopsy/tix/internal/web"
)

// A burst of live updates used to take the screen away from the reader. Each
// update replaced the task list, the board's columns and the lanes whole, so
// a scroll in progress was scrolling a box that had just been thrown away and
// snapped back, every scrollbar on the screen was redrawn and blinked, rows
// replayed their entrance, and a task added above the reader pushed what they
// were reading down. None of that is visible to the JavaScript suite, which
// has no layout, so these drive the real templates and assets in Chrome.

// chromeOrSkip finds Chrome, or fails on CI and skips elsewhere, the same way
// the drag test does.
func chromeOrSkip(t *testing.T) string {
	t.Helper()
	chrome := findChrome()
	if chrome == "" {
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			t.Fatalf("no Chrome on this CI runner: %s", chromeCapability.Why)
		}
		testenv.Skip(t, chromeCapability)
	}
	return chrome
}

// createAt adds a task with a priority, so a test can put it above or below
// the rows on screen under the default urgency sort.
func (b *browser) createAt(project, title, priority string) {
	b.t.Helper()
	resp := b.post("/tasks", url.Values{"project_ref": {project}, "title": {title}, "priority": {priority}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(b.t, resp, http.StatusSeeOther)
}

// openLive loads a task screen and waits until its scripts have run and its
// live binding is in place.
func (p *cdpPage) openLive(t *testing.T, address, ready string) {
	t.Helper()
	p.call("Page.navigate", map[string]any{"url": address})
	p.waitFor(t, `document.readyState === "complete" && window.htmx && window.tix && window.tix.morph &&
		document.getElementById('live-screen') && document.querySelector(`+fmt.Sprintf("%q", ready)+`)`,
		"the screen never finished loading at "+address)
	p.waitFor(t, `document.getElementById('live-stale')?.hidden === false`,
		"the screen never said it was not live at "+address)
}

// burst asks the screen for a refresh the way a run of events does, many
// times in quick succession, and waits for the redraws to settle. While it
// runs, every frame records what the reader would see change: the pointer,
// the busy marks, and whether a scrolling box gained or lost a scrollbar.
func (p *cdpPage) burst(t *testing.T, boxes string) {
	t.Helper()
	p.eval(t, `(() => {
		window.__regions = 0;
		document.addEventListener('tix:regions', () => window.__regions++);
		const boxes = () => [...document.querySelectorAll(`+fmt.Sprintf("%q", boxes)+`)];
		const shape = () => boxes().map(b => [b.scrollWidth > b.clientWidth, b.scrollHeight > b.clientHeight, b.clientWidth, b.clientHeight].join()).join('|');
		const first = shape();
		window.__frames = {cursors: new Set(), busy: 0, shapes: new Set([first]), stop: false};
		const tick = () => {
			const f = window.__frames;
			for (const el of [document.documentElement, document.body, ...boxes()]) f.cursors.add(getComputedStyle(el).cursor);
			f.busy = Math.max(f.busy, document.querySelectorAll('.htmx-request, [aria-busy="true"], .is-busy').length);
			f.shapes.add(shape());
			if (!f.stop) requestAnimationFrame(tick);
		};
		new MutationObserver(() => window.__frames.shapes.add(shape())).observe(document.body, {subtree: true, childList: true});
		requestAnimationFrame(tick);
		let n = 0;
		const fire = () => { document.dispatchEvent(new CustomEvent('tix:refresh')); if (++n < 12) setTimeout(fire, 40); };
		fire();
	})()`, nil)
	p.waitFor(t, `window.__regions >= 2`, "the burst never redrew the screen twice")
	time.Sleep(700 * time.Millisecond)
	p.eval(t, `window.__frames.stop = true`, nil)
	var frames struct {
		Cursors []string `json:"cursors"`
		Busy    int      `json:"busy"`
		Shapes  []string `json:"shapes"`
	}
	p.eval(t, `({cursors: [...window.__frames.cursors], busy: window.__frames.busy, shapes: [...window.__frames.shapes]})`, &frames)
	for _, c := range frames.Cursors {
		if c == "progress" || c == "wait" {
			t.Errorf("a background refresh showed the %q pointer", c)
		}
	}
	if frames.Busy > 0 {
		t.Errorf("a background refresh marked %d elements busy", frames.Busy)
	}
	if len(frames.Shapes) > 1 {
		t.Errorf("a scrolling box gained or lost a scrollbar, or changed size, during the burst: %q", frames.Shapes)
	}
}

// keep records an element under a name, to ask later whether it is still the
// same element.
func (p *cdpPage) keep(t *testing.T, name, expr string) {
	t.Helper()
	var ok bool
	p.eval(t, `(() => { window.__kept = window.__kept || {}; window.__kept[`+fmt.Sprintf("%q", name)+`] = `+expr+`; return !!window.__kept[`+fmt.Sprintf("%q", name)+`]; })()`, &ok)
	if !ok {
		t.Fatalf("nothing on the page for %s", name)
	}
}

func (p *cdpPage) same(t *testing.T, name, expr string) bool {
	t.Helper()
	var ok bool
	p.eval(t, `window.__kept[`+fmt.Sprintf("%q", name)+`] === (`+expr+`)`, &ok)
	return ok
}

func TestLiveUpdatesLeaveTheReaderWhereTheyAreInABrowser(t *testing.T) {
	chrome := chromeOrSkip(t)
	f := newFixture(t)
	b := f.as("alice")
	seedProject(t, f.store, f.tenantA.ID, "ops")
	seedProject(t, f.store, f.tenantA.ID, "docs")
	for i := range 40 {
		b.createAt("infra", fmt.Sprintf("Steady row %02d", i), "3")
	}
	for i := range 12 {
		b.createAt("ops", fmt.Sprintf("Ops card %02d", i), "3")
		b.createAt("docs", fmt.Sprintf("Docs card %02d", i), "3")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	page := launchChrome(ctx, t, chrome)
	page.call("Network.enable", nil)
	page.call("Page.enable", nil)
	page.call("Network.setExtraHTTPHeaders", map[string]any{"headers": map[string]string{actorHeader: "alice"}})
	view := func(v string) {
		page.call("Network.setCookie", map[string]any{"name": web.TaskViewCookie, "value": v, "url": f.server.URL})
	}

	t.Run("list", func(t *testing.T) {
		view(web.TaskViewList)
		page.openLive(t, f.server.URL+"/tasks?limit=100", "ul.tasklist li.task")
		var anchor string
		page.eval(t, `(() => {
			const box = document.querySelector('ul.tasklist');
			box.scrollTop = 400;
			const top = box.getBoundingClientRect().top;
			const row = [...box.querySelectorAll('li.task')].find(r => r.getBoundingClientRect().top >= top);
			window.__anchorTop = row.getBoundingClientRect().top;
			row.focus({preventScroll: true});
			return row.id;
		})()`, &anchor)
		page.keep(t, "box", `document.querySelector('ul.tasklist')`)
		page.keep(t, "row", `document.getElementById(`+fmt.Sprintf("%q", anchor)+`)`)
		page.keep(t, "listing", `document.getElementById('tasks-listing')`)

		for i := range 3 {
			b.createAt("infra", fmt.Sprintf("Urgent arrival %d", i), "1")
		}
		b.createAt("infra", "Quiet arrival", "5")
		page.burst(t, "ul.tasklist")

		if !page.same(t, "box", `document.querySelector('ul.tasklist')`) {
			t.Error("the list's scrolling box was replaced, so a scroll in progress was lost")
		}
		if !page.same(t, "row", `document.getElementById(`+fmt.Sprintf("%q", anchor)+`)`) {
			t.Error("the row the reader was on was redrawn instead of kept")
		}
		var moved float64
		page.eval(t, `Math.round(document.getElementById(`+fmt.Sprintf("%q", anchor)+`).getBoundingClientRect().top - window.__anchorTop)`, &moved)
		if moved != 0 {
			t.Errorf("rows added above pushed the row the reader was on by %vpx", moved)
		}
		if !page.same(t, "row", `document.activeElement`) {
			t.Error("focus left the row the reader had focused")
		}
		var pill string
		page.eval(t, `document.querySelector('.livepill:not([hidden])')?.textContent ?? ''`, &pill)
		if pill != "3 new, show" {
			t.Errorf("the pill says %q, want %q", pill, "3 new, show")
		}
		var top float64
		page.eval(t, `(() => { document.querySelector('.livepill:not([hidden])').click(); return 0; })()`, nil)
		page.waitFor(t, `document.querySelector('ul.tasklist').scrollTop === 0`, "pressing the pill did not go to the new rows")
		page.eval(t, `document.querySelector('ul.tasklist li.task').textContent.includes('Urgent arrival') ? 0 : 1`, &top)
		if top != 0 {
			t.Error("the top of the list is not the rows the pill announced")
		}
	})

	t.Run("an open menu holds the refresh until it closes", func(t *testing.T) {
		view(web.TaskViewList)
		page.openLive(t, f.server.URL+"/tasks?limit=100", "ul.tasklist li.task [popover]")
		var id string
		page.eval(t, `(() => { const m = document.querySelector('ul.tasklist li.task [popover]'); m.showPopover(); return m.id; })()`, &id)
		page.keep(t, "menu", `document.getElementById(`+fmt.Sprintf("%q", id)+`)`)
		b.createAt("infra", "Arrives under an open menu", "1")
		page.eval(t, `(() => { window.__regions = 0; document.addEventListener('tix:regions', () => window.__regions++); document.dispatchEvent(new CustomEvent('tix:refresh')); })()`, nil)
		time.Sleep(900 * time.Millisecond)
		var open bool
		page.eval(t, `document.getElementById(`+fmt.Sprintf("%q", id)+`).matches(':popover-open')`, &open)
		if !open {
			t.Fatal("a live update closed the menu the reader had open")
		}
		page.eval(t, `document.getElementById(`+fmt.Sprintf("%q", id)+`).hidePopover()`, nil)
		page.waitFor(t, `window.__regions >= 1`, "the held refresh never landed once the menu closed")
		if !page.same(t, "menu", `document.getElementById(`+fmt.Sprintf("%q", id)+`)`) {
			t.Error("the menu's row was redrawn instead of kept")
		}
	})

	page.call("Emulation.setDeviceMetricsOverride", map[string]any{
		"width": 900, "height": 800, "deviceScaleFactor": 1, "mobile": false})

	// The box is the first column or lane, which the strip is scrolled past
	// to the next one: a snap point, so the strip stays where it is put.
	for _, c := range []struct{ view, strip, box string }{
		{web.TaskViewBoard, ".board", ".board .column"},
		{web.TaskViewLanes, ".lanes", ".lanes .lane"},
	} {
		t.Run(c.view, func(t *testing.T) {
			view(c.view)
			page.openLive(t, f.server.URL+"/tasks?limit=100", c.box+" [id^=t-]")
			var wide string
			page.eval(t, `(() => {
				const strip = document.querySelector(`+fmt.Sprintf("%q", c.strip)+`);
				const box = document.querySelector(`+fmt.Sprintf("%q", c.box)+`);
				strip.scrollLeft = box.nextElementSibling.offsetLeft - strip.offsetLeft;
				box.scrollTop = 120;
				window.__left = strip.scrollLeft;
				const card = [...box.querySelectorAll('[id^=t-]')].find(r => r.getBoundingClientRect().top >= box.getBoundingClientRect().top);
				window.__card = card;
				window.__cardAt = [card.getBoundingClientRect().left, card.getBoundingClientRect().top];
				return strip.scrollWidth > strip.clientWidth && strip.scrollLeft > 0 && box.scrollTop > 0 ? "" :
					[strip.scrollWidth, strip.clientWidth, strip.scrollLeft, box.scrollHeight, box.clientHeight, box.scrollTop].join(" ");
			})()`, &wide)
			if wide != "" {
				t.Fatalf("the %s does not scroll both ways at this size, so this checks nothing: %s", c.view, wide)
			}
			page.keep(t, "strip", `document.querySelector(`+fmt.Sprintf("%q", c.strip)+`)`)
			page.keep(t, "box", `document.querySelector(`+fmt.Sprintf("%q", c.box)+`)`)

			for i := range 3 {
				b.createAt("infra", fmt.Sprintf("%s arrival %d", c.view, i), "1")
			}
			page.burst(t, c.strip+", "+c.box)

			if !page.same(t, "strip", `document.querySelector(`+fmt.Sprintf("%q", c.strip)+`)`) {
				t.Errorf("the %s's sideways strip was replaced", c.view)
			}
			if !page.same(t, "box", `document.querySelector(`+fmt.Sprintf("%q", c.box)+`)`) {
				t.Errorf("the %s's first column was replaced", c.view)
			}
			var left float64
			page.eval(t, `document.querySelector(`+fmt.Sprintf("%q", c.strip)+`).scrollLeft - window.__left`, &left)
			if left != 0 {
				t.Errorf("the %s scrolled sideways by %vpx", c.view, left)
			}
			var shift []float64
			page.eval(t, `window.__card.isConnected ? [Math.round(window.__card.getBoundingClientRect().left - window.__cardAt[0]), Math.round(window.__card.getBoundingClientRect().top - window.__cardAt[1])] : [999, 999]`, &shift)
			if len(shift) != 2 || shift[0] != 0 || shift[1] != 0 {
				t.Errorf("the card the reader was looking at moved by %v", shift)
			}
		})
	}
}

func TestPendingButtonsInABrowser(t *testing.T) {
	chrome := chromeOrSkip(t)
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "Something on the list")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	page := launchChrome(ctx, t, chrome)
	page.call("Network.enable", nil)
	page.call("Page.enable", nil)
	page.call("Network.setExtraHTTPHeaders", map[string]any{"headers": map[string]string{actorHeader: "alice"}})
	page.call("Network.setCookie", map[string]any{"name": web.TaskViewCookie, "value": web.TaskViewList, "url": f.server.URL})
	page.openLive(t, f.server.URL+"/tasks", "form.quickadd button[type=submit]")
	// Every post to /tasks waits for ever, so the pending state holds while
	// it is measured.
	page.call("Fetch.enable", map[string]any{"patterns": []map[string]any{{"urlPattern": "*/tasks", "requestStage": "Request"}}})

	var got struct {
		Busy    bool    `json:"busy"`
		Color   string  `json:"color"`
		Width   float64 `json:"width"`
		Before  float64 `json:"before"`
		Submits int     `json:"submits"`
	}
	page.eval(t, `(() => {
		const form = document.querySelector('form.quickadd');
		const btn = form.querySelector('button[type=submit]');
		form.querySelector('input[name=title]').value = 'Held in flight';
		let submits = 0;
		form.addEventListener('submit', () => submits++);
		const before = btn.getBoundingClientRect().width;
		form.requestSubmit(btn);
		const busy = btn.classList.contains('is-busy');
		const color = getComputedStyle(btn).color;
		form.requestSubmit(btn);
		return {busy, color, width: btn.getBoundingClientRect().width, before, submits};
	})()`, &got)
	if !got.Busy {
		t.Error("the pressed button was not marked pending in the same frame as the press")
	}
	if !strings.HasSuffix(strings.ReplaceAll(got.Color, " ", ""), ",0)") {
		t.Errorf("the label of a text-only button was still drawn (%s) under the spinner", got.Color)
	}
	if got.Width != got.Before {
		t.Errorf("the button changed width from %v to %v while pending", got.Before, got.Width)
	}
	if got.Submits != 1 {
		t.Errorf("a second press on a pending button submitted again (%d submits)", got.Submits)
	}

	var icon struct {
		Overlap bool    `json:"overlap"`
		Label   string  `json:"label"`
		Width   float64 `json:"width"`
		Before  float64 `json:"before"`
	}
	page.eval(t, `(() => {
		const form = document.querySelector('form.quickadd');
		const old = form.querySelector('button[type=submit]');
		old.classList.remove('is-busy', 'has-busy-slot'); old.disabled = false;
		const btn = document.createElement('button');
		btn.type = 'submit';
		btn.className = 'secondary';
		btn.innerHTML = '<svg class="nav-icon" aria-hidden="true" width="16" height="16"><use href="/assets/icons.svg#plus"/></svg>Add the task';
		form.appendChild(btn);
		const before = btn.getBoundingClientRect().width;
		form.requestSubmit(btn);
		const box = btn.getBoundingClientRect();
		const after = getComputedStyle(btn, '::after');
		const s = {left: box.left + btn.clientLeft + parseFloat(after.left), top: box.top + btn.clientTop + parseFloat(after.top)};
		s.right = s.left + parseFloat(after.width);
		s.bottom = s.top + parseFloat(after.height);
		const range = document.createRange();
		range.selectNodeContents(btn.lastChild);
		const text = range.getBoundingClientRect();
		const overlap = s.left < text.right && text.left < s.right && s.top < text.bottom && text.top < s.bottom;
		return {overlap, label: getComputedStyle(btn).color, width: btn.getBoundingClientRect().width, before};
	})()`, &icon)
	if icon.Overlap {
		t.Error("the spinner on a button with an icon is drawn over its label")
	}
	if strings.HasSuffix(strings.ReplaceAll(icon.Label, " ", ""), ",0)") {
		t.Error("the label of a button with an icon was hidden; the spinner should take the icon's place")
	}
	if icon.Width != icon.Before {
		t.Errorf("the button with an icon changed width from %v to %v while pending", icon.Before, icon.Width)
	}
}
