// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// A cell becomes a custom field's value in that field's type, and a cell that
// cannot names the field and what it needed.
func TestImportFieldValueReadsEachFieldType(t *testing.T) {
	enum := core.FieldDef{Key: "size", Type: core.FieldEnum, EnumOptions: []string{"Small", "Large"}}
	for _, tc := range []struct {
		def  core.FieldDef
		raw  string
		want any
	}{
		{core.FieldDef{Key: "n", Type: core.FieldInt}, " 1 200 ", int64(1200)},
		{core.FieldDef{Key: "n", Type: core.FieldInt}, "3.0", int64(3)},
		{core.FieldDef{Key: "f", Type: core.FieldFloat}, "2,5", 2.5},
		{core.FieldDef{Key: "b", Type: core.FieldBool}, "Yes", true},
		{core.FieldDef{Key: "b", Type: core.FieldBool}, "не", false},
		{core.FieldDef{Key: "d", Type: core.FieldDate}, "2026-11-06", "2026-11-06"},
		{enum, "small", "Small"},
		{core.FieldDef{Key: "j", Type: core.FieldJSON}, "plain text", "plain text"},
		{core.FieldDef{Key: "s", Type: core.FieldString}, " kept ", "kept"},
	} {
		got, err := importFieldValue(tc.def, tc.raw, "")
		if err != nil || got != tc.want {
			t.Errorf("%s %q = %#v, %v; want %#v", tc.def.Type, tc.raw, got, err, tc.want)
		}
	}
	obj, err := importFieldValue(core.FieldDef{Key: "j", Type: core.FieldJSON}, `{"a":1}`, "")
	if raw, _ := json.Marshal(obj); err != nil || string(raw) != `{"a":1}` {
		t.Errorf("a JSON cell = %s, %v", raw, err)
	}

	for _, tc := range []struct {
		def  core.FieldDef
		raw  string
		want string
	}{
		{core.FieldDef{Key: "n", Type: core.FieldInt}, "3.5", "whole number"},
		{core.FieldDef{Key: "f", Type: core.FieldFloat}, "lots", "needs a number"},
		{core.FieldDef{Key: "b", Type: core.FieldBool}, "maybe", "yes or no"},
		{core.FieldDef{Key: "d", Type: core.FieldDateTime}, "someday", `custom field "d"`},
		{enum, "medium", "accepts only Small, Large"},
	} {
		_, err := importFieldValue(tc.def, tc.raw, "")
		if core.KindOf(err) != core.KindInvalid || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s %q: %v, want invalid mentioning %q", tc.def.Type, tc.raw, err, tc.want)
		}
	}
}

func TestErrMessageDropsTheKind(t *testing.T) {
	if got := errMessage(core.Invalid("bad %s", "row")); got != "bad row" {
		t.Errorf("errMessage(domain) = %q", got)
	}
	if got := errMessage(errors.New("plain")); got != "plain" {
		t.Errorf("errMessage(plain) = %q", got)
	}
}

// The history column carries a task's audit trail in order, and the date
// styles render an instant as asked; a relative style cannot be read back,
// so it is refused.
func TestExportTasksCarriesHistoryAndDateStyles(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")

	rows := csvRows(t, exportTasks(t, l, ctx, core.TaskExportInput{
		ProjectRef: "infra", Columns: []string{"title", "history", "created_at"}, Dates: core.TaskDatesDate,
	}))
	if len(rows) < 2 || !strings.Contains(rows[1][1], "task.create") {
		t.Fatalf("history cell = %v, want the create entry", rows)
	}
	if len(rows[1][2]) != len("2006-01-02") {
		t.Errorf("a date-style created_at = %q", rows[1][2])
	}

	shown := exportTasks(t, l, ctx, core.TaskExportInput{ProjectRef: "infra", Columns: []string{"created_at"},
		Dates: core.TaskDatesDisplay, TimeFormat: "dotted", Timezone: "Europe/Sofia"})
	if !strings.Contains(shown, ".20") {
		t.Errorf("display dates are not dotted:\n%s", shown)
	}
	for _, format := range []string{"relative", "roman"} {
		err := l.ExportTasks(ctx, core.TaskExportInput{ProjectRef: "infra", Dates: core.TaskDatesDisplay,
			TimeFormat: format}, &strings.Builder{})
		if core.FieldOf(err) != "time_format" {
			t.Errorf("display in %s: %v, want a refusal on time_format", format, err)
		}
	}
}
