// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestGuardCellDefusesFormulas(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"=SUM(A1)":   "'=SUM(A1)",
		"+1":         "'+1",
		"-x":         "'-x",
		"@cmd":       "'@cmd",
		"\tTAB":      "'\tTAB",
		"\rCR":       "'\rCR",
		"plain":      "plain",
		"a=b":        "a=b",
		"":           "",
		"2026-01-02": "2026-01-02",
	}
	for in, want := range cases {
		if got := GuardCell(in); got != want {
			t.Errorf("GuardCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTaskCSVWritesTheHeaderEvenWithNoRows(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := New(FormatCSV).Format(&buf, []core.Task{}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "ref,project,title,status,priority,assignee,tags,due,created,updated\n" {
		t.Fatalf("empty listing = %q", got)
	}
}

func TestTaskCSVRow(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	task := core.Task{Ref: "web-ops-12", Title: `say "hi", now`, Status: "todo", Priority: core.PriorityHigh,
		AssigneeActorID: "01ABC", Tags: []string{"a", "b"}, DueAt: &at, CreatedAt: at, UpdatedAt: at}
	var buf bytes.Buffer
	if err := NewWithNames(FormatCSV, ModeNever, TimeStyle{}, map[string]string{"01ABC": "lena"}).Format(&buf, []core.Task{task}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	want := `web-ops-12,web-ops,"say ""hi"", now",todo,high,lena,a b,2026-03-04T05:06:07Z,2026-03-04T05:06:07Z,2026-03-04T05:06:07Z`
	if len(lines) != 2 || lines[1] != want {
		t.Fatalf("row = %q, want %q", lines, want)
	}
	if err := New(FormatCSV).Format(&buf, core.Project{}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("csv of a project: err = %v, want invalid", err)
	}
}
