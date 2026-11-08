// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// expanderAllowed are the only <details> elements the templates may hold, by
// template and class, each with why it is a toggle or a menu rather than a
// page section. A form that makes a record, a page's settings or a step that
// cannot be undone is never one: it is a button opening a dialog
// (actiondlg.go), and an explanation is visible text or an (i) tip.
var expanderAllowed = map[string]string{
	"partials.html field-info":     "the (i) beside a field: a sentence about one control, opened where it is read",
	"partials.html field-info tip": "the (i) tip beside a heading, the same pattern",
	"board.html card-move":         "a card's Move menu, one card's control",
	"tasks.html card-move":         "a card's Move menu on the task board, one card's control",
	"tasks.html cform-more":        "the rarely used fields inside Add with details, a part of a form already open",
	"task.html titleedit":          "Rename, an edit switch on the title it edits",
	"task.html descedit":           "Edit, an edit switch on the description it edits",
	"release.html relentry":        "one version's notes in the release history",
	"syncruns.html note":           "one run's warnings, a detail of its table row",
}

var detailsTag = regexp.MustCompile(`<details(?:\s+class="([^"]*)")?`)

// TestNoPageActionOrSectionSitsInAnExpander keeps the folds from coming back:
// every <details> in every template is one of the allowed toggles, and the
// stylesheet no longer carries the "+" disclosure they were drawn with.
func TestNoPageActionOrSectionSitsInAnExpander(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("templates/*.html")
	if err != nil || len(files) < 20 {
		t.Fatalf("reading the templates: %v (%d files)", err, len(files))
	}
	seen := map[string]bool{}
	for _, file := range files {
		src, err := os.ReadFile(file) // #nosec G304 -- a template of this package
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range detailsTag.FindAllStringSubmatch(string(src), -1) {
			key := filepath.Base(file) + " " + m[1]
			if _, ok := expanderAllowed[key]; !ok {
				t.Errorf("%s holds a <details class=%q>: a page action or section is a button and a dialog, an explanation is text or an (i) tip; see AGENTS.md", filepath.Base(file), m[1])
			}
			seen[key] = true
		}
	}
	for key := range expanderAllowed {
		if !seen[key] {
			t.Errorf("the allowlist names %q, which no template holds any more", key)
		}
	}
	sheet := readSheet(t)
	for _, gone := range []string{"details.panel.addmore", `content: "+"`, `content: "\00d7"`} {
		if strings.Contains(sheet, gone) {
			t.Errorf("the stylesheet still carries %s, the drawing of the add disclosure", gone)
		}
	}
}

// dialogTag is the opening tag of one action dialog.
func dialogTag(t *testing.T, page, id string) string {
	t.Helper()
	block := dialogBlock(page, id)
	if block == "" {
		t.Fatalf("the page has no dialog %q", id)
	}
	tag, _, _ := strings.Cut(block, ">")
	return tag
}

// createDialogs are the screens whose one action makes a record: each has a
// button in its header, or beside its section's heading, opening a dialog
// that holds the create form.
var createDialogs = []struct {
	name, path, id, action, head string
}{
	{"token", "/admin/tokens", "token-new", "/admin/tokens", `<div class="pagehead-tools">`},
	{"user", "/admin/users", "user-new", "/admin/users", `<div class="pagehead-tools">`},
	{"domain", "/admin/domains", "domain-new", "/admin/domains", `<div class="pagehead-tools">`},
	{"ssh key", "/admin/ssh-keys", "key-new", "/admin/ssh-keys", `<div class="pagehead-tools">`},
	{"webhook", "/admin/webhooks", "webhook-new", "/admin/webhooks", `<div class="pagehead-tools">`},
	{"member", "/admin/tenant", "member-new", "/admin/tenant/members", `<div class="sechead">`},
	{"channel", "/settings", "channel-new", "/settings/notifications", `<div class="sechead">`},
	{"field", "/projects/infra/fields", "field-new", "/projects/infra/fields", `<div class="pagehead-tools">`},
	{"sync source", "/sync", "source-new", "/sync/sources", `<div class="sechead">`},
}

func TestEveryCreateButtonOpensItsFormInADialog(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, tc := range createDialogs {
		page := b.page(tc.path)
		opener := `href="` + tc.path + `?dialog=` + tc.id + `#` + tc.id + `" data-dlg-open="` + tc.id + `" aria-haspopup="dialog"><svg class="icon" aria-hidden="true" width="16" height="16"><use href="/assets/icons.svg#plus"/>`
		at, from := strings.Index(page, opener), strings.Index(page, tc.head)
		if at < 0 || from < 0 || from > at ||
			strings.Contains(page[from:at], "<table") || strings.Contains(page[from:at], "<dialog") ||
			strings.Contains(page[from:at], `class="peoplelist`) || strings.Contains(page[from:at], `class="reclist`) {
			t.Errorf("%s: the header holds no plus button opening %s", tc.name, tc.id)
		}
		if tag := dialogTag(t, page, tc.id); strings.Contains(tag, " open") {
			t.Errorf("%s: the dialog is drawn open before it is asked for: %s", tc.name, tag)
		}
		dlg := dialogBlock(page, tc.id)
		if !strings.Contains(dlg, `<form method="post"`) || !strings.Contains(dlg, `action="`+tc.action+`"`) {
			t.Errorf("%s: the dialog does not hold the form posting to %s:\n%s", tc.name, tc.action, dlg)
		}
		if !strings.Contains(dlg, `data-dlg-close>Cancel</a>`) {
			t.Errorf("%s: the dialog has no Cancel:\n%s", tc.name, dlg)
		}
	}
}

// With scripting off the button is a link, and the screen it leads to draws
// the same form in place, open.
func TestACreateDialogIsAPageWithoutScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, tc := range createDialogs {
		page := b.page(tc.path + "?dialog=" + tc.id)
		tag := dialogTag(t, page, tc.id)
		if !strings.HasSuffix(tag, " open") || !strings.Contains(tag, "is-inplace") {
			t.Errorf("%s: the fallback page does not draw the dialog open in place: %s", tc.name, tag)
		}
		if !strings.Contains(dialogBlock(page, tc.id), `action="`+tc.action+`"`) {
			t.Errorf("%s: the fallback page does not hold the form", tc.name)
		}
		if !strings.Contains(dialogBlock(page, tc.id), `href="`+tc.path+`" data-dlg-close>Cancel</a>`) {
			t.Errorf("%s: Cancel does not lead back to the screen without the dialog", tc.name)
		}
	}
}

// A submission the service refuses comes back to the screen it was made on,
// with its dialog open, the message inside it and what was typed kept.
func TestARefusedCreateComesBackWithItsDialogOpen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, tc := range []struct {
		name, post, id, typed string
		form                  url.Values
	}{
		{"token", "/admin/tokens", "token-new", "", url.Values{"name": {"no scopes"}, "expires": {"90d"}}},
		{"user", "/admin/users", "user-new", `value="Kept Name"`, url.Values{"email": {""}, "display_name": {"Kept Name"}, "role": {"member"}}},
		{"domain", "/admin/domains", "domain-new", `value="bad host!"`, url.Values{"hostname": {"bad host!"}, "cert_mode": {"none"}}},
		{"ssh key", "/admin/ssh-keys", "key-new", `value="laptop"`, url.Values{"public_key": {"not a key"}, "label": {"laptop"}}},
		{"webhook", "/admin/webhooks", "webhook-new", `value="ftp://example.com/x"`, url.Values{"url": {"ftp://example.com/x"}}},
		{"member", "/admin/tenant/members", "member-new", "", url.Values{"actor_id": {""}, "role": {"member"}}},
		{"field", "/projects/infra/fields", "field-new", `value="size"`, url.Values{"key": {"size"}, "type": {"nonsense"}}},
		{"sync source", "/sync/sources", "source-new", `value="upstream"`, url.Values{"name": {"upstream"}, "system": {"nonsense"}}},
		{"sync settings", "/sync/sources", "source-new", `value="ops.yaml"`, url.Values{"name": {"ops"},
			"system": {"generic"}, "mapping_path": {"ops.yaml"}, "url": {"https://jira.example"}}},
		{"sync page size", "/sync/sources", "source-new", `value="https://jira.example"`, url.Values{"name": {"ops"},
			"system": {"jira"}, "url": {"https://jira.example"}, "page_size": {"0"}}},
	} {
		resp := b.post(tc.post, tc.form)
		page := body(t, resp)
		if resp.StatusCode < 400 || resp.StatusCode >= 500 {
			t.Errorf("%s: a refused create answered %d", tc.name, resp.StatusCode)
			continue
		}
		if tag := dialogTag(t, page, tc.id); !strings.HasSuffix(tag, " open") {
			t.Errorf("%s: the refusal does not come back with the dialog open: %s", tc.name, tag)
		}
		dlg := dialogBlock(page, tc.id)
		if !strings.Contains(dlg, `class="field-error"`) {
			t.Errorf("%s: the refusal is not inside the dialog:\n%s", tc.name, dlg)
		}
		if tc.typed != "" && !strings.Contains(dlg, tc.typed) {
			t.Errorf("%s: the dialog lost what was typed (%s):\n%s", tc.name, tc.typed, dlg)
		}
	}
}

// The project's settings are their own screen, reached from the board's
// header, with every section the board used to fold under its columns drawn
// as a heading and its form, the danger zone last.
func TestTheProjectSettingsScreenHoldsEveryFormerBoardSection(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	board := b.page("/projects/infra")
	head := between(t, board, `<div class="pagehead-tools">`, "</div>")
	if !strings.Contains(head, `href="/projects/infra/settings"`) || !strings.Contains(head, "icons.svg#settings") {
		t.Errorf("the board's header has no Settings button:\n%s", head)
	}
	for _, gone := range []string{"/projects/infra/update", "/projects/infra/lease", "/projects/infra/housekeeping",
		"/projects/infra/archive", "<details class=\"panel"} {
		if strings.Contains(contentOf(t, board), gone) {
			t.Errorf("the board still carries %s", gone)
		}
	}

	page := b.page("/projects/infra/settings")
	var last int
	for _, s := range []struct{ id, action string }{
		{"project", "/projects/infra/update"}, {"claims", "/projects/infra/lease"},
		{"housekeeping", "/projects/infra/housekeeping"}, {"danger", ""},
	} {
		at := strings.Index(page, `id="`+s.id+`" aria-labelledby="projset-`+s.id+`"`)
		if at < last {
			t.Errorf("the %s section is missing or out of order", s.id)
			continue
		}
		last = at
		section := between(t, page[at:], ">", "</section>")
		if !strings.Contains(section, `<h2 id="projset-`+s.id+`">`) {
			t.Errorf("the %s section has no heading", s.id)
		}
		if s.action != "" && !strings.Contains(section, `action="`+s.action+`"`) {
			t.Errorf("the %s section does not hold its form", s.id)
		}
	}
	danger := between(t, page, `id="danger"`, "</section>")
	if !strings.Contains(danger, `data-dlg-open="archive"`) || strings.Contains(danger, "/projects/infra/archive") {
		t.Errorf("archiving is not behind its dialog:\n%s", danger)
	}
	if !strings.Contains(formAt(t, dialogBlock(page, "archive"), "/projects/infra/archive"), "Archive project") {
		t.Error("the archive dialog does not hold the archive form")
	}
	if !strings.Contains(danger, `class="btn danger" href="/projects/infra/delete"`) {
		t.Errorf("deleting is not a red button leading to its confirmation:\n%s", danger)
	}
	if folds := strings.Count(page, "<details") - strings.Count(page, `<details class="field-info`); folds != 0 {
		t.Errorf("the settings screen folds %d sections", folds)
	}
}

// A step that cannot be undone is a red button, and only the dialog it opens
// carries the form that takes it.
func TestDestructiveActionsAreARedButtonAndAConfirmDialog(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "release notes")
	made := b.post("/tasks/"+ref+"/template", url.Values{"name": {"Release"}, "scope": {"tenant"}})
	_ = made.Body.Close()
	tpl, err := f.svc.GetTemplate(f.ctx(), "release")
	if err != nil {
		t.Fatal(err)
	}
	channel, _ := addChannel(t, f, b, url.Values{"name": {"hook"}, "url": {"https://hooks.invalid/hook"}, "kinds": {"mention"}, "active": {"on"}})

	for _, tc := range []struct{ path, id, action string }{
		{"/templates/" + tpl.ID, "template-delete", "/templates/" + tpl.ID + "/delete"},
		{channel, "channel-remove", channel + "/delete"},
	} {
		page := b.page(tc.path)
		dlg := dialogBlock(page, tc.id)
		outside := strings.Replace(page, dlg, "", 1)
		if dlg == "" || strings.Contains(outside, `action="`+tc.action+`"`) {
			t.Errorf("%s: the destructive form is not only in its dialog", tc.path)
			continue
		}
		if !strings.Contains(outside, `class="btn danger" href="`+tc.path+`?dialog=`+tc.id+`#`+tc.id+`" data-dlg-open="`+tc.id+`"`) {
			t.Errorf("%s: there is no red button opening %s", tc.path, tc.id)
		}
		if !strings.Contains(formAt(t, dlg, tc.action), `<button class="danger" type="submit">`) {
			t.Errorf("%s: the dialog's own button is not the red one", tc.path)
		}
	}
	if resp := b.get("/templates/" + tpl.ID); resp.StatusCode != http.StatusOK {
		t.Fatalf("reading the template = %d", resp.StatusCode)
	}
}
