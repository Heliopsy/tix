// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestChecklistEndpoint(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("with a list")
	f.createTask("without one")
	body := "- [ ] one\n- [ ] two\n"
	path := strings.Replace(wire.RouteTask, "{ref}", task.Ref, 1)
	updated := f.call(http.MethodPatch, path, core.UpdateTaskInput{Body: &body})
	var read core.Task
	decodeBody(t, updated, &read)
	if read.Checklist == nil || *read.Checklist != (core.Checklist{Done: 0, Total: 2}) {
		t.Fatalf("an update answers checklist %+v, want 0/2", read.Checklist)
	}

	check := strings.Replace(wire.RouteTaskChecklist, "{ref}", task.Ref, 1)
	ticked := f.call(http.MethodPost, check, core.CheckItemInput{Item: 2, Done: true, Version: read.Version})
	var got core.Task
	decodeBody(t, ticked, &got)
	if got.Body != "- [ ] one\n- [x] two\n" || got.Checklist == nil || got.Checklist.Done != 1 {
		t.Fatalf("after ticking item 2: body %q checklist %+v", got.Body, got.Checklist)
	}

	stale := f.call(http.MethodPost, check, core.CheckItemInput{Item: 1, Done: true, Version: read.Version})
	mustStatus(t, stale, http.StatusConflict)
	_ = stale.Body.Close()
	missing := f.call(http.MethodPost, check, core.CheckItemInput{Item: 3, Done: true})
	mustStatus(t, missing, http.StatusBadRequest)
	_ = missing.Body.Close()

	lists := []struct {
		query string
		want  []string
	}{
		{"?checklist=true", []string{"with a list"}},
		{"?checklist=false", []string{"without one"}},
		{"?checklist_open=true", []string{"with a list"}},
	}
	for _, l := range lists {
		if titles := f.listTitles(l.query); !slices.Equal(titles, l.want) {
			t.Errorf("list %q = %v, want %v", l.query, titles, l.want)
		}
	}
}
