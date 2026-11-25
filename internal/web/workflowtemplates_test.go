// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// templateOption is one template's choice in a picker, from its radio to the
// end of its label, so an assertion about its order reads that one choice.
func templateOption(t *testing.T, page, value string) string {
	t.Helper()
	opt := between(t, page, `value="`+value+`"`, "</label>")
	if opt == "" {
		t.Fatalf("the picker offers no %s:\n%s", value, page)
	}
	return opt
}

// inOrder fails unless every token appears in the text, each after the last.
func inOrder(t *testing.T, where, text string, tokens ...string) {
	t.Helper()
	at := -1
	for _, tok := range tokens {
		i := strings.Index(text, tok)
		if i < 0 {
			t.Fatalf("%s has no %q:\n%s", where, tok, text)
		}
		if i <= at {
			t.Fatalf("%s draws %q before what should precede it:\n%s", where, tok, text)
		}
		at = i
	}
}

func TestTheNewProjectPickerLeadsWithEachTemplatesPurpose(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/projects")
	form := formAt(t, page, "/projects\" enctype=\"multipart/form-data")

	if !strings.Contains(form, `name="workflow_key" value="default" checked`) {
		t.Errorf("the picker does not start on the default workflow:\n%s", form)
	}
	opt := templateOption(t, form, "template:tix-sales")
	inOrder(t, "the Sales template's choice", opt,
		`class="wfpick-purpose">Track deals from first contact to signed.`,
		`class="wfpick-name">Sales or BizDev pipeline`,
		`wfpick-builtin">Built-in`,
		`class="wfpick-states">Lead → Contacted`)
	if strings.Contains(opt, "tix-sales</") {
		t.Errorf("the template's name carries its key prefix:\n%s", opt)
	}
	if strings.Count(form, `value="template:tix-`) != 12 {
		t.Errorf("the picker offers %d templates, want 12", strings.Count(form, `value="template:tix-`))
	}
}

func TestANewProjectStartsFromATemplateWithoutScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post("/projects", url.Values{"key": {"deals"}, "name": {"Deals"}, "workflow_key": {"template:tix-sales"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	fields := b.page("/projects/deals/fields")
	for _, want := range []string{"company", "deal_value", "expected_close"} {
		if !strings.Contains(fields, want) {
			t.Errorf("the project made from the template has no %s field", want)
		}
	}
	if wf := b.page("/workflows"); !strings.Contains(wf, `href="/workflows/tix-sales"`) {
		t.Errorf("creating the project did not add the template's workflow:\n%s", wf)
	}

	refused := b.post("/projects", url.Values{"key": {"more"}, "name": {"More"}, "workflow_key": {"template:tix-nope"}})
	defer func() { _ = refused.Body.Close() }()
	if refused.StatusCode != http.StatusNotFound {
		t.Fatalf("an unknown template answered %d, want the form back with 404", refused.StatusCode)
	}
	body := readAll(t, refused)
	if !strings.Contains(body, `value="template:tix-kanban"`) || !strings.Contains(body, "tix-nope") {
		t.Errorf("the refusal does not redraw the picker with the reason:\n%s", body)
	}
}

func TestTheWorkflowsScreenAddsAWorkflowFromATemplate(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	list := b.page("/workflows")
	if !strings.Contains(list, `href="/workflows?dialog=workflow-template#workflow-template"`) {
		t.Fatalf("the Workflows screen has no Add from template link for a browser without scripting:\n%s", list)
	}
	open := b.page("/workflows?dialog=workflow-template")
	if !strings.Contains(open, `id="workflow-template"`) || !strings.Contains(between(t, open, `id="workflow-template"`, ">"), " open") {
		t.Fatalf("the dialog is not drawn open in place:\n%s", open)
	}
	form := formAt(t, open, "/workflow-templates/apply")
	inOrder(t, "the bug triage choice", templateOption(t, form, "tix-bugs"),
		`class="wfpick-purpose">Confirm bugs before anyone fixes them.`, `class="wfpick-name">Bug triage`)

	resp := b.post("/workflow-templates/apply", url.Values{"template": {"tix-bugs"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/workflows/tix-bugs") {
		t.Fatalf("applying went to %q, want the new workflow", loc)
	}

	again := b.post("/workflow-templates/apply", url.Values{"template": {"tix-bugs"}})
	defer func() { _ = again.Body.Close() }()
	if again.StatusCode != http.StatusConflict {
		t.Fatalf("applying onto a taken key answered %d, want 409", again.StatusCode)
	}
	if body := readAll(t, again); !strings.Contains(body, "already exists") || !strings.Contains(body, `action="/workflow-templates/apply"`) {
		t.Errorf("the refusal does not come back in the dialog:\n%s", body)
	}

	renamed := b.post("/workflow-templates/apply", url.Values{"template": {"tix-bugs"}, "on_collision": {"rename"}})
	defer func() { _ = renamed.Body.Close() }()
	wantStatus(t, renamed, http.StatusSeeOther)
	if loc := renamed.Header.Get("Location"); !strings.HasPrefix(loc, "/workflows/tix-bugs-2") {
		t.Fatalf("rename went to %q", loc)
	}
}

func TestTemplatesAreNamedInBulgarian(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie("tix_lang", "bg")
	opt := templateOption(t, b.page("/projects"), "template:tix-kanban")
	inOrder(t, "the Bulgarian kanban choice", opt,
		"Местете картите отляво надясно, нищо повече.", "Обикновен канбан", "Вграден", "Натрупани → За правене")
}
