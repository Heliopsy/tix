// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"reflect"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestShowKeyAndShowRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, key, ref string
	}{
		{"plain key", "infra", "INFRA", "infra"},
		{"ref", "infra-8", "INFRA-8", "INFRA-8"},
		{"key with a hyphen", "web-app-12", "WEB-APP-12", "WEB-APP-12"},
		{"already upper", "INFRA-8", "INFRA-8", "INFRA-8"},
		{"stable identifier", "01jabcdefghjkmnpqrstvwxyz0", "01JABCDEFGHJKMNPQRSTVWXYZ0", "01jabcdefghjkmnpqrstvwxyz0"},
		{"not a ref", "session", "SESSION", "session"},
		{"empty", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ShowKey(tc.in); got != tc.key {
				t.Errorf("ShowKey(%q) = %q, want %q", tc.in, got, tc.key)
			}
			if got := ShowRef(tc.in); got != tc.ref {
				t.Errorf("ShowRef(%q) = %q, want %q", tc.in, got, tc.ref)
			}
		})
	}
}

// A painted ref is the upper-case ref, with or without colour.
func TestPainterRefShowsTheRefUpperCase(t *testing.T) {
	t.Parallel()
	if got := NewPainter(ModeNever, nil).Ref("infra-8"); got != "INFRA-8" {
		t.Fatalf("plain Ref = %q", got)
	}
	if got := NewPainter(ModeAlways, new(strings.Builder)).Ref("infra-8"); !strings.Contains(got, "INFRA-8") {
		t.Fatalf("coloured Ref = %q", got)
	}
}

// A reflected table shows a field tagged show:"ref" or show:"key" in display
// form and leaves every other field as it is.
func TestReflectedCellsTaggedShowRenderUpperCase(t *testing.T) {
	t.Parallel()
	type row struct {
		Ref     string `json:"ref" show:"ref"`
		Project string `json:"project" show:"key"`
		Name    string `json:"name"`
	}
	var b strings.Builder
	if err := New(FormatTable).Format(&b, []row{{Ref: "infra-8", Project: "infra", Name: "lower-1"}}); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{"INFRA-8", "INFRA ", "lower-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("table is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "infra") {
		t.Errorf("a tagged cell kept its lower case:\n%s", got)
	}
	field, _ := reflect.TypeOf(row{}).FieldByName("Name")
	if showCell(field, "lower-1") != "lower-1" {
		t.Error("an untagged cell was changed")
	}
}

// Every table that names a project or a task shows it upper case.
func TestTablesShowProjectKeysAndRefsUpperCase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data any
		want string
	}{
		{"task", []core.Task{{Ref: "infra-8", Title: "t"}}, "INFRA-8"},
		{"project", []core.Project{{Key: "infra", Name: "Infrastructure"}}, "INFRA "},
		{"inbox", []core.InboxItem{{ID: "i1", Kind: core.InboxMention, TaskRef: "infra-8"}}, "INFRA-8"},
		{"template", []core.TaskTemplate{{Name: "release", ProjectID: "p1", ProjectKey: "infra"}}, "INFRA "},
		{"claim", core.Claim{Task: &core.Task{Ref: "infra-8"}}, "INFRA-8"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var b strings.Builder
			if err := New(FormatTable).Format(&b, tc.data); err != nil {
				t.Fatal(err)
			}
			if got := b.String(); !strings.Contains(got, tc.want) || strings.Contains(got, "infra") {
				t.Fatalf("table does not show %q in place of the stored key:\n%s", tc.want, got)
			}
		})
	}
}
