// SPDX-License-Identifier: AGPL-3.0-or-later

package notices_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/notices"
)

const isc = `Copyright (c) 2025 Coder

Permission to use, copy, modify, and distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES`

const apache = `
                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/`

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func goroot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Skipf("no go tool: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestIdentify(t *testing.T) {
	for want, text := range map[string]string{
		"MIT":           read(t, "../../LICENSES/MIT.txt"),
		"BSD-2-Clause":  read(t, "../../LICENSES/BSD-2-Clause.txt"),
		"0BSD":          read(t, "../../LICENSES/0BSD.txt"),
		"AGPL-3.0-only": read(t, "../../LICENSE"),
		"BSD-3-Clause":  read(t, filepath.Join(goroot(t), "LICENSE")),
		"ISC":           isc,
		"Apache-2.0":    apache,
	} {
		got, err := notices.Identify(text)
		if err != nil || got != want {
			t.Errorf("Identify(%s text) = %q, %v", want, got, err)
		}
	}
	for name, text := range map[string]string{
		"unknown":   "All rights reserved. Do what the author says.",
		"ambiguous": read(t, "../../LICENSES/MIT.txt") + apache,
	} {
		if got, err := notices.Identify(text); err == nil {
			t.Errorf("%s text identified as %q", name, got)
		}
	}
}

func TestFromDir(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := notices.FromDir("example.com/none", "v1", dir); err == nil || !strings.Contains(err.Error(), "no licence file") {
		t.Fatalf("a module without a licence file gave %v", err)
	}
	write("LICENSE.txt", isc)
	write("NOTICE", "Example notice")
	write("README.md", "not a notice")
	mod, err := notices.FromDir("example.com/m", "v1.2.3", dir)
	if err != nil {
		t.Fatal(err)
	}
	if mod.License != "ISC" || mod.LicenseText != isc || mod.Version != "v1.2.3" {
		t.Fatalf("module %+v", mod)
	}
	if len(mod.Other) != 1 || mod.Other[0].Name != "NOTICE" || mod.Other[0].Text != "Example notice" {
		t.Fatalf("other files %+v, want the NOTICE alone", mod.Other)
	}
	write("LICENSE.txt", "Proprietary.")
	if _, err := notices.FromDir("example.com/m", "v1", dir); err == nil || !strings.Contains(err.Error(), "example.com/m") {
		t.Fatalf("an unidentified licence gave %v", err)
	}
}

func TestCollectNamesEveryModuleTheLibraryLinks(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go tool")
	}
	report, err := notices.Collect(context.Background(), notices.MainModule+"/embedded/libtix",
		notices.Target{GOOS: "android", GOARCH: "arm64"}, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if report.Target != "android/arm64" {
		t.Fatalf("target %q", report.Target)
	}
	byPath := map[string]notices.Module{}
	for _, m := range report.Modules {
		if m.License == "" || m.LicenseText == "" {
			t.Errorf("module %s has no licence", m.Path)
		}
		byPath[m.Path] = m
	}
	if m := byPath["modernc.org/sqlite"]; m.License != "BSD-3-Clause" || m.Version == "" {
		t.Errorf("modernc.org/sqlite: %+v", m.Path+" "+m.Version+" "+m.License)
	}
	if m := byPath["std"]; m.License != "BSD-3-Clause" || !strings.HasPrefix(m.Version, "go") {
		t.Errorf("std: %s %s", m.Version, m.License)
	}
	main := byPath[notices.MainModule]
	if main.License != "AGPL-3.0-or-later" || main.Version != "1.2.3" {
		t.Errorf("tix: %s %s", main.Version, main.License)
	}
	var reuse bool
	for _, f := range main.Other {
		reuse = reuse || f.Name == "REUSE.toml" && strings.Contains(f.Text, "htmx.min.js")
	}
	if !reuse {
		t.Error("the tix entry does not carry REUSE.toml, which names the vendored assets")
	}
	if _, ok := byPath["github.com/spf13/cobra"]; ok {
		t.Error("the library's notices list cobra, which only the tix binary links")
	}
}

func TestCollectReportsAListingFailure(t *testing.T) {
	if _, err := notices.Collect(context.Background(), "example.invalid/nothing", notices.Target{GOOS: "linux", GOARCH: "amd64"}, ""); err == nil {
		t.Fatal("listing a package that does not exist succeeded")
	}
}
