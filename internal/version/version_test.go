// SPDX-License-Identifier: AGPL-3.0-or-later

package version

import (
	"runtime"
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
