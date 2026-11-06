// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeChrome writes a stand-in browser that fails its first failures starts,
// as Chrome does on a loaded runner, and then writes DevToolsActivePort and
// waits like a real one. It records the profile each start was given.
const fakeChrome = `#!/bin/sh
dir=""
for a in "$@"; do case "$a" in --user-data-dir=*) dir="${a#--user-data-dir=}";; esac; done
n=$(cat "$FAKE_CHROME_COUNT" 2>/dev/null || echo 0)
n=$((n + 1))
echo "$n" > "$FAKE_CHROME_COUNT"
echo "$dir" >> "$FAKE_CHROME_PROFILES"
if [ "$n" -le "$FAKE_CHROME_FAILURES" ]; then
  if [ "$FAKE_CHROME_MODE" = hang ]; then exec sleep 30; fi
  echo "simulated crash $n" >&2
  exit 1
fi
printf '4242\n/devtools/browser/fake\n' > "$dir/DevToolsActivePort"
exec sleep 30
`

func writeFakeChrome(t *testing.T, failures, mode string) (chrome, profiles string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in browser is a shell script")
	}
	dir := t.TempDir()
	chrome = filepath.Join(dir, "chrome")
	if err := os.WriteFile(chrome, []byte(fakeChrome), 0o700); err != nil { // #nosec G306 -- it has to be executable
		t.Fatal(err)
	}
	profiles = filepath.Join(dir, "profiles")
	t.Setenv("FAKE_CHROME_COUNT", filepath.Join(dir, "count"))
	t.Setenv("FAKE_CHROME_PROFILES", profiles)
	t.Setenv("FAKE_CHROME_FAILURES", failures)
	t.Setenv("FAKE_CHROME_MODE", mode)
	return chrome, profiles
}

func startedProfiles(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path) // #nosec G304 -- the test's own temp file
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(raw))
}

func TestChromeLaunchRetriesWithAFreshProfile(t *testing.T) {
	for _, tc := range []struct{ name, mode string }{
		{"after it exits", "crash"},
		{"after it never writes the port", "hang"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chrome, profiles := writeFakeChrome(t, "2", tc.mode)
			proc, endpoint, err := startChromeRetrying(chrome, 3, 5*time.Second, t.Logf)
			if err != nil {
				t.Fatalf("the third start should have been used: %v", err)
			}
			defer proc.stop(0)
			if endpoint != "ws://127.0.0.1:4242/devtools/browser/fake" {
				t.Errorf("endpoint = %q", endpoint)
			}
			seen := startedProfiles(t, profiles)
			if len(seen) != 3 {
				t.Fatalf("started %d times, want 3: %v", len(seen), seen)
			}
			if seen[0] == seen[1] || seen[1] == seen[2] || seen[0] == seen[2] {
				t.Errorf("a retry reused a profile: %v", seen)
			}
			for _, failed := range seen[:2] {
				if _, err := os.Stat(failed); !os.IsNotExist(err) {
					t.Errorf("the failed start's profile %s was left behind", failed)
				}
			}
		})
	}
}

func TestChromeLaunchGivesUpWithTheBrowsersOwnWords(t *testing.T) {
	chrome, profiles := writeFakeChrome(t, "9", "crash")
	_, _, err := startChromeRetrying(chrome, 3, 5*time.Second, t.Logf)
	if err == nil {
		t.Fatal("a browser that never starts was reported as started")
	}
	for _, want := range []string{"3 attempts", "DevToolsActivePort", "simulated crash 3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	if n := len(startedProfiles(t, profiles)); n != 3 {
		t.Errorf("started %d times, want 3", n)
	}
}
