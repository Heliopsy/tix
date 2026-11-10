// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestTenantArtifactLimitsOnlyTighten(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	l.artifactMaxBytes = 1000
	l.artifactTypes = []string{"image/*", "text/plain", "application/pdf"}

	tests := []struct {
		name string
		in   core.UpdateTenantInput
		want string
	}{
		{"size above the server", core.UpdateTenantInput{ArtifactMaxBytes: sizeOf(1001)},
			"above this server's limit of 1000 bytes"},
		{"type the server refuses", core.UpdateTenantInput{ArtifactTypes: &[]string{"text/html"}},
			`"text/html" is not accepted by this server, which accepts: image/*, text/plain, application/pdf`},
		{"pattern wider than the server", core.UpdateTenantInput{ArtifactTypes: &[]string{"text/*"}}, "text/*"},
		{"negative size", core.UpdateTenantInput{ArtifactMaxBytes: sizeOf(-1)}, "negative"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := l.UpdateTenant(ctx, "", tc.in)
			if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want invalid containing %q", err, tc.want)
			}
		})
	}

	got, err := l.UpdateTenant(ctx, "", core.UpdateTenantInput{
		ArtifactMaxBytes: sizeOf(10), ArtifactTypes: &[]string{"image/png", " application/PDF "}})
	if err != nil {
		t.Fatal(err)
	}
	if got.ArtifactMaxBytes != 10 || !slices.Equal(got.ArtifactTypes, []string{"image/png", "application/pdf"}) {
		t.Fatalf("tenant = %d %v", got.ArtifactMaxBytes, got.ArtifactTypes)
	}
	limits, err := l.ArtifactLimits(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if limits.MaxBytes != 10 || limits.ServerMaxBytes != 1000 || len(limits.AllowedTypes) != 2 {
		t.Fatalf("limits = %+v", limits)
	}

	if _, err := l.UpdateTenant(ctx, "", core.UpdateTenantInput{
		ArtifactMaxBytes: sizeOf(0), ArtifactTypes: &[]string{}}); err != nil {
		t.Fatal(err)
	}
	if limits, _ = l.ArtifactLimits(ctx); limits.MaxBytes != 1000 || len(limits.AllowedTypes) != 3 {
		t.Fatalf("cleared limits should inherit, got %+v", limits)
	}
}

func TestPutArtifactHonoursTheTenantLimits(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "t"})
	ref := core.TaskRef{ID: task.ID}
	put := func(ct string, n int) error {
		_, err := l.PutArtifact(ctx, ref, core.ArtifactInput{Kind: core.ArtifactFile, ContentType: ct, Blob: make([]byte, n)})
		return err
	}

	if err := put("text/plain", 20); err != nil {
		t.Fatalf("an artifact within the defaults: %v", err)
	}
	if err := put("text/html", 20); err == nil || !strings.Contains(err.Error(), "accepted types: image/png") {
		t.Fatalf("html under the defaults = %v, want refused naming the accepted types", err)
	}
	if _, err := l.UpdateTenant(ctx, "", core.UpdateTenantInput{
		ArtifactMaxBytes: sizeOf(10), ArtifactTypes: &[]string{"application/pdf"}}); err != nil {
		t.Fatal(err)
	}
	if err := put("application/pdf", 11); err == nil || !strings.Contains(err.Error(), "over the limit of 10 bytes") {
		t.Fatalf("over the tenant limit = %v", err)
	}
	if err := put("text/plain", 5); err == nil || !strings.Contains(err.Error(), "accepted types: application/pdf") {
		t.Fatalf("type outside the tenant list = %v", err)
	}
	if err := put("application/pdf", 10); err != nil {
		t.Fatalf("within the tenant limits: %v", err)
	}
	if _, err := l.PutArtifact(ctx, ref, core.ArtifactInput{Kind: core.ArtifactResult, ContentType: "text/html"}); err != nil {
		t.Fatalf("no blob is never judged: %v", err)
	}
	stored, err := l.ListArtifacts(ctx, ref)
	if err != nil || len(stored) != 3 || len(stored[0].Blob) != 20 {
		t.Fatalf("existing artifacts must stay untouched, got %d, %v", len(stored), err)
	}
}

func sizeOf(n int64) *int64 { return &n }
