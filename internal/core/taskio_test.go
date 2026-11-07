// SPDX-License-Identifier: AGPL-3.0-or-later

package core_test

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestTaskFormatsNameTheirExtension(t *testing.T) {
	for f, want := range map[core.TaskFormat]string{
		core.TaskFormatCSV: "csv", core.TaskFormatJSON: "json",
		core.TaskFormatNDJSON: "ndjson", core.TaskFormatMarkdown: "md",
	} {
		if !f.Valid() {
			t.Errorf("%s is not valid", f)
		}
		if got := f.Extension(); got != want {
			t.Errorf("%s.Extension() = %q, want %q", f, got, want)
		}
	}
	if core.TaskFormat("xlsx").Valid() || core.TaskFormatLines.Valid() {
		t.Error("an import-only or unknown format passes as an export format")
	}
}

func TestCustomColumnsRoundTrip(t *testing.T) {
	col := core.CustomColumn("sprint")
	if key, ok := core.CustomColumnKey(col); !ok || key != "sprint" {
		t.Errorf("CustomColumnKey(%q) = %q, %v", col, key, ok)
	}
	for _, c := range []string{"custom:", "title"} {
		if _, ok := core.CustomColumnKey(c); ok {
			t.Errorf("%q read as a custom column", c)
		}
	}
}

func TestTaskExportInputFillsDefaultsAndNamesTheBadField(t *testing.T) {
	in := core.TaskExportInput{ProjectRef: "infra"}.Normalize()
	if in.Format != core.TaskFormatCSV || in.Dates != core.TaskDatesISO || in.Delimiter != "," {
		t.Errorf("Normalize() = %+v, want csv, iso and a comma", in)
	}
	if err := in.Validate(); err != nil {
		t.Errorf("the defaults do not validate: %v", err)
	}
	ok := core.TaskExportInput{ProjectRef: "infra", Columns: []string{"title", "custom:sprint"}, Delimiter: "tab"}
	if err := ok.Validate(); err != nil {
		t.Errorf("a custom column and a tab refused: %v", err)
	}
	for _, tc := range []struct {
		name  string
		in    core.TaskExportInput
		field string
		msg   string
	}{
		{"no project", core.TaskExportInput{ProjectRef: "  "}, "project_ref", "project"},
		{"format", core.TaskExportInput{ProjectRef: "p", Format: "xlsx"}, "format", "csv, json, ndjson, markdown"},
		{"dates", core.TaskExportInput{ProjectRef: "p", Dates: "epoch"}, "dates", "epoch"},
		{"delimiter", core.TaskExportInput{ProjectRef: "p", Delimiter: ":"}, "delimiter", ":"},
		{"twice", core.TaskExportInput{ProjectRef: "p", Columns: []string{"title", "title"}}, "columns", "twice"},
		{"unknown", core.TaskExportInput{ProjectRef: "p", Columns: []string{"colour"}}, "columns", "colour"},
	} {
		err := tc.in.Validate()
		if core.KindOf(err) != core.KindInvalid || core.FieldOf(err) != tc.field || !strings.Contains(err.Error(), tc.msg) {
			t.Errorf("%s: %v (field %q), want invalid on %s mentioning %q", tc.name, err, core.FieldOf(err), tc.field, tc.msg)
		}
	}
}

func TestTaskImportInputNamesTheBadField(t *testing.T) {
	if err := (core.TaskImportInput{ProjectRef: "p", Format: core.TaskFormatLines, Mode: core.TaskImportSkip, Preview: 200}).Validate(); err != nil {
		t.Errorf("a valid input refused: %v", err)
	}
	for _, tc := range []struct {
		in    core.TaskImportInput
		field string
	}{
		{core.TaskImportInput{}, "project_ref"},
		{core.TaskImportInput{ProjectRef: "p", Format: core.TaskFormatMarkdown}, "format"},
		{core.TaskImportInput{ProjectRef: "p", Mode: "merge"}, "mode"},
		{core.TaskImportInput{ProjectRef: "p", Preview: -1}, "preview"},
		{core.TaskImportInput{ProjectRef: "p", Preview: core.MaxImportPreview + 1}, "preview"},
	} {
		if err := tc.in.Validate(); core.FieldOf(err) != tc.field {
			t.Errorf("%+v: %v, want a refusal on %s", tc.in, err, tc.field)
		}
	}
}

func TestTimePrefsValidateAndNormalize(t *testing.T) {
	s := func(v string) *string { return &v }
	if err := (core.TimePrefsInput{TimeFormat: s("dotted"), Timezone: s("Europe/Sofia")}).Validate(); err != nil {
		t.Errorf("a valid choice refused: %v", err)
	}
	if err := core.ValidateTimePrefs(nil, s("")); err != nil {
		t.Errorf("nil and empty refused: %v", err)
	}
	for _, tc := range []struct {
		format, zone *string
		want         string
	}{
		{s("roman"), nil, "roman"},
		{nil, s("Local"), "server's zone"},
		{nil, s("Mars/Olympus"), "Mars/Olympus"},
	} {
		err := core.ValidateTimePrefs(tc.format, tc.zone)
		if core.KindOf(err) != core.KindInvalid || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ValidateTimePrefs: %v, want invalid mentioning %q", err, tc.want)
		}
	}

	format, zone := " Dotted ", " utc "
	core.NormalizeTimePrefs(&format, &zone)
	if format != "dotted" || zone != "UTC" {
		t.Errorf("normalized to %q, %q", format, zone)
	}
	core.NormalizeTimePrefs(nil, nil)
}

// Each field resolves on its own: a person who chose only a zone keeps the
// tenant's format.
func TestTimePrefsResolveFieldByField(t *testing.T) {
	got := core.ResolveTimePrefs(
		core.TimePrefs{TimeFormat: "iso", Timezone: "UTC"},
		core.TimePrefs{TimeFormat: "dotted"},
		core.TimePrefs{Timezone: "Asia/Tokyo"})
	if got.Effective != (core.TimePrefs{TimeFormat: "dotted", Timezone: "Asia/Tokyo"}) {
		t.Errorf("effective = %+v", got.Effective)
	}
	if got.FormatSource != core.TimeSourceTenant || got.TimezoneSource != core.TimeSourceUser {
		t.Errorf("sources = %s, %s", got.FormatSource, got.TimezoneSource)
	}
	global := core.ResolveTimePrefs(core.TimePrefs{TimeFormat: "iso"}, core.TimePrefs{}, core.TimePrefs{})
	if global.Effective.TimeFormat != "iso" || global.FormatSource != core.TimeSourceGlobal {
		t.Errorf("fallback = %+v", global)
	}
}

func TestInviteInputAndExpiry(t *testing.T) {
	if got := core.InvitePath("abc"); got != "/invite/abc" {
		t.Errorf("InvitePath = %q", got)
	}
	if d, err := core.InviteExpiry(0); err != nil || d != core.DefaultInviteExpiry {
		t.Errorf("no expiry = %v, %v; want the default", d, err)
	}
	if d, err := core.InviteExpiry(core.Duration(48 * time.Hour)); err != nil || d != 48*time.Hour {
		t.Errorf("48h = %v, %v", d, err)
	}
	for _, d := range []time.Duration{time.Minute, 31 * 24 * time.Hour} {
		if _, err := core.InviteExpiry(core.Duration(d)); core.FieldOf(err) != "expires_in" {
			t.Errorf("%s accepted: %v", d, err)
		}
	}
	if err := (core.CreateInviteInput{Email: "ada@example.com", Role: core.RoleMember}).Validate(); err != nil {
		t.Errorf("a valid invite refused: %v", err)
	}
	for _, tc := range []struct {
		in    core.CreateInviteInput
		field string
	}{
		{core.CreateInviteInput{Email: " "}, "email"},
		{core.CreateInviteInput{Email: "ada"}, "email"},
		{core.CreateInviteInput{Email: "ada@example.com", Role: "emperor"}, "role"},
		{core.CreateInviteInput{Email: "ada@example.com", ExpiresIn: core.Duration(time.Second)}, "expires_in"},
	} {
		if err := tc.in.Validate(); core.FieldOf(err) != tc.field {
			t.Errorf("%+v: %v, want a refusal on %s", tc.in, err, tc.field)
		}
	}
}

func TestAnInviteIsPendingUntilUsedRevokedOrExpired(t *testing.T) {
	now := time.Now()
	live := core.Invite{ExpiresAt: now.Add(time.Hour)}
	if !live.Pending(now) {
		t.Error("a live invite is not pending")
	}
	used, revoked := live, live
	used.AcceptedAt, revoked.RevokedAt = &now, &now
	for name, inv := range map[string]core.Invite{
		"used": used, "revoked": revoked, "expired": {ExpiresAt: now.Add(-time.Minute)},
	} {
		if inv.Pending(now) {
			t.Errorf("a %s invite is pending", name)
		}
	}
}
