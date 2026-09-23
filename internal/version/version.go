// SPDX-License-Identifier: AGPL-3.0-or-later

// Package version holds build-time version information injected via ldflags,
// falling back to what the Go toolchain recorded when they were not.
package version

import (
	"fmt"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
)

// These variables are set at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Only GoReleaser passes the ldflags, so every other way of getting the binary
// used to report "tix dev (commit: none, built: unknown)". That includes
// `go install github.com/heliopsy/tix@latest`, which is the first install
// instruction we publish: people followed it and were told they had an
// unreleased build. The toolchain knows better in both cases and records it in
// the binary, so read it rather than leaving the question unanswered.
//
// The two sources differ in what they can say. A module install knows its
// version and nothing about the commit, because there is no checkout. A build
// from a working tree knows the revision, the time and whether the tree was
// dirty, but calls its version "(devel)". Each field is filled from whichever
// source has it, and anything neither knows keeps its placeholder.
func init() { fill(debug.ReadBuildInfo) }

func fill(read func() (*debug.BuildInfo, bool)) {
	info, ok := read()
	if !ok {
		return
	}

	if Version == "dev" && isRelease(info.Main.Version) {
		// GoReleaser passes 0.1.0; the module system says v0.1.0. Same build,
		// so it should not read differently depending on how it was installed.
		Version = strings.TrimPrefix(info.Main.Version, "v")
	}

	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if Commit == "none" && s.Value != "" {
				Commit = s.Value
			}
		case "vcs.time":
			if Date == "unknown" && s.Value != "" {
				Date = s.Value
			}
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}

	// A build from an edited tree is not the commit it names, and saying so is
	// the whole value of reporting the commit in a bug report.
	if dirty && Commit != "none" && !strings.HasSuffix(Commit, "-dirty") {
		Commit += "-dirty"
	}
}

// pseudo matches the version the module system invents for a commit that no
// tag points at: a timestamp and a short revision, appended to the tag after
// the one it follows.
var pseudo = regexp.MustCompile(`[0-9]{14}-[0-9a-f]{12}(\+[0-9A-Za-z.-]+)?$`)

// isRelease reports whether a recorded module version names an actual release.
//
// Building from a working tree yields a pseudo-version, and its numeric part
// is the release *after* the newest tag: a local build of v0.1.0 calls itself
// v0.1.1-0.20260923063127-09c88a06a12a+dirty. Reporting that would be worse
// than reporting nothing, because it names a version that was never released.
// Such a build keeps "dev" and says which commit it is instead, which is what
// a bug report needs from it anyway.
func isRelease(v string) bool {
	return v != "" && v != "(devel)" && !pseudo.MatchString(v)
}

// String returns a formatted version string.
func String() string {
	return fmt.Sprintf("tix %s (commit: %s, built: %s, %s/%s)",
		Version, Commit, Date, runtime.GOOS, runtime.GOARCH)
}
