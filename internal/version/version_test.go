// SPDX-License-Identifier: AGPL-3.0-or-later

package version

import (
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func TestStringContainsAllFields(t *testing.T) {
	got := String()
	for _, want := range []string{"tix", Version, Commit, Date, runtime.GOOS, runtime.GOARCH} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, missing %q", got, want)
		}
	}
}

func TestStringDefaults(t *testing.T) {
	if Version == "" || Commit == "" || Date == "" {
		t.Fatal("version variables must have non-empty defaults when not set via ldflags")
	}
}

// withVars runs fn against a clean set of placeholders and restores whatever
// the real build put there, so one case cannot leak into the next.
func withVars(t *testing.T, version, commit, date string, fn func()) {
	t.Helper()
	v, c, d := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = v, c, d })
	Version, Commit, Date = version, commit, date
	fn()
}

func buildInfo(mainVersion string, settings map[string]string) func() (*debug.BuildInfo, bool) {
	bi := &debug.BuildInfo{}
	bi.Main.Version = mainVersion
	for k, v := range settings {
		bi.Settings = append(bi.Settings, debug.BuildSetting{Key: k, Value: v})
	}
	return func() (*debug.BuildInfo, bool) { return bi, true }
}

func TestFillFromBuildInfo(t *testing.T) {
	const rev = "09c88a06a12ac4f61ae66fa35a009dc5ebd1032a"

	cases := []struct {
		name                    string
		read                    func() (*debug.BuildInfo, bool)
		version, commit, date   string
		wantVersion, wantCommit string
		wantDate                string
	}{
		{
			// The case this exists for: `go install github.com/heliopsy/tix@latest`.
			// No checkout, so no revision, but the version is exact.
			name:    "module install reports its release",
			read:    buildInfo("v0.1.0", nil),
			version: "dev", commit: "none", date: "unknown",
			wantVersion: "0.1.0", wantCommit: "none", wantDate: "unknown",
		},
		{
			// A pseudo-version names the release *after* the newest tag, so
			// taking it would claim a version that was never cut.
			name: "a build from a working tree keeps dev and names the commit",
			read: buildInfo("v0.1.1-0.20260923063127-09c88a06a12a", map[string]string{
				"vcs.revision": rev, "vcs.time": "2026-09-23T06:31:27Z", "vcs.modified": "false",
			}),
			version: "dev", commit: "none", date: "unknown",
			wantVersion: "dev", wantCommit: rev, wantDate: "2026-09-23T06:31:27Z",
		},
		{
			name: "an edited tree says so",
			read: buildInfo("(devel)", map[string]string{
				"vcs.revision": rev, "vcs.modified": "true",
			}),
			version: "dev", commit: "none", date: "unknown",
			wantVersion: "dev", wantCommit: rev + "-dirty", wantDate: "unknown",
		},
		{
			// GoReleaser already answered; the toolchain does not get to argue.
			name: "ldflags win",
			read: buildInfo("v9.9.9", map[string]string{
				"vcs.revision": rev, "vcs.time": "2026-09-23T06:31:27Z",
			}),
			version: "0.1.0", commit: "abc1234", date: "2026-01-01T00:00:00Z",
			wantVersion: "0.1.0", wantCommit: "abc1234", wantDate: "2026-01-01T00:00:00Z",
		},
		{
			name:    "no build info at all changes nothing",
			read:    func() (*debug.BuildInfo, bool) { return nil, false },
			version: "dev", commit: "none", date: "unknown",
			wantVersion: "dev", wantCommit: "none", wantDate: "unknown",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withVars(t, tc.version, tc.commit, tc.date, func() {
				fill(tc.read)
				if Version != tc.wantVersion {
					t.Errorf("Version = %q, want %q", Version, tc.wantVersion)
				}
				if Commit != tc.wantCommit {
					t.Errorf("Commit = %q, want %q", Commit, tc.wantCommit)
				}
				if Date != tc.wantDate {
					t.Errorf("Date = %q, want %q", Date, tc.wantDate)
				}
			})
		})
	}
}

func TestIsRelease(t *testing.T) {
	for v, want := range map[string]bool{
		"v0.1.0":                               true,
		"v1.2.3-rc.1":                          true,
		"v0.1.1-0.20260923063127-09c88a06a12a": false,
		"v0.1.1-0.20260923063127-09c88a06a12a+dirty": false,
		"(devel)": false,
		"":        false,
	} {
		if got := isRelease(v); got != want {
			t.Errorf("isRelease(%q) = %v, want %v", v, got, want)
		}
	}
}
