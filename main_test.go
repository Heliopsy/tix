// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/changelog"
)

// TestTheBinaryCarriesItsOwnZoneDatabase guards the import, not the host. The
// host happens to have tzdata, so LoadLocation succeeds here either way; what
// this pins is that the shipped binary does not need it, because a base image
// without one turns a reader's timezone into the deployment default with no
// visible failure.
func TestTheBinaryCarriesItsOwnZoneDatabase(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing main.go: %v", err)
	}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if path == "time/tzdata" {
			if _, err := time.LoadLocation("Asia/Tokyo"); err != nil {
				t.Fatalf("loading a zone with the embedded database: %v", err)
			}
			return
		}
	}
	t.Fatal("main.go does not import time/tzdata; the binary depends on the host having a zone database")
}

// TestTheBinaryCarriesItsChangelog pins both halves: the embedded file parses
// into releases, and main hands it to the package the release screen reads.
func TestTheBinaryCarriesItsChangelog(t *testing.T) {
	t.Parallel()
	if got := changelog.Parse(changelogMD); len(got) == 0 {
		t.Fatal("the embedded CHANGELOG.md has no release entries")
	}
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	if !strings.Contains(string(src), "changelog.Source = changelogMD") {
		t.Fatal("main.go does not hand the embedded changelog to internal/changelog")
	}
}
