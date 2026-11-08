// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// shareMenu is the task screen's Share menu and nothing else of the page.
func shareMenu(t *testing.T, page, id string) string {
	t.Helper()
	menu := popover(t, page, id)
	if menu == "" {
		t.Fatalf("the page has no Share menu %s", id)
	}
	return menu
}

// The header's Share button opens a menu of every form the task is shared
// in, each control drawn only with scripting, over the address itself as a
// plain link for a reader without one.
func TestTheTaskScreenHasAShareMenu(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "Fix [the] build")
	page := b.page("/tasks/" + ref)
	shown := strings.ToUpper(ref)

	head := header(t, page)
	if !strings.Contains(head, `class="secondary small share-open" popovertarget="share-`+ref+`"`) ||
		!strings.Contains(head, `aria-label="Share `+shown+`"`) {
		t.Errorf("the header has no Share button named for the task:\n%s", head)
	}
	menu := shareMenu(t, page, "share-"+ref)
	for _, want := range []string{
		`data-share-task data-share-ref="` + shown + `" data-share-title="Fix [the] build" data-share-path="/tasks/` + ref + `"`,
		`data-share="native" data-needs-js hidden>Share…</button>`,
		`data-share="link" data-needs-js>Copy link</button>`,
		`data-share="markdown" data-needs-js>Copy as Markdown</button>`,
		`data-share="text" data-needs-js>Copy key and title</button>`,
		`<a href="` + f.server.URL + `/tasks/` + ref + `" class="mono" data-share-url>` + f.server.URL + `/tasks/` + ref + `</a>`,
		`href="/docs/web-ui.md?section=sharing-a-task"`,
		`popovertarget="share-` + ref + `" popovertargetaction="hide"`,
	} {
		if !strings.Contains(menu, want) {
			t.Errorf("the Share menu lacks %s:\n%s", want, menu)
		}
	}
	if strings.Contains(strings.ToLower(menu), "tenant") {
		t.Errorf("the shared address names a tenant:\n%s", menu)
	}
}

// The link names no tenant, and needs none: another member of the tenant
// opens it as it is, and a tenant added to it would be refused.
func TestASharedLinkOpensForAnotherMemberOfTheTenant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "shared with the team")
	if page := f.as("viewer").page("/tasks/" + ref); !strings.Contains(page, "shared with the team") {
		t.Fatalf("another member of the tenant does not see the task the link names")
	}
	resp := f.as("viewer").get("/tasks/" + ref + "?tenant=acme")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a link naming its tenant answered %d, want %d: the Share menu must not add one", resp.StatusCode, http.StatusForbidden)
	}
}

// In the task dialog the menu and its button carry the dialog's prefix, so
// they never collide with the page under it.
func TestTheTaskDialogHasItsOwnShareMenu(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "in a dialog")
	resp := b.getDialog("/tasks/" + ref)
	page := readAll(t, resp)
	_ = resp.Body.Close()
	if !strings.Contains(page, `popovertarget="dlg-share-`+ref+`"`) {
		t.Errorf("the dialog's header has no Share button of its own")
	}
	shareMenu(t, page, "dlg-share-"+ref)
}

// A lane card's more actions menu offers the same forms beside its Copy link.
func TestALaneCardsMenuSharesTheTask(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "on a card")
	b.chooseLanes("/tasks")
	menu := moreMenu(t, laneCard(t, b.page("/tasks"), ref), ref)
	item := `data-share-task data-share-ref="` + strings.ToUpper(ref) + `" data-share-title="on a card" data-share-path="/tasks/` + ref + `" data-needs-js`
	for _, kind := range []string{"native", "markdown", "text"} {
		if !strings.Contains(menu, `data-share="`+kind+`" `+item) {
			t.Errorf("the card's menu has no %s share item:\n%s", kind, menu)
		}
	}
}
