package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// seedCustomFields defines the fields the custom field filter tests select on.
func (f *apiFixture) seedCustomFields() {
	f.t.Helper()
	defs := []core.FieldDefInput{
		{Key: "severity", Type: core.FieldString},
		{Key: "team", Type: core.FieldString},
		{Key: "points", Type: core.FieldInt},
	}
	for _, in := range defs {
		resp := f.call(http.MethodPut, "/api/v1/projects/infra/fields", in)
		mustStatus(f.t, resp, http.StatusOK)
		_ = resp.Body.Close()
	}
}

// createTaskWithFields creates a task carrying custom field values.
func (f *apiFixture) createTaskWithFields(title string, fields map[string]any) core.Task {
	f.t.Helper()
	resp := f.call(http.MethodPost, wire.RouteTasks,
		core.CreateTaskInput{ProjectRef: "infra", Title: title, CustomFields: fields})
	defer func() { _ = resp.Body.Close() }()
	mustStatus(f.t, resp, http.StatusCreated)
	var task core.Task
	decodeBody(f.t, resp, &task)
	return task
}

// listTitles lists tasks through the API and returns their titles in title
// order. The ordering is requested explicitly: the default is by urgency, and
// tasks that share a priority and carry no due date tie there, leaving the
// order to the identifier tiebreak. Asserting on that would test the shape of
// generated identifiers rather than the filter.
func (f *apiFixture) listTitles(query string) []string {
	f.t.Helper()
	sep := "&"
	if query == "" {
		sep = "?"
	}
	query += sep + "sort=" + core.SortTitle + "&direction=" + string(core.Ascending)
	resp := f.call(http.MethodGet, wire.RouteTasks+query, nil)
	defer func() { _ = resp.Body.Close() }()
	mustStatus(f.t, resp, http.StatusOK)
	var body struct {
		Items []core.Task `json:"items"`
	}
	decodeBody(f.t, resp, &body)
	titles := make([]string, 0, len(body.Items))
	for _, task := range body.Items {
		titles = append(titles, task.Title)
	}
	return titles
}

func TestTaskListFiltersByCustomField(t *testing.T) {
	f := newFixture(t)
	f.seedCustomFields()
	f.createTaskWithFields("outage", map[string]any{"severity": "high", "team": "infra", "points": 3})
	f.createTaskWithFields("paperwork", map[string]any{"severity": "low", "team": "infra", "points": 1})
	f.createTaskWithFields("audit", map[string]any{"severity": "high", "team": "sec", "points": 3})

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"one field", "?field.severity=high", []string{"audit", "outage"}},
		{"two fields are an and", "?field.severity=high&field.team=infra", []string{"outage"}},
		{"a number filters a number", "?field.points=3", []string{"audit", "outage"}},
		{"a quoted number filters a string", `?field.points="3"`, []string{}},
		{"no match is an empty page", "?field.severity=catastrophic", []string{}},
		{"a typed object is equivalent", `?custom_fields={"severity":"low"}`, []string{"paperwork"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.listTitles(tc.query); !slices.Equal(got, tc.want) {
				t.Fatalf("titles = %v, want %v (in title order)", got, tc.want)
			}
		})
	}
}

func TestCustomFieldFilterRejectsMalformedInput(t *testing.T) {
	f := newFixture(t)
	f.seedCustomFields()

	cases := []struct {
		name  string
		query string
	}{
		{"space in the key", "?field.bad+key=1"},
		{"quote in the key", `?field.a'b=1`},
		{"empty key", "?field.=1"},
		{"repeated key", "?field.severity=high&field.severity=low"},
		{"null value", "?field.severity=null"},
		{"array value", "?field.severity=[1,2]"},
		{"object is not an object", "?custom_fields=nonsense"},
		{"object holds a nested value", `?custom_fields={"severity":{"x":1}}`},
		{"object holds a bad key", `?custom_fields={"bad%20key":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(http.MethodGet, wire.RouteTasks+tc.query, nil)
			defer func() { _ = resp.Body.Close() }()
			mustStatus(t, resp, http.StatusBadRequest)
			var body wire.ErrorEnvelope
			decodeBody(t, resp, &body)
			if body.Error.Code != core.KindInvalid {
				t.Fatalf("error code = %q, want %q", body.Error.Code, core.KindInvalid)
			}
		})
	}
}

func TestCustomFieldFilterSurvivesTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	f.seedCustomFields()
	f.createTaskWithFields("outage", map[string]any{"severity": "high", "points": 3})
	f.createTaskWithFields("paperwork", map[string]any{"severity": "low", "points": 1})

	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	ctx := context.Background()

	cases := []struct {
		name   string
		filter core.TaskFilter
		want   string
	}{
		{"string field", core.TaskFilter{CustomFields: map[string]any{"severity": "high"}}, "outage"},
		{"numeric field", core.TaskFilter{CustomFields: map[string]any{"points": 1}}, "paperwork"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := remote.ListTasks(ctx, tc.filter)
			if err != nil {
				t.Fatalf("listing: %v", err)
			}
			if len(page.Tasks) != 1 || page.Tasks[0].Title != tc.want {
				t.Fatalf("tasks = %+v, want only %q", page.Tasks, tc.want)
			}
		})
	}

	empty, err := remote.ListTasks(ctx, core.TaskFilter{CustomFields: map[string]any{"severity": "none"}})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(empty.Tasks) != 0 {
		t.Fatalf("tasks = %+v, want none", empty.Tasks)
	}

	if _, err := remote.ListTasks(ctx, core.TaskFilter{CustomFields: map[string]any{"bad key": 1}}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("error = %v, want an invalid error", err)
	}
}
