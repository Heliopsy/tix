// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func openAll(t *testing.T, r io.Reader, opts ReadOptions) (Reader, []Row) {
	t.Helper()
	rd, err := Open(r, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	var rows []Row
	for {
		row, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return rd, rows
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		rows = append(rows, row)
	}
}

func openFixture(t *testing.T, name string) (Reader, []Row) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	head := make([]byte, 512)
	n, _ := f.Read(head)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	return openAll(t, f, ReadOptions{Format: DetectFormat(name, head[:n])})
}

func day(y int, m time.Month, d, h, min int) *time.Time {
	t := time.Date(y, m, d, h, min, 0, 0, time.UTC)
	return &t
}

type draftWant struct {
	row       int
	title     string
	statusRaw string
	priority  core.Priority
	tags      []string
	due       *time.Time
	parent    string
	external  string
	assignee  string
	errors    int
}

func TestFixtures(t *testing.T) {
	fields := []FieldInfo{{Key: "points", Label: "Story points", Type: core.FieldInt}, {Key: "start", Label: "Start", Type: core.FieldDate}}
	tests := []struct {
		file    string
		source  string
		rows    int
		suggest map[string]Target
		drafts  []draftWant
	}{
		{
			file: "jira.csv", source: SourceJira, rows: 4,
			suggest: map[string]Target{
				"Summary": TargetTitle, "Issue key": TargetExternalID, "Issue id": TargetAlias,
				"Parent id": TargetParent, "Status": TargetStatus, "Priority": TargetPriority,
				"Assignee": TargetAssignee, "Due date": TargetDue, "Labels": TargetTags,
				"Labels (2)": TargetTags, "Description": TargetBody, "Created": TargetIgnore,
				"Watchers": TargetIgnore, "Watchers (2)": TargetIgnore, "Sprint": TargetAppendBody,
				"Sprint (2)": TargetAppendBody, "Custom field (Story Points)": CustomTarget("points"),
				"Project key": TargetIgnore, "Reporter": TargetAppendBody,
			},
			drafts: []draftWant{
				{row: 0, title: "Set up CI pipeline", statusRaw: "Done", priority: core.PriorityHigh,
					tags: []string{"infra", "ci"}, due: day(2026, 1, 15, 17, 0), external: "DEMO-1", assignee: "alice.smith"},
				{row: 2, title: "Fix login redirect", statusRaw: "To Do", priority: core.PriorityHighest,
					tags: []string{"auth", "urgent"}, due: day(2026, 2, 3, 11, 15), external: "DEMO-3"},
				{row: 3, title: "Add cache step", statusRaw: "Selected for Development", priority: core.PriorityLow,
					tags: []string{"ci"}, parent: "10001", external: "DEMO-4", assignee: "alice.smith"},
			},
		},
		{
			file: "github.csv", source: SourceGitHub, rows: 3,
			suggest: map[string]Target{
				"number": TargetExternalID, "title": TargetTitle, "body": TargetBody, "state": TargetStatus,
				"labels": TargetTags, "assignees": TargetAssignee, "milestone": TargetAppendBody,
				"html_url": TargetIgnore, "created_at": TargetIgnore, "closed_at": TargetIgnore,
			},
			drafts: []draftWant{
				{row: 1, title: "Add dark mode", statusRaw: "closed", tags: []string{"enhancement", "ui"},
					external: "15", assignee: "octo-dev"},
			},
		},
		{
			file: "github.json", source: SourceGitHub, rows: 2,
			suggest: map[string]Target{
				"number": TargetExternalID, "title": TargetTitle, "body": TargetBody, "state": TargetStatus,
				"labels": TargetTags, "assignee.login": TargetAssignee, "assignees": TargetIgnore,
				"milestone.title": TargetAppendBody, "html_url": TargetIgnore, "id": TargetIgnore,
				"user.login": TargetAppendBody, "user.id": TargetIgnore, "milestone.due_on": TargetIgnore,
				"node_id": TargetIgnore, "locked": TargetIgnore,
			},
			drafts: []draftWant{
				{row: 0, title: "Crash on empty config", statusRaw: "open", priority: 0, tags: []string{"bug", "p1"},
					external: "12", assignee: "octo-dev"},
				{row: 1, title: "Add dark mode", statusRaw: "closed", tags: []string{"enhancement", "ui"}, external: "15"},
			},
		},
		{
			file: "trello.json", source: SourceTrello, rows: 3,
			suggest: map[string]Target{
				"name": TargetTitle, "desc": TargetBody, "list": TargetStatus, "labels": TargetTags,
				"members": TargetAssignee, "due": TargetDue, "idShort": TargetExternalID, "id": TargetAlias,
				"closed": TargetIgnore, "url": TargetIgnore, "checklists": TargetAppendBody,
				"start": CustomTarget("start"),
			},
			drafts: []draftWant{
				{row: 0, title: "Plan the launch", statusRaw: "To Do", tags: []string{"marketing"},
					due: day(2026, 2, 1, 12, 0), external: "1", assignee: "pat.example"},
				{row: 1, title: "Design the banner", statusRaw: "Doing", tags: []string{"red", "marketing"},
					external: "2", assignee: "pat.example"},
			},
		},
		{
			file: "linear.csv", source: SourceLinear, rows: 3,
			suggest: map[string]Target{
				"ID": TargetExternalID, "Title": TargetTitle, "Description": TargetBody, "Status": TargetStatus,
				"Priority": TargetPriority, "Assignee": TargetAssignee, "Labels": TargetTags,
				"Due Date": TargetDue, "Parent issue": TargetParent, "Estimate": CustomTarget("points"),
				"Project ID": TargetIgnore, "Created": TargetIgnore, "Started": TargetIgnore,
				"Canceled": TargetIgnore, "Project": TargetAppendBody, "Cycle Name": TargetAppendBody,
			},
			drafts: []draftWant{
				{row: 0, title: "Migrate the database", statusRaw: "In Progress", priority: core.PriorityHigh,
					tags: []string{"Backend", "Infra"}, due: day(2026, 3, 15, 0, 0), external: "ENG-10", assignee: "lee.example"},
				{row: 1, title: "Write the migration script", statusRaw: "Todo", priority: core.PriorityHighest,
					tags: []string{"Backend"}, parent: "ENG-10", external: "ENG-11", assignee: "kim.example"},
				{row: 2, title: "Clean up old tables", statusRaw: "Backlog", priority: 0, parent: "ENG-10", external: "ENG-12"},
			},
		},
		{
			file: "generic.csv", source: "", rows: 3,
			suggest: map[string]Target{
				"Заглавие": TargetTitle, "Описание": TargetBody, "Статус": TargetStatus, "Срок": TargetDue,
				"Етикети": TargetTags,
			},
			drafts: []draftWant{
				{row: 0, title: "Подготви офертата", statusRaw: "Нова", tags: []string{"продажби", "спешно"},
					due: day(2026, 3, 15, 0, 0)},
				{row: 2, statusRaw: "Готово", due: day(2026, 12, 31, 0, 0), errors: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			rd, rows := openFixture(t, tt.file)
			if rd.Source() != tt.source {
				t.Errorf("Source = %q, want %q", rd.Source(), tt.source)
			}
			if len(rows) != tt.rows {
				t.Fatalf("rows = %d, want %d", len(rows), tt.rows)
			}
			got := Suggest(rd.Headers(), rd.Source(), fields)
			for h, want := range tt.suggest {
				if got[h] != want {
					t.Errorf("Suggest[%q] = %q, want %q", h, got[h], want)
				}
			}
			m := &Mapping{}
			cols := m.Resolve(rd.Headers(), rd.Source(), fields)
			for _, w := range tt.drafts {
				d := m.Draft(rows[w.row], cols)
				if d.Title != w.title || d.StatusRaw != w.statusRaw || d.Priority != w.priority ||
					d.Parent != w.parent || d.ExternalID != w.external || d.Assignee != w.assignee {
					t.Errorf("row %d: got title %q status %q priority %d parent %q external %q assignee %q",
						w.row, d.Title, d.StatusRaw, d.Priority, d.Parent, d.ExternalID, d.Assignee)
				}
				if !reflect.DeepEqual(d.Tags, w.tags) {
					t.Errorf("row %d: tags = %q, want %q", w.row, d.Tags, w.tags)
				}
				if !reflect.DeepEqual(d.Due, w.due) {
					t.Errorf("row %d: due = %v, want %v", w.row, d.Due, w.due)
				}
				if len(d.Errors) != w.errors {
					t.Errorf("row %d: errors = %q, want %d", w.row, d.Errors, w.errors)
				}
			}
		})
	}
}

func TestJiraDraftBodyAndCustom(t *testing.T) {
	rd, rows := openFixture(t, "jira.csv")
	fields := []FieldInfo{{Key: "points", Label: "Story points", Type: core.FieldInt}}
	m := &Mapping{}
	d := m.Draft(rows[1], m.Resolve(rd.Headers(), rd.Source(), fields))
	for _, part := range []string{
		"Cover the local setup.\nMention the \"make\" targets.",
		"**Issue Type:** Story",
		"**Sprint:** Sprint 1",
		"**Sprint:** Sprint 2",
		"**Reporter:** bob.jones",
	} {
		if !strings.Contains(d.Body, part) {
			t.Errorf("body lacks %q:\n%s", part, d.Body)
		}
	}
	if strings.Contains(d.Body, "Watchers") || strings.Contains(d.Body, "Resolution") {
		t.Errorf("body carries ignored or empty columns:\n%s", d.Body)
	}
	if !strings.HasPrefix(d.Body, "Cover the local setup.") {
		t.Errorf("body does not start with the description:\n%s", d.Body)
	}
	if d.Custom["points"] != "5" {
		t.Errorf("custom points = %q", d.Custom["points"])
	}
	if !reflect.DeepEqual(d.Aliases, []string{"10002"}) {
		t.Errorf("aliases = %q", d.Aliases)
	}
}

func TestTrelloChecklistAndMembers(t *testing.T) {
	_, rows := openFixture(t, "trello.json")
	if got := rows[0].Values["checklists"]; got != "### Steps\n- [x] Pick a date\n- [ ] Book the venue" {
		t.Errorf("checklists = %q", got)
	}
	if got := rows[1].Values["members"]; got != "pat.example, Sam Sample" {
		t.Errorf("members = %q", got)
	}
	if got := rows[2].Values["closed"]; got != "true" {
		t.Errorf("closed = %q", got)
	}
	if got := rows[2].Values["list"]; got != "Done" {
		t.Errorf("list = %q", got)
	}
}
