// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The web reconnected without a word: nothing in the shell said whether
// changes made elsewhere were reaching the page, or why not.
func TestTheShellCarriesTheLiveIndicatorAndItsDetails(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/tasks")

	head := between(t, page, `<div class="side-head">`, `<div id="sidemenu"`)
	if !strings.Contains(head, `data-live-status`) || !strings.Contains(head, `aria-haspopup="dialog"`) {
		t.Errorf("the sidebar's head has no live indicator opening a dialog:\n%s", head)
	}

	dialog := between(t, page, `<dialog id="live-details"`, `</dialog>`)
	if dialog == "" {
		t.Fatalf("the page has no connection details dialog:\n%s", page)
	}
	for _, want := range []string{
		"Connection details", "Last error", "Carried over", "Recent changes",
		"data-live-reconnect", "Reconnect now", "data-live-copy", "Copy details", "data-live-close",
	} {
		if !strings.Contains(dialog, want) {
			t.Errorf("the connection details lack %s:\n%s", want, dialog)
		}
	}

	status := strings.Index(page, `src="/assets/livestatus.js"`)
	live := strings.Index(page, `src="/assets/live.js"`)
	if status < 0 || live < 0 || status > live {
		t.Error("livestatus.js must load before live.js, which reports to it as it binds")
	}
}

// The words the script writes come from the page, in the page's language.
func TestTheLiveIndicatorSpeaksThePagesLanguage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post("/language", url.Values{"lang": {"bg"}, "next": {"/tasks"}})
	_ = resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		t.Fatalf("choosing Bulgarian answered %d", resp.StatusCode)
	}
	page := b.page("/tasks")
	raw := between(t, page, `<script id="tix-live-words" type="application/json">`, `</script>`)
	var words map[string]string
	if err := json.Unmarshal([]byte(html.UnescapeString(raw)), &words); err != nil {
		t.Fatalf("the indicator's words are not JSON: %v\n%s", err, raw)
	}
	if words["mode:offline"] != "Няма връзка" || words["mode:live"] != "На живо" {
		t.Errorf("the indicator's words are not in the page's language: %v", words)
	}
	for _, key := range []string{"info:polling", "hint:upgrade", "error:closed", "transport:poll", "Live updates: %s"} {
		if words[key] == "" {
			t.Errorf("the indicator's words lack %s", key)
		}
	}
}
