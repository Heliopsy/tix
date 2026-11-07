// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"fmt"
	"testing"
)

// livestaleLayouts is where every element of the page's main column sits,
// leaving out the notice itself, with the notice showing and then hidden. Both
// are read in one turn of the page's event loop, so no live refresh or late
// font can land between them.
const livestaleLayouts = `(() => {
	const notice = document.getElementById("live-stale");
	const layout = () => JSON.stringify([...document.querySelectorAll("main *")]
		.filter((el) => !notice.contains(el))
		.map((el) => { const r = el.getBoundingClientRect(); return [Math.round(r.top), Math.round(r.left), el.tagName + "." + el.className]; }));
	const shown = layout();
	notice.hidden = true;
	const hidden = layout();
	notice.hidden = false;
	return {Shown: shown, Hidden: hidden};
})()`

// The notice that a screen is not live arrives seconds after the page, when
// the socket gives up. As a block above the list it pushed every row down
// under a pointer already on its way to one. The fixture serves no event
// stream, so the notice always comes; with it showing, and with it hidden
// again, nothing else on the page may be anywhere different.
func TestTheNotLiveNoticeMovesNothingInABrowser(t *testing.T) {
	b, ref := rawFixture(t)
	f := b.fix
	page := browserPage(t, f)
	for _, tc := range []struct{ path, view string }{
		{"/activity", ""},
		{"/tasks", "list"},
		{"/tasks", "board"},
		{"/tasks", "lanes"},
		{"/projects/infra", ""},
		{"/tasks/" + ref, ""},
	} {
		t.Run(tc.path+" "+tc.view, func(t *testing.T) {
			for _, vp := range []struct{ w, h int }{{1280, 900}, {390, 844}} {
				page.call("Emulation.setDeviceMetricsOverride", map[string]any{
					"width": vp.w, "height": vp.h, "deviceScaleFactor": 1, "mobile": vp.w < 640})
				if tc.view != "" {
					page.load(t, f.server.URL+"/tasks", "true")
					page.eval(t, `document.cookie = "tix_task_view=`+tc.view+`; path=/"`, nil)
				}
				page.load(t, f.server.URL+tc.path, `document.getElementById("live-stale")`)
				page.waitFor(t, `!document.getElementById("live-stale").hidden`,
					"the screen never said it was not live")
				var height float64
				page.eval(t, `document.getElementById("live-stale").getBoundingClientRect().height`, &height)
				if height == 0 {
					t.Fatalf("%dpx: the notice is up but has no height, so this measures nothing", vp.w)
				}
				var got struct{ Shown, Hidden string }
				page.eval(t, livestaleLayouts, &got)
				if got.Shown != got.Hidden {
					t.Errorf("%dpx: the page moves when the not-live notice appears: %s", vp.w, firstMoved(got.Shown, got.Hidden))
				}
			}
		})
	}
}

// firstMoved names the first element that sits elsewhere in two layouts.
func firstMoved(a, b string) string {
	var x, y [][]any
	_ = json.Unmarshal([]byte(a), &x)
	_ = json.Unmarshal([]byte(b), &y)
	for i := range min(len(x), len(y)) {
		if fmt.Sprint(x[i]) != fmt.Sprint(y[i]) {
			return fmt.Sprintf("%v with it, %v without", x[i], y[i])
		}
	}
	return fmt.Sprintf("%d elements with it, %d without", len(x), len(y))
}
