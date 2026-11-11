// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestMoveToIsOfferedToWritersOnly(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "move me")
	link := `href="/tasks/` + ref + `/move" data-dialog-load`

	if bar := actionsMenu(t, f.as("alice").page("/tasks/"+ref), ref); !strings.Contains(bar, link) {
		t.Errorf("a writer is not offered Move to in the actions menu:\n%s", bar)
	}
	if page := f.as("viewer").page("/tasks/" + ref); strings.Contains(page, "/tasks/"+ref+"/move") {
		t.Errorf("a read-only reader is offered Move to")
	}
}

func TestTheMoveFormOffersOnlyOtherProjectsAndCarriesTheVersion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")
	task, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)})
	if err != nil {
		t.Fatal(err)
	}

	form := formAt(t, b.page("/tasks/"+ref+"/move"), "/tasks/"+ref+"/move")
	if !strings.Contains(form, `<option value="web" selected>`) {
		t.Errorf("the other project is not offered and selected:\n%s", form)
	}
	if strings.Contains(form, `<option value="infra"`) {
		t.Errorf("the task's own project is offered:\n%s", form)
	}
	if !strings.Contains(form, `name="version" value="`+strconv.Itoa(task.Version)+`"`) {
		t.Errorf("the form does not carry version %d:\n%s", task.Version, form)
	}
}

func TestMovingOpensTheTaskUnderItsNewReference(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")

	resp := b.post("/tasks/"+ref+"/move", url.Values{"project_ref": {"web"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(loc.Path, "/tasks/web-") || loc.Query().Get("flash") != "task moved" {
		t.Fatalf("redirect = %q, want the task under a web reference with a toast", loc)
	}
	if _, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("the old reference still resolves: %v", err)
	}
}

func TestARefusedMoveDrawsTheFormAgain(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	b := f.as("alice")
	ref := b.createTask("infra", "rotate keys")
	cases := []struct {
		name   string
		form   url.Values
		status int
		field  string
	}{
		{"unknown project", url.Values{"project_ref": {"nope"}}, http.StatusNotFound, "project_ref"},
		{"stale version", url.Values{"project_ref": {"web"}, "version": {"999"}}, http.StatusConflict, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := b.post("/tasks/"+ref+"/move", tc.form)
			page := body(t, resp)
			wantStatus(t, resp, tc.status)
			form := formAt(t, page, "/tasks/"+ref+"/move")
			if !strings.Contains(form, `class="field-error"`) {
				t.Errorf("the refusal is not drawn in the form:\n%s", form)
			}
			if tc.field != "" && !strings.Contains(form, `id="`+tc.field+`-error"`) {
				t.Errorf("the message is not under %s:\n%s", tc.field, form)
			}
		})
	}
	if _, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)}); err != nil {
		t.Errorf("a refused move moved the task: %v", err)
	}
}
