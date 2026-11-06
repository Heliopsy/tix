// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"reflect"
	"strings"
	"testing"
)

const sampleMapping = `version: 1
source: jira
format: csv
delimiter: ";"
mode: upsert
date_format: DD/MM/YYYY
columns:
  Summary: title
  Issue key: external_id
  Sprint: append_body
  Story Points: custom:points
  Watchers: ignore
values:
  status: {"In Progress": doing, "Done": done}
  priority: {Highest: highest, "P1": high}
  assignee: {"jane.doe@example.com": jane}
unknown:
  status: default
  assignee: ignore
  priority: default
defaults:
  status: todo
  priority: normal
tags_separator: ","
strict: false
`

func TestParseMappingRoundTrip(t *testing.T) {
	m, err := ParseMapping([]byte(sampleMapping))
	if err != nil {
		t.Fatal(err)
	}
	if m.Columns["Story Points"] != CustomTarget("points") || m.Values[TargetStatus]["In Progress"] != "doing" ||
		m.Unknown[TargetAssignee] != PolicyIgnore || m.Defaults[TargetPriority] != "normal" || m.Mode != ModeUpsert {
		t.Fatalf("parsed mapping = %+v", m)
	}
	raw, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseMapping(raw)
	if err != nil {
		t.Fatalf("re-parsing %s: %v", raw, err)
	}
	if !reflect.DeepEqual(m, again) {
		t.Errorf("round trip changed the mapping:\n%s\n%+v\n%+v", raw, m, again)
	}
	second, _ := again.Marshal()
	if string(raw) != string(second) {
		t.Errorf("Marshal is not stable:\n%s\n---\n%s", raw, second)
	}
	if !strings.HasPrefix(string(raw), "version: 1\nsource: jira\n") {
		t.Errorf("Marshal does not lead with version and source:\n%s", raw)
	}
}

func TestParseMappingJSON(t *testing.T) {
	m, err := ParseMapping([]byte(`{"columns":{"Name":"title"},"mode":"create","strict":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 1 || m.Columns["Name"] != TargetTitle || m.Mode != ModeCreate || !m.Strict {
		t.Errorf("mapping = %+v", m)
	}
	empty, err := ParseMapping(nil)
	if err != nil || empty.Version != 1 {
		t.Errorf("empty mapping = %+v, %v", empty, err)
	}
	raw, err := (&Mapping{}).Marshal()
	if err != nil || string(raw) != "version: 1\n" {
		t.Errorf("zero mapping marshals to %q, %v", raw, err)
	}
}

func TestParseMappingRefuses(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"version", "version: 2", "mapping version 2 is not supported"},
		{"target", "columns: {A: titel}", `column "A" maps to "titel", which is not a target`},
		{"custom without key", "columns: {A: 'custom:'}", "not a target"},
		{"custom with space", "columns: {A: 'custom:a b'}", "not a target"},
		{"mode", "mode: merge", `mode "merge" is not one of create, upsert or skip`},
		{"policy", "unknown: {status: ignore}", `the status policy "ignore" is not one of default, skip_row, error, keep`},
		{"assignee policy", "unknown: {assignee: default}", "assignee policy"},
		{"policy target", "unknown: {tags: keep}", "only status, assignee and priority take a policy"},
		{"values target", "values: {colour: {a: b}}", `values names "colour"`},
		{"defaults target", "defaults: {ignore: x}", `defaults names "ignore"`},
		{"date format", "date_format: DD/MMM/YYYY hh", "is not one of the tokens"},
		{"date format digit", "date_format: DD1MM", "is not one of the tokens"},
		{"format", "format: xls", `format "xls"`},
		{"delimiter", "delimiter: ';;'", "not a single character"},
		{"quote delimiter", `delimiter: '"'`, "not a single character"},
		{"unknown key", "colums: {}", "reading the mapping"},
		{"syntax", "columns: [", "reading the mapping"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMapping([]byte(tt.in))
			assertInvalid(t, err, tt.want)
		})
	}
}

func TestMappingHelpers(t *testing.T) {
	m := &Mapping{Delimiter: "tab", Format: "jsonl"}
	opts, err := m.ReadOptions(Limits{MaxRows: 3})
	if err != nil || opts.Delimiter != '\t' || opts.Format != FormatNDJSON || opts.Limits.MaxRows != 3 {
		t.Errorf("ReadOptions = %+v, %v", opts, err)
	}
	if _, err := (&Mapping{Format: "x"}).ReadOptions(Limits{}); err == nil {
		t.Error("ReadOptions accepted a bad format")
	}
	if _, err := (&Mapping{Delimiter: "ab"}).ReadOptions(Limits{}); err == nil {
		t.Error("ReadOptions accepted a bad delimiter")
	}
	tests := []struct {
		m        Mapping
		detected string
		source   string
		mode     Mode
	}{
		{Mapping{}, "", DefaultSource, ModeUpsert},
		{Mapping{}, SourceJira, SourceJira, ModeUpsert},
		{Mapping{Source: "acme", Mode: ModeSkip}, SourceJira, "acme", ModeSkip},
	}
	for _, tt := range tests {
		if got := tt.m.SourceName(tt.detected); got != tt.source {
			t.Errorf("SourceName = %q, want %q", got, tt.source)
		}
		if got := tt.m.EffectiveMode(); got != tt.mode {
			t.Errorf("EffectiveMode = %q, want %q", got, tt.mode)
		}
	}
	p := &Mapping{Unknown: map[Target]UnknownPolicy{TargetStatus: PolicyError}}
	for target, want := range map[Target]UnknownPolicy{
		TargetStatus: PolicyError, TargetAssignee: PolicyIgnore, TargetPriority: PolicyDefault, TargetTags: PolicyKeep,
	} {
		if got := p.Policy(target); got != want {
			t.Errorf("Policy(%s) = %q, want %q", target, got, want)
		}
	}
}

func TestResolve(t *testing.T) {
	headers := []string{"Summary", "Name", "Labels", "Labels (2)", "Created", "Notes", "Status"}
	tests := []struct {
		name string
		m    Mapping
		want map[string]Target
	}{
		{
			name: "suggestions alone",
			want: map[string]Target{
				"Summary": TargetTitle, "Name": TargetAppendBody, "Labels": TargetTags, "Labels (2)": TargetTags,
				"Created": TargetIgnore, "Notes": TargetBody, "Status": TargetStatus,
			},
		},
		{
			name: "explicit column wins and demotes the suggestion",
			m:    Mapping{Columns: map[string]Target{"Name": TargetTitle, "Labels": TargetAppendBody}},
			want: map[string]Target{
				"Summary": TargetAppendBody, "Name": TargetTitle, "Labels": TargetAppendBody,
				"Labels (2)": TargetAppendBody, "Created": TargetIgnore, "Notes": TargetBody, "Status": TargetStatus,
			},
		},
		{
			name: "strict ignores unlisted headers",
			m:    Mapping{Strict: true, Columns: map[string]Target{"Name": TargetTitle, "Labels (2)": TargetTags}},
			want: map[string]Target{
				"Summary": TargetIgnore, "Name": TargetTitle, "Labels": TargetIgnore, "Labels (2)": TargetTags,
				"Created": TargetIgnore, "Notes": TargetIgnore, "Status": TargetIgnore,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.Resolve(headers, "", nil); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Resolve = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTargets(t *testing.T) {
	for _, tgt := range Targets {
		if !tgt.Valid() {
			t.Errorf("%q is not valid", tgt)
		}
	}
	if key, ok := CustomTarget("points").CustomKey(); !ok || key != "points" {
		t.Errorf("CustomKey = %q, %v", key, ok)
	}
	if _, ok := TargetTitle.CustomKey(); ok {
		t.Error("title has a custom key")
	}
	if Target("nope").Valid() {
		t.Error("nope is valid")
	}
}
