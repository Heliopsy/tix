// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestAnImportStageIsItsUploadersAloneAndExpires(t *testing.T) {
	s := newImportStages()
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	alice := &core.Actor{ID: "a", TenantID: "t"}
	st, err := s.put(alice, "infra", "f.csv", strings.NewReader("Title\nx\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		actor   *core.Actor
		project string
	}{
		{"another actor", &core.Actor{ID: "b", TenantID: "t"}, "infra"},
		{"another tenant", &core.Actor{ID: "a", TenantID: "u"}, "infra"},
		{"another project", alice, "ops"},
	} {
		if _, err := s.get(tc.actor, tc.project, st.ID); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("%s reached the stage: %v", tc.name, err)
		}
	}
	got, err := s.get(alice, "infra", st.ID)
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.open(got)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(f)
	_ = f.Close()
	if string(raw) != "Title\nx\n" {
		t.Errorf("stage holds %q", raw)
	}
	now = now.Add(importStageTTL + time.Minute)
	if _, err := s.get(alice, "infra", st.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("an expired stage was still served: %v", err)
	}
	if _, err := s.open(got); err == nil {
		t.Error("an expired stage's file is still on disk")
	}
}

func TestAnImportStageOpensNothingOutsideItsDirectory(t *testing.T) {
	s := newImportStages()
	if _, err := s.put(&core.Actor{ID: "a", TenantID: "t"}, "infra", "f.csv", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../../../../etc/passwd", "/etc/passwd"} {
		if f, err := s.open(&importStage{ID: id}); err == nil {
			_ = f.Close()
			t.Errorf("a stage named %q opened a file outside the staging directory", id)
		}
	}
}
