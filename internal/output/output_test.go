package output

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"gopkg.in/yaml.v3"
)

var (
	refTime  = time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	refTime2 = time.Date(2026, 4, 5, 6, 7, 8, 0, time.UTC)
)

func ptrTime(t time.Time) *time.Time { return &t }

func sampleTask() core.Task {
	return core.Task{
		ID:              "tsk_1",
		TenantID:        "ten_1",
		ProjectID:       "prj_1",
		Seq:             7,
		Ref:             "ENG-7",
		Title:           "Wire the output package",
		Status:          "in_progress",
		Priority:        core.PriorityHigh,
		Tags:            []string{"cli", "output"},
		AssigneeActorID: "act_1",
		CreatorActorID:  "act_2",
		DueAt:           ptrTime(refTime2),
		CreatedAt:       refTime,
		UpdatedAt:       refTime,
		Version:         3,
	}
}

func sampleProject() core.Project {
	return core.Project{
		ID: "prj_1", TenantID: "ten_1", Key: "ENG", Name: "Engineering",
		WorkflowID: "wfl_1", CreatedAt: refTime, UpdatedAt: refTime,
	}
}

func sampleWorkflow() core.Workflow {
	return core.Workflow{
		ID: "wfl_1", TenantID: "ten_1", Key: "default", Name: "Default",
		Builtin: true, CreatedAt: refTime, UpdatedAt: refTime,
		Definition: core.WorkflowDefinition{
			Initial: "todo",
			States: []core.State{
				{Key: "todo", Label: "To Do"},
				{Key: "done", Label: "Done", Terminal: true},
			},
			Transitions: []core.Transition{{From: "todo", To: "done"}},
		},
	}
}

func sampleComment() core.Comment {
	return core.Comment{
		ID: "cmt_1", TenantID: "ten_1", TaskID: "tsk_1", AuthorActorID: "act_1",
		Body: "first line\nsecond line", CreatedAt: refTime, UpdatedAt: refTime,
	}
}

func sampleToken() core.APIToken {
	return core.APIToken{
		ID: "tok_1", TenantID: "ten_1", ActorID: "act_1", Name: "agent",
		Scopes:    []core.Scope{core.ScopeTaskRead, core.ScopeTaskClaim},
		CreatedAt: refTime, ExpiresAt: ptrTime(refTime2),
	}
}

func sampleAudit() core.AuditEntry {
	return core.AuditEntry{
		Seq: 42, TenantID: "ten_1", ActorID: "act_1", Action: "task.create",
		SubjectType: "task", SubjectID: "tsk_1", Source: core.SourceCLI, OccurredAt: refTime,
	}
}

func sampleEndpoint() core.WebhookEndpoint {
	return core.WebhookEndpoint{
		ID: "whk_1", TenantID: "ten_1", URL: "https://example.test/hook",
		Secret: "s3cr3t", EventTypes: []string{"task.created"}, Active: true, CreatedAt: refTime,
	}
}

func sampleTenant() core.Tenant {
	return core.Tenant{ID: "ten_1", Key: "acme", Name: "Acme", CreatedAt: refTime, UpdatedAt: refTime}
}

// utcStyle is the TimeStyle every test in this file renders table output
// through, so a golden string is the same regardless of the host machine's
// local timezone. Zone-specific behaviour has its own tests in timestyle_test.go.
func utcStyle(t *testing.T) TimeStyle {
	t.Helper()
	style, err := NewTimeStyle(TimeISO, "utc")
	if err != nil {
		t.Fatalf("building utc style: %v", err)
	}
	return style
}

func render(t *testing.T, format string, data any) string {
	t.Helper()
	var buf bytes.Buffer
	if err := NewWithStyle(format, ModeAuto, utcStyle(t)).Format(&buf, data); err != nil {
		t.Fatalf("format %s: %v", format, err)
	}
	return buf.String()
}

func TestNewSelectsFormatter(t *testing.T) {
	cases := []struct {
		format string
		want   reflect.Type
	}{
		{FormatJSON, reflect.TypeOf(&jsonFormatter{})},
		{FormatYAML, reflect.TypeOf(&yamlFormatter{})},
		{FormatTable, reflect.TypeOf(&tableFormatter{})},
		{"", reflect.TypeOf(&tableFormatter{})},
		{"xml", reflect.TypeOf(&tableFormatter{})},
	}
	for _, tc := range cases {
		if got := reflect.TypeOf(New(tc.format)); got != tc.want {
			t.Errorf("New(%q) = %v, want %v", tc.format, got, tc.want)
		}
	}
}

func TestAllTypesRenderInEveryFormat(t *testing.T) {
	values := map[string]any{
		"tasks":     []core.Task{sampleTask()},
		"task":      sampleTask(),
		"taskptr":   ptrTask(sampleTask()),
		"projects":  []core.Project{sampleProject()},
		"project":   sampleProject(),
		"workflows": []core.Workflow{sampleWorkflow()},
		"workflow":  sampleWorkflow(),
		"comments":  []core.Comment{sampleComment()},
		"comment":   sampleComment(),
		"tokens":    []core.APIToken{sampleToken()},
		"token":     sampleToken(),
		"audits":    []core.AuditEntry{sampleAudit()},
		"audit":     sampleAudit(),
		"endpoints": []core.WebhookEndpoint{sampleEndpoint()},
		"endpoint":  sampleEndpoint(),
		"tenants":   []core.Tenant{sampleTenant()},
		"tenant":    sampleTenant(),
		"taskpage":  core.TaskPage{Tasks: []core.Task{sampleTask()}, NextCursor: "abc"},
		"emptypage": core.TaskPage{},
	}
	for name, v := range values {
		for _, f := range []string{FormatTable, FormatJSON, FormatYAML} {
			out := render(t, f, v)
			if strings.TrimSpace(out) == "" {
				t.Errorf("%s/%s: empty output", name, f)
			}
			if !strings.HasSuffix(out, "\n") {
				t.Errorf("%s/%s: output not newline terminated: %q", name, f, out)
			}
			if strings.Contains(out, "\x1b[") {
				t.Errorf("%s/%s: output contains ANSI escapes", name, f)
			}
			if strings.Contains(out, "0001-01-01") {
				t.Errorf("%s/%s: output contains zero time", name, f)
			}
		}
	}
}

func ptrTask(t core.Task) *core.Task { return &t }

func TestTableTaskGolden(t *testing.T) {
	got := render(t, FormatTable, []core.Task{sampleTask()})
	want := "" +
		"┌───────┬─────────────────────────┬─────────────┬──────────┬──────────┬─────────────┬──────────────────┬──────────────────┬─────────┐\n" +
		"│ REF   │ TITLE                   │ STATUS      │ PRIORITY │ ASSIGNEE │ TAGS        │ DUE              │ UPDATED          │ BLOCKED │\n" +
		"├───────┼─────────────────────────┼─────────────┼──────────┼──────────┼─────────────┼──────────────────┼──────────────────┼─────────┤\n" +
		"│ ENG-7 │ Wire the output package │ in_progress │ high     │ act_1    │ cli, output │ 2026-04-05 06:07 │ 2026-03-04 05:06 │ no      │\n" +
		"└───────┴─────────────────────────┴─────────────┴──────────┴──────────┴─────────────┴──────────────────┴──────────────────┴─────────┘\n"
	if got != want {
		t.Errorf("task table mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestTableSecretNeverRendered(t *testing.T) {
	out := render(t, FormatTable, []core.WebhookEndpoint{sampleEndpoint()})
	if strings.Contains(out, "s3cr3t") {
		t.Error("webhook secret leaked into table output")
	}
	if strings.Contains(render(t, FormatJSON, sampleEndpoint()), "s3cr3t") {
		t.Error("webhook secret leaked into json output")
	}
}

func TestTableCommentBodyIsSingleLine(t *testing.T) {
	out := render(t, FormatTable, []core.Comment{sampleComment()})
	if strings.Contains(out, "second line\n") && strings.Count(out, "\n") > 5 {
		t.Errorf("comment body not flattened: %s", out)
	}
	if !strings.Contains(out, "first line") {
		t.Errorf("comment body missing: %s", out)
	}
}

func TestEmptySlicesRenderHeaders(t *testing.T) {
	cases := map[string]any{
		"tasks":     []core.Task{},
		"projects":  []core.Project{},
		"workflows": []core.Workflow{},
		"comments":  []core.Comment{},
		"tokens":    []core.APIToken{},
		"audits":    []core.AuditEntry{},
		"endpoints": []core.WebhookEndpoint{},
		"tenants":   []core.Tenant{},
	}
	for name, v := range cases {
		out := render(t, FormatTable, v)
		if strings.TrimSpace(out) == "" {
			t.Errorf("%s: empty table output", name)
		}
		if !strings.Contains(out, "│") {
			t.Errorf("%s: expected a header row, got %q", name, out)
		}
	}
	if got := render(t, FormatJSON, []core.Task{}); got != "[]\n" {
		t.Errorf("empty json = %q, want %q", got, "[]\n")
	}
}

func TestNilInput(t *testing.T) {
	if got := render(t, FormatTable, nil); got != "No results.\n" {
		t.Errorf("table nil = %q", got)
	}
	if got := render(t, FormatJSON, nil); got != "null\n" {
		t.Errorf("json nil = %q", got)
	}
	var tp *core.Task
	if got := render(t, FormatTable, tp); got != "No results.\n" {
		t.Errorf("table nil pointer = %q", got)
	}
	if got := render(t, FormatTable, []core.Task(nil)); !strings.Contains(got, "REF") {
		t.Errorf("table nil slice = %q", got)
	}
}

func TestUnknownTypesDoNotPanic(t *testing.T) {
	type custom struct {
		Alpha string
		Beta  int
	}
	cases := []any{
		custom{Alpha: "a", Beta: 1},
		[]custom{{Alpha: "a", Beta: 1}},
		[]custom{},
		42,
		"plain string",
		map[string]string{"k": "v"},
		[]string{"a", "b"},
		[]*custom{nil},
	}
	for _, v := range cases {
		out := render(t, FormatTable, v)
		if strings.TrimSpace(out) == "" {
			t.Errorf("%T: empty output", v)
		}
		if !strings.HasSuffix(out, "\n") {
			t.Errorf("%T: missing trailing newline", v)
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original := []core.Task{sampleTask()}
	out := render(t, FormatJSON, original)
	var back []core.Task
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(original, back) {
		t.Errorf("round trip mismatch:\n%#v\n%#v", original, back)
	}
	page := core.TaskPage{Tasks: original, NextCursor: "cur"}
	var backPage core.TaskPage
	if err := json.Unmarshal([]byte(render(t, FormatJSON, page)), &backPage); err != nil {
		t.Fatalf("unmarshal page: %v", err)
	}
	if !reflect.DeepEqual(page, backPage) {
		t.Errorf("page round trip mismatch")
	}
}

func TestJSONIsIndentedAndDeterministic(t *testing.T) {
	first := render(t, FormatJSON, sampleTask())
	second := render(t, FormatJSON, sampleTask())
	if first != second {
		t.Error("json output is not deterministic")
	}
	if !strings.Contains(first, "\n  \"id\": \"tsk_1\"") {
		t.Errorf("json not indented: %s", first)
	}
	if !strings.Contains(first, "2026-04-05T06:07:08Z") {
		t.Errorf("json timestamps not RFC3339: %s", first)
	}
}

func TestYAMLParsesBack(t *testing.T) {
	out := render(t, FormatYAML, sampleProject())
	var m map[string]any
	if err := yaml.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	if m["key"] != "ENG" {
		t.Errorf("yaml key = %v", m["key"])
	}
}

func TestFormatErrorsPropagate(t *testing.T) {
	bad := make(chan int)
	if err := New(FormatJSON).Format(&bytes.Buffer{}, bad); err == nil {
		t.Error("expected json marshal error")
	}
	if err := New(FormatYAML).Format(&bytes.Buffer{}, func() {}); err == nil {
		t.Error("expected yaml marshal error")
	}
	if err := New(FormatJSON).Format(failWriter{}, sampleTask()); err == nil {
		t.Error("expected json write error")
	}
	if err := New(FormatYAML).Format(failWriter{}, sampleTask()); err == nil {
		t.Error("expected yaml write error")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

func TestTimestampHelpers(t *testing.T) {
	cases := []struct {
		name        string
		in          *time.Time
		wantRFC     string
		wantCompact string
	}{
		{"nil", nil, "", ""},
		{"zero", ptrTime(time.Time{}), "", ""},
		{"value", ptrTime(refTime), "2026-03-04T05:06:07Z", "2026-03-04 05:06"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatTimestampPtr(tc.in); got != tc.wantRFC {
				t.Errorf("FormatTimestampPtr = %q, want %q", got, tc.wantRFC)
			}
			if got := FormatCompactPtr(tc.in); got != tc.wantCompact {
				t.Errorf("FormatCompactPtr = %q, want %q", got, tc.wantCompact)
			}
		})
	}
	if got := FormatTimestamp(time.Time{}); got != "" {
		t.Errorf("zero FormatTimestamp = %q", got)
	}
	if got := FormatCompact(refTime); got != "2026-03-04 05:06" {
		t.Errorf("FormatCompact = %q", got)
	}
}

func TestPriorityLabels(t *testing.T) {
	cases := map[core.Priority]string{
		core.PriorityHighest: "highest",
		core.PriorityHigh:    "high",
		core.PriorityNormal:  "normal",
		core.PriorityLow:     "low",
		core.PriorityLowest:  "lowest",
		core.Priority(9):     "9",
	}
	for p, want := range cases {
		if got := priorityLabel(p); got != want {
			t.Errorf("priorityLabel(%d) = %q, want %q", p, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 3); got != "ab…" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate("ab", 5); got != "ab" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("a\nb\tc", 10); got != "a b c" {
		t.Errorf("truncate whitespace = %q", got)
	}
}

func TestTaskPageRendersCursor(t *testing.T) {
	out := render(t, FormatTable, core.TaskPage{Tasks: []core.Task{sampleTask()}, NextCursor: "cur123"})
	if !strings.Contains(out, "next cursor: cur123") {
		t.Errorf("cursor missing: %s", out)
	}
	plain := render(t, FormatTable, core.TaskPage{Tasks: []core.Task{sampleTask()}})
	if strings.Contains(plain, "next cursor") {
		t.Errorf("unexpected cursor line: %s", plain)
	}
}

func TestPointerSlicesRender(t *testing.T) {
	task := sampleTask()
	out := render(t, FormatTable, []*core.Task{&task})
	if !strings.Contains(out, "ENG-7") {
		t.Errorf("pointer slice not rendered: %s", out)
	}
}
