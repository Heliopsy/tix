// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"slices"
	"strings"
	"testing"
)

func TestArtifactLimitsCheck(t *testing.T) {
	l := ArtifactLimits{MaxBytes: 10, AllowedTypes: []string{"image/*", "text/plain"}}
	tests := []struct {
		name, ct string
		size     int
		want     string
	}{
		{"no blob", "text/html", 0, ""},
		{"at limit", "text/plain; charset=utf-8", 10, ""},
		{"pattern", "image/PNG", 3, ""},
		{"over limit", "text/plain", 11, "over the limit of 10 bytes"},
		{"html refused", "text/html", 3, `"text/html" is not accepted on this tenant; accepted types: image/*, text/plain`},
		{"untyped refused", "", 3, "application/octet-stream"},
		{"garbage refused", "not a type;;", 3, "not accepted"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := l.Check(tc.ct, make([]byte, tc.size))
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("want accepted, got %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Fatalf("want an error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestEffectiveArtifactLimits(t *testing.T) {
	server := []string{"image/png", "image/jpeg", "application/pdf"}
	tests := []struct {
		name      string
		tenant    Tenant
		wantMax   int64
		wantTypes []string
	}{
		{"inherit", Tenant{}, 100, server},
		{"tighter", Tenant{ArtifactMaxBytes: 40, ArtifactTypes: []string{"application/pdf"}}, 40, []string{"application/pdf"}},
		{"server lowered since", Tenant{ArtifactMaxBytes: 400}, 100, server},
		{"pattern narrowed to the server's", Tenant{ArtifactTypes: []string{"image/*"}}, 100, []string{"image/png", "image/jpeg"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EffectiveArtifactLimits(100, server, tc.tenant)
			if got.MaxBytes != tc.wantMax || !slices.Equal(got.AllowedTypes, tc.wantTypes) || got.ServerMaxBytes != 100 {
				t.Fatalf("got %+v, want max %d types %v", got, tc.wantMax, tc.wantTypes)
			}
		})
	}
}

func TestParseArtifactTypes(t *testing.T) {
	got, err := ParseArtifactTypes([]string{" Image/* ", "", "text/plain", "text/plain", "*/*"})
	if err != nil || !slices.Equal(got, []string{"image/*", "text/plain", "*/*"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, bad := range []string{"text", "*/plain", "te*t/plain", "text/", "a/b c"} {
		if _, err := ParseArtifactTypes([]string{bad}); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestByteSizes(t *testing.T) {
	for in, want := range map[string]int64{"1048576": 1 << 20, "512KiB": 512 << 10, "5 MiB": 5 << 20, "2m": 2 << 20, "1GB": 1 << 30} {
		if got, err := ParseByteSize(in); err != nil || got != want {
			t.Errorf("ParseByteSize(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "x", "-1", "5TB"} {
		if _, err := ParseByteSize(bad); err == nil {
			t.Errorf("ParseByteSize(%q) should fail", bad)
		}
	}
	for n, want := range map[int64]string{1 << 20: "1 MiB", 1536: "1.5 KiB", 12: "12 bytes"} {
		if got := ByteSize(n); got != want {
			t.Errorf("ByteSize(%d) = %q, want %q", n, got, want)
		}
	}
	if (ArtifactLimits{AllowedTypes: []string{"*/*"}}).Accept() != "" {
		t.Error("everything allowed should leave accept empty")
	}
}
