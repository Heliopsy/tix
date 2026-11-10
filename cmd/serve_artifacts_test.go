// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"maps"
	"slices"
	"testing"
)

func TestServeArtifactFlagsJoinTheFlagLayerOnlyWhenTyped(t *testing.T) {
	cmd := commandNamed(t, "serve")
	if got := serveConfigFlags(cmd); len(got) != 0 {
		t.Fatalf("untyped flags set %v, want nothing", got)
	}
	if err := cmd.Flags().Parse([]string{"--artifact-max-bytes", "5242880", "--artifact-allowed-types", "image/*,application/pdf"}); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"artifacts.max_bytes": "5242880", "artifacts.allowed_types": "image/*,application/pdf"}
	got := serveConfigFlags(cmd)
	if !maps.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	c := newCLI(t)
	g := &globals{environ: c.environ(), dir: c.home, configFlags: got}
	resolved, err := g.resolve()
	if err != nil {
		t.Fatal(err)
	}
	a := resolved.Config.Artifacts
	if a.MaxBytes != 5<<20 || !slices.Equal(a.AllowedTypes, []string{"image/*", "application/pdf"}) {
		t.Fatalf("resolved artifacts = %+v", a)
	}
}
