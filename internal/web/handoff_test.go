// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/handoff"
)

// TestTheTaskScreenHandsTheTaskToAnAgent opens the hand to an agent dialog on
// the task screen and in the task dialog: the button sits in the header, and
// the dialog names this task, its title and the server the reader reached,
// carries every documented client, the MCP prompt, Copy buttons and a Close.
func TestTheTaskScreenHandsTheTaskToAnAgent(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	title := `it's "$(touch x)" and ` + "`id`"
	ref := b.createTask("infra", title)
	shown := strings.ToUpper(ref)
	want := handoff.Task{Ref: shown, Title: title, Server: f.server.URL}

	resp := b.getDialog("/tasks/" + ref)
	inDialog := readAll(t, resp)
	_ = resp.Body.Close()

	for name, tc := range map[string]struct{ page, id string }{
		"page":   {b.page("/tasks/" + ref), "handoff"},
		"dialog": {inDialog, "dlg-handoff"},
	} {
		head := header(t, tc.page)
		if !strings.Contains(head, `data-dlg-open="`+tc.id+`"`) || !strings.Contains(head, "#agent") {
			t.Errorf("%s: the header has no Hand to an agent button opening %s:\n%s", name, tc.id, head)
		}
		dlg := dialogBlock(tc.page, tc.id)
		if dlg == "" {
			t.Fatalf("%s: no dialog %s on the task screen", name, tc.id)
		}
		text := html.UnescapeString(dlg)
		prefix := strings.TrimSuffix(tc.id, "handoff")
		for _, s := range handoff.For(want) {
			if !strings.Contains(text, `data-handoff-tab="`+s.ID+`"`) || !strings.Contains(text, `data-handoff-panel="`+s.ID+`"`) {
				t.Errorf("%s: no tab or panel for %s", name, s.Name)
			}
			if s.Command != "" && !strings.Contains(text, `id="`+prefix+`handoff-cmd-`+s.ID+`"><code>`+s.Command+"</code>") {
				t.Errorf("%s: %s's command is not drawn as %q", name, s.Name, s.Command)
			}
			if s.Command == "" && !strings.Contains(text, `id="`+prefix+`handoff-paste-`+s.ID+`"><code>`+s.Prompt+"</code>") {
				t.Errorf("%s: %s's prompt to paste is not drawn as %q", name, s.Name, s.Prompt)
			}
		}
		for id, block := range map[string]string{"handoff-prompt": handoff.Prompt(want), "handoff-mcp": handoff.MCPPrompt(want)} {
			if !strings.Contains(text, `id="`+prefix+id+`"><code>`+block+"</code>") {
				t.Errorf("%s: block %s does not carry %q", name, id, block)
			}
		}
		for _, part := range []string{shown, f.server.URL, "tix claim task " + shown} {
			if !strings.Contains(text, part) {
				t.Errorf("%s: the dialog does not name %q", name, part)
			}
		}
		if n := strings.Count(dlg, "data-copy-target="); n != len(handoff.Clients)+2 {
			t.Errorf("%s: %d Copy buttons, want one per client and two prompts", name, n)
		}
		if !strings.Contains(dlg, "data-dlg-close") || !strings.Contains(dlg, `aria-label="Close"`) {
			t.Errorf("%s: the dialog has no Close", name)
		}
		if strings.Contains(text, "TIX_TOKEN") {
			t.Errorf("%s: the dialog carries a token variable", name)
		}
	}
}

// TestTheHandoffDialogDrawsInPlaceWithoutScripting follows the button's own
// link, the way a browser without scripting does: the dialog is drawn open,
// with every client's block under its heading and a way back.
func TestTheHandoffDialogDrawsInPlaceWithoutScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "fix the build")
	page := b.page("/tasks/" + ref + "?dialog=handoff")
	dlg := dialogBlock(page, "handoff")
	if !strings.Contains(dlg, " is-inplace") || !strings.Contains(dlg, " open") {
		t.Fatalf("the dialog is not drawn open in place:\n%s", dlg)
	}
	if !strings.Contains(dlg, `class="handoff-tabs" role="tablist"`) || !strings.Contains(dlg, `data-needs-js`) {
		t.Error("the tab strip is drawn without scripting")
	}
	for _, c := range handoff.Clients {
		if !strings.Contains(dlg, `<h4 class="handoff-name" id="handoff-name-`+c.ID+`">`+html.EscapeString(c.Name)+`</h4>`) {
			t.Errorf("no heading for %s's block", c.Name)
		}
	}
	if !strings.Contains(dlg, `href="/tasks/`+ref+`" data-dlg-close`) {
		t.Error("the dialog has no way back to the task without scripting")
	}
	if resp := b.get("/tasks/" + ref + "?dialog=handoff"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET with the dialog open = %d", resp.StatusCode)
	}
}
