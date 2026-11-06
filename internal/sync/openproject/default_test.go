// SPDX-License-Identifier: AGPL-3.0-or-later

package openproject

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// fixtureServer answers the recorded two-page work package collection.
func fixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := os.ReadFile(filepath.Join("testdata", "work_packages_"+r.URL.Query().Get("offset")+".json"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/hal+json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func defaultMappingFor(t *testing.T, project string) *extsync.Mapping {
	t.Helper()
	raw := strings.Replace(string(DefaultMapping()), "project: CHANGE-ME", "project: "+project, 1)
	m, err := extsync.ParseMapping([]byte(raw))
	if err != nil {
		t.Fatalf("ParseMapping(default with project %q): %v", project, err)
	}
	return m
}

func TestDefaultMappingNeedsOnlyAProject(t *testing.T) {
	if !strings.Contains(string(DefaultMapping()), "\nproject: CHANGE-ME\n") {
		t.Fatal("the default mapping must carry the CHANGE-ME project placeholder")
	}
	m := defaultMappingFor(t, "ops")
	if m.Project != "ops" || m.Identity.ID != "id" || m.Identity.UpdatedAt != "updatedAt" {
		t.Errorf("project %q identity %+v", m.Project, m.Identity)
	}
	if m.Unmapped.Preserve == nil || *m.Unmapped.Preserve {
		t.Error("the default mapping must not preserve unnamed HAL links")
	}
	builtin := core.WorkflowDefinition{Initial: "todo", States: []core.State{
		{Key: "todo"}, {Key: "doing"}, {Key: "blocked"}, {Key: "done", Terminal: true}, {Key: "cancelled", Terminal: true},
	}}
	if err := m.ValidateAgainst(builtin); err != nil {
		t.Errorf("the default mapping does not fit the built-in workflow: %v", err)
	}
	mutated := DefaultMapping()
	mutated[0] = 'X'
	if DefaultMapping()[0] == 'X' {
		t.Error("DefaultMapping hands out the embedded bytes")
	}
}

func TestDefaultMappingTranslatesRecordedWorkPackages(t *testing.T) {
	srv := fixtureServer(t)
	m := defaultMappingFor(t, "ops")
	i := newImporter(t, srv, func(time.Duration) {}, "")

	byID := map[string]extsync.Mapped{}
	opt := extsync.Options{}
	for {
		batch, err := i.Fetch(context.Background(), opt)
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		for _, rec := range batch.Records {
			got, err := m.Apply(rec)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			byID[got.ExternalID] = got
		}
		if batch.Page == "" {
			break
		}
		opt.Page = batch.Page
	}
	if len(byID) != 4 {
		t.Fatalf("translated %d work packages, want both pages", len(byID))
	}

	tests := []struct {
		id, title, status, assignee, email, parent string
		priority                                   core.Priority
		tags                                       []string
	}{
		{"41", "Set up CI", "todo", "Jane Doe", "JANE@corp.test", "", core.PriorityNormal, []string{"task"}},
		{"42", "Write the operator guide", "doing", "J. Smith", "", "", core.PriorityHigh, []string{"feature"}},
		{"43", "Fix the sign-in loop", "done", "Nobody Known", "", "/api/v3/work_packages/42", core.PriorityHighest, []string{"bug"}},
		{"44", "Vendor lock-in", "todo", "", "", "", core.PriorityLow, nil},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			got := byID[tc.id]
			if got.Title != tc.title || got.Status != tc.status || got.Priority != tc.priority {
				t.Errorf("title %q status %q priority %v", got.Title, got.Status, got.Priority)
			}
			if got.Assignee != tc.assignee || got.AssigneeEmail != tc.email || got.ExternalParentID != tc.parent {
				t.Errorf("assignee %q <%s> parent %q", got.Assignee, got.AssigneeEmail, got.ExternalParentID)
			}
			if strings.Join(got.Tags, ",") != strings.Join(tc.tags, ",") {
				t.Errorf("tags = %v, want %v", got.Tags, tc.tags)
			}
			if got.CustomFields["op_project"] != "Demo project" {
				t.Errorf("custom fields = %v, want the project kept", got.CustomFields)
			}
			for key := range got.CustomFields {
				if strings.Contains(key, "links") || strings.Contains(key, "embedded") {
					t.Errorf("HAL plumbing %q was preserved", key)
				}
			}
		})
	}
	first := byID["41"]
	if first.Body != "Build and test on every push." || first.DueAt == nil || first.CustomFields["start_date"] == nil {
		t.Errorf("body %q due %v start %v", first.Body, first.DueAt, first.CustomFields["start_date"])
	}
	if first.ExternalURL != srv.URL+"/work_packages/41" {
		t.Errorf("url = %q", first.ExternalURL)
	}
	if byID["44"].CustomFields["op_type"] != "Risk" {
		t.Errorf("an unlisted type was dropped: %v", byID["44"].CustomFields)
	}
}
