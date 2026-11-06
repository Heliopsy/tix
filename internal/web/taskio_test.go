// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestTheBoardOffersExportToReadersWhoMayExport(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	head := between(t, f.as("alice").page("/projects/infra"), `<div class="pagehead-tools">`, "</div>")
	if !strings.Contains(head, `data-dlg-open="export"`) || !strings.Contains(head, "icons.svg#download") {
		t.Errorf("the board's header has no Export button:\n%s", head)
	}
	if page := f.as("viewer").page("/projects/infra"); strings.Contains(page, `data-dlg-open="export"`) ||
		dialogBlock(page, "export") != "" {
		t.Errorf("a viewer, who holds no export scope, is offered Export")
	}
}

func TestTheExportDialogIsAFormWithEveryChoice(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	dlg := dialogBlock(f.as("alice").page("/projects/infra"), "export")
	if dlg == "" {
		t.Fatal("the board draws no export dialog")
	}
	form := formAt(t, dlg, "/projects/infra/tasks/export")
	for _, want := range []string{
		`name="format"`, `<option value="markdown"`, `name="dates"`, `name="delimiter"`, `name="q"`,
		`name="finished" value="1" checked`, `name="subtasks" value="1" checked`, `name="trashed" value="1">`,
		`name="column" value="ref" checked`, `name="column" value="history">`,
		`data-colmove="up"`, `data-colmove="down"`, `data-dlg-close`, `type="submit"`,
	} {
		if !strings.Contains(form, want) {
			t.Errorf("the export form lacks %s:\n%s", want, form)
		}
	}
	if strings.Contains(dlg, "<details") {
		t.Errorf("the export dialog hides a choice in an expander:\n%s", dlg)
	}
}

func TestExportDownloadsTheChosenColumnsInOrder(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "=1+1")
	resp := b.post("/projects/infra/tasks/export", url.Values{
		"format": {"csv"}, "column": {"title", "ref"}, "finished": {"1"}, "subtasks": {"1"},
		"dates": {"iso"}, "delimiter": {";"},
	})
	got := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, got)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != `attachment; filename="infra-tasks.csv"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if lines[0] != "title;ref" || !strings.HasPrefix(lines[1], "'=1+1;") {
		t.Errorf("download =\n%s", got)
	}
}

func TestARefusedExportComesBackInTheDialog(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").post("/projects/infra/tasks/export", url.Values{
		"format": {"json"}, "q": {"priority:urgentest"}, "column": {"title"},
	})
	page := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
	dlg := dialogBlock(page, "export")
	if !strings.Contains(dlg, " open>") && !strings.Contains(dlg, " open ") {
		t.Errorf("the dialog is not drawn open after a refusal:\n%s", dlg)
	}
	if !strings.Contains(dlg, `value="priority:urgentest"`) || !strings.Contains(dlg, `<option value="json" selected>`) {
		t.Errorf("the refused choices are not kept:\n%s", dlg)
	}
	if q := between(t, dlg, `id="export-q"`, "</div>"); !strings.Contains(q, `class="field-error"`) {
		t.Errorf("the refusal is not shown in the dialog:\n%s", dlg)
	}
}
