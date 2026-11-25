// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/heliopsy/tix/internal/notices"
)

func TestRunWritesTheReport(t *testing.T) {
	out := filepath.Join(t.TempDir(), "lib", "NOTICES.json")
	if err := run([]string{"-pkg", notices.MainModule + "/embedded/libtix", "-goos", "linux", "-goarch", "arm64",
		"-version", "9.9.9", "-o", out}); err != nil {
		t.Fatal(err)
	}
	var report notices.Report
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatal(err)
	}
	if report.Target != "linux/arm64" || len(report.Modules) < 10 {
		t.Fatalf("report for %s with %d modules", report.Target, len(report.Modules))
	}
	if err := run([]string{"-no-such-flag"}); err == nil {
		t.Fatal("an unknown flag was accepted")
	}
	if err := run([]string{"-pkg", "example.invalid/x", "-o", out}); err == nil {
		t.Fatal("a missing package was accepted")
	}
}
