// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

const sampleMapping = `
version: 1
system: jira
project: ops
workflow: default
identity:
  id: key
  url: url
  version: fields.updated
  updated_at: fields.updated
fields:
  fields.summary: title
  fields.description: body
  fields.duedate: due_at
  fields.labels: tags
  fields.assignee: assignee
  fields.priority.name: priority
  fields.parent.key: parent
status_field: fields.status.name
statuses:
  To Do: todo
  In Progress: doing
  Done: done
default_status: todo
priorities:
  Blocker: highest
types:
  field: fields.issuetype.name
  map:
    Bug:
      tags: [bug]
      priority: high
custom:
  fields.customfield_10001:
    key: story_points
    type: float
    label: Story points
unmapped:
  preserve: true
  prefix: ext_
lossy:
  - field: fields.comment
    reason: comments are flattened to a count
`

func testWorkflow() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo"}, {Key: "doing"}, {Key: "done", Terminal: true},
		},
	}
}

func mustMapping(t *testing.T, raw string) *Mapping {
	t.Helper()
	m, err := ParseMapping([]byte(raw))
	if err != nil {
		t.Fatalf("parsing mapping: %v", err)
	}
	return m
}

func jiraRecord(t *testing.T) Record {
	t.Helper()
	raw := `{
		"key": "OPS-1",
		"url": "https://jira.test/browse/OPS-1",
		"fields": {
			"summary": "ship it",
			"description": "the body",
			"duedate": "2026-03-01",
			"labels": ["ops", "urgent"],
			"assignee": "alice",
			"priority": {"name": "Blocker"},
			"status": {"name": "In Progress"},
			"issuetype": {"name": "Bug"},
			"updated": "2026-02-01T10:00:00Z",
			"customfield_10001": "8",
			"comment": {"total": 3},
			"reporter": "bob"
		}
	}`
	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return Record{Fields: fields}
}

func TestMappingAppliesStatusesFieldsAndTypes(t *testing.T) {
	m := mustMapping(t, sampleMapping)
	if err := m.ValidateAgainst(testWorkflow()); err != nil {
		t.Fatalf("validating mapping: %v", err)
	}

	got, err := m.Apply(jiraRecord(t))
	if err != nil {
		t.Fatalf("applying mapping: %v", err)
	}
	if got.ExternalID != "OPS-1" {
		t.Errorf("external id = %q", got.ExternalID)
	}
	if got.ExternalURL != "https://jira.test/browse/OPS-1" {
		t.Errorf("external url = %q", got.ExternalURL)
	}
	if got.ExternalVersion != "2026-02-01T10:00:00Z" {
		t.Errorf("external version = %q", got.ExternalVersion)
	}
	if got.Title != "ship it" || got.Body != "the body" {
		t.Errorf("title/body = %q / %q", got.Title, got.Body)
	}
	if got.Status != "doing" {
		t.Errorf("status = %q, want the mapped workflow state", got.Status)
	}
	if got.Priority != core.PriorityHigh {
		t.Errorf("priority = %v, want the type rule's high", got.Priority)
	}
	if got.DueAt == nil || got.DueAt.Format("2006-01-02") != "2026-03-01" {
		t.Errorf("due = %v", got.DueAt)
	}
	if !strings.Contains(strings.Join(got.Tags, ","), "bug") {
		t.Errorf("tags = %v, want the type rule's tag", got.Tags)
	}
	if got.Assignee != "alice" {
		t.Errorf("assignee = %q", got.Assignee)
	}
	if got.CustomFields["story_points"] != 8.0 {
		t.Errorf("story_points = %v, want the coerced float", got.CustomFields["story_points"])
	}
}

func TestMappingUsesThePriorityTableWhenNoTypeRuleApplies(t *testing.T) {
	m := mustMapping(t, sampleMapping)
	var fields map[string]any
	if err := json.Unmarshal([]byte(`{"key":"OPS-7","fields":{"summary":"x","priority":{"name":"Blocker"}}}`), &fields); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	got, err := m.Apply(Record{Fields: fields})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got.Priority != core.PriorityHighest {
		t.Errorf("priority = %v, want the mapped highest", got.Priority)
	}
}

func TestMappingPreservesUnmappedFields(t *testing.T) {
	m := mustMapping(t, sampleMapping)
	got, err := m.Apply(jiraRecord(t))
	if err != nil {
		t.Fatalf("applying mapping: %v", err)
	}
	if got.CustomFields["ext_fields_reporter"] != "bob" {
		t.Fatalf("unmapped reporter was dropped: %v", got.CustomFields)
	}
	if !containsSubstring(got.Warnings, "preserved unmapped fields") {
		t.Errorf("warnings = %v, want the preservation reported", got.Warnings)
	}
}

func TestMappingReportsLossyConcepts(t *testing.T) {
	m := mustMapping(t, sampleMapping)
	got, err := m.Apply(jiraRecord(t))
	if err != nil {
		t.Fatalf("applying mapping: %v", err)
	}
	if !containsSubstring(got.Warnings, "lossy mapping for OPS-1") {
		t.Errorf("warnings = %v, want the lossy concept reported", got.Warnings)
	}
}

func TestMappingRefusesUnknownWorkflowState(t *testing.T) {
	m := mustMapping(t, strings.Replace(sampleMapping, "In Progress: doing", "In Progress: nowhere", 1))
	err := m.ValidateAgainst(testWorkflow())
	if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "In Progress") {
		t.Fatalf("ValidateAgainst = %v, want the offending entry named", err)
	}
}

func TestMappingRefusesUnknownDefaultAndTypeState(t *testing.T) {
	m := mustMapping(t, strings.Replace(sampleMapping, "default_status: todo", "default_status: nowhere", 1))
	if err := m.ValidateAgainst(testWorkflow()); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("default_status = %v, want invalid", err)
	}
	withType := strings.Replace(sampleMapping, "      priority: high", "      status: nowhere", 1)
	if err := mustMapping(t, withType).ValidateAgainst(testWorkflow()); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("types.map status = %v, want invalid", err)
	}
}

func TestParseMappingRejectsBadDocuments(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"no project", "version: 1\nidentity:\n  id: key\n", "project"},
		{"no identity", "version: 1\nproject: ops\n", "external identifier"},
		{"unknown target", "version: 1\nproject: ops\nidentity:\n  id: key\nfields:\n  a: nonsense\n", "nonsense"},
		{"custom without key", "version: 1\nproject: ops\nidentity:\n  id: key\ncustom:\n  a:\n    type: int\n", "custom field key"},
		{"custom bad type", "version: 1\nproject: ops\nidentity:\n  id: key\ncustom:\n  a:\n    key: k\n    type: rune\n", "field type"},
		{"bad priority", "version: 1\nproject: ops\nidentity:\n  id: key\npriorities:\n  P1: sideways\n", "priority"},
		{"statuses without field", "version: 1\nproject: ops\nidentity:\n  id: key\nstatuses:\n  A: todo\n", "status_field"},
		{"wrong version", "version: 99\nproject: ops\nidentity:\n  id: key\n", "version"},
		{"not yaml", "\tnot: [yaml", "parsing mapping"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseMapping([]byte(tc.raw))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseMapping = %v, want an error mentioning %q", err, tc.want)
			}
		})
	}
}

func TestApplyReportsRecordLevelFailures(t *testing.T) {
	m := mustMapping(t, sampleMapping)
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"no identity value", `{"fields":{"summary":"x"}}`, "identity field"},
		{"no title", `{"key":"OPS-2","fields":{}}`, "maps to no title"},
		{"bad due date", `{"key":"OPS-3","fields":{"summary":"x","duedate":"soon"}}`, "due date"},
		{"bad custom value", `{"key":"OPS-4","fields":{"summary":"x","customfield_10001":"lots"}}`, "story_points"},
		{"bad priority", `{"key":"OPS-5","fields":{"summary":"x","priority":{"name":"sideways"}}}`, "priority"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var fields map[string]any
			if err := json.Unmarshal([]byte(tc.raw), &fields); err != nil {
				t.Fatalf("fixture: %v", err)
			}
			_, err := m.Apply(Record{Fields: fields})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Apply = %v, want an error mentioning %q", err, tc.want)
			}
		})
	}
}

func TestApplyFallsBackForUnknownStatus(t *testing.T) {
	m := mustMapping(t, sampleMapping)
	var fields map[string]any
	if err := json.Unmarshal([]byte(`{"key":"OPS-9","fields":{"summary":"x","status":{"name":"Parked"},
		"issuetype":{"name":"Chore"}}}`), &fields); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	got, err := m.Apply(Record{Fields: fields})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got.Status != "todo" {
		t.Errorf("status = %q, want the default", got.Status)
	}
	if !containsSubstring(got.Warnings, "fell back") {
		t.Errorf("warnings = %v, want the fallback reported", got.Warnings)
	}
	if got.CustomFields["ext_type"] != "Chore" {
		t.Errorf("unmapped issue type was dropped: %v", got.CustomFields)
	}
}

func TestApplyRefusesUnknownStatusWithoutDefault(t *testing.T) {
	m := mustMapping(t, strings.Replace(sampleMapping, "default_status: todo", "", 1))
	var fields map[string]any
	if err := json.Unmarshal([]byte(`{"key":"OPS-9","fields":{"summary":"x","status":{"name":"Parked"}}}`), &fields); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if _, err := m.Apply(Record{Fields: fields}); err == nil {
		t.Fatal("Apply accepted an unmappable status with no default")
	}
}

func TestLoadMappingReadsAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mapping.yaml")
	if err := os.WriteFile(path, []byte(sampleMapping), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	if _, err := LoadMapping(path); err != nil {
		t.Fatalf("LoadMapping: %v", err)
	}
	if _, err := LoadMapping(""); err == nil {
		t.Error("LoadMapping accepted an empty path")
	}
	if _, err := LoadMapping(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Error("LoadMapping accepted a missing file")
	}
}

func TestRequiredPathsNamesIdentityAndTitle(t *testing.T) {
	m := mustMapping(t, sampleMapping)
	got := strings.Join(m.RequiredPaths(), ",")
	if !strings.Contains(got, "key") || !strings.Contains(got, "fields.summary") {
		t.Errorf("RequiredPaths() = %s", got)
	}
}

func TestCustomKeyAndPriorityParsing(t *testing.T) {
	if got := CustomKey("fields.Custom Field-1"); got != "fields_custom_field_1" {
		t.Errorf("CustomKey() = %q", got)
	}
	if p, err := parsePriority("2"); err != nil || p != core.PriorityHigh {
		t.Errorf("parsePriority(2) = %v, %v", p, err)
	}
	if p, err := parsePriority(""); err != nil || p != 0 {
		t.Errorf("parsePriority(empty) = %v, %v", p, err)
	}
	if _, err := parsePriority("9"); err == nil {
		t.Error("parsePriority accepted an out-of-range number")
	}
}

func containsSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
