// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The command line defaulted to no expiry while the browser form proposed
// ninety days: one product handing out an immortal credential on one surface
// and a bounded one on the other. Both are ninety days now, and `never` is the
// only way to get a token that does not expire.

// createdToken is what `token create -o json` prints, of which only the expiry
// matters here.
type createdToken struct {
	ExpiresAt *time.Time `json:"expires_at"`
}

// mintToken runs `token create` with the given extra flags and returns the
// expiry the service stored.
func mintToken(t *testing.T, name string, extra ...string) *time.Time {
	t.Helper()
	host := newCLI(t)
	args := append([]string{"token", "create", name, "--scope", "task:read", "-o", "json"}, extra...)
	out := host.mustRun(args...).out
	var got createdToken
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("token create: %v (%s)", err, out)
	}
	return got.ExpiresAt
}

// TestTokenCreateDefaultsToNinetyDays is the breaking change itself. A script
// that minted an immortal token by saying nothing now gets a bounded one.
func TestTokenCreateDefaultsToNinetyDays(t *testing.T) {
	before := time.Now().UTC()
	got := mintToken(t, "defaulted")
	if got == nil {
		t.Fatalf("token create with no --expires minted a token that never expires")
	}
	// The day, not the instant: the expiry is counted from the wall clock on
	// the way through, so only the date is predictable.
	want := before.AddDate(0, 0, 90).Format("2006-01-02")
	if day := got.UTC().Format("2006-01-02"); day != want {
		t.Errorf("the default expiry falls on %s, want %s (ninety days)", day, want)
	}
}

// TestTokenCreateExpiresNeverStillMintsAnImmortalToken is the escape hatch,
// and the only one.
func TestTokenCreateExpiresNeverStillMintsAnImmortalToken(t *testing.T) {
	if got := mintToken(t, "immortal", "--expires", "never"); got != nil {
		t.Errorf("--expires never minted a token expiring at %s", got)
	}
}

// TestTokenCreateStillTakesAnAbsoluteExpiry keeps the grammar that existed
// before the default did: a script passing a date is unaffected.
func TestTokenCreateStillTakesAnAbsoluteExpiry(t *testing.T) {
	got := mintToken(t, "dated", "--expires", "2027-01-01")
	if got == nil {
		t.Fatalf("--expires 2027-01-01 minted a token that never expires")
	}
	if day := got.UTC().Format("2006-01-02"); day != "2027-01-01" {
		t.Errorf("the expiry falls on %s, want 2027-01-01", day)
	}
}

// TestTokenCreateHelpStatesTheDefaultPlainly reads the help text, because a
// default that changed behaviour for every existing caller has to be visible
// without reading the changelog. It reads the one flag's line, not the whole
// help: the word "90d" also appears in the long description above it, which is
// exactly how a wider read would pass with the flag's own text stale.
func TestTokenCreateHelpStatesTheDefaultPlainly(t *testing.T) {
	host := newCLI(t)
	out := host.mustRun("token", "create", "--help").out
	_, rest, ok := strings.Cut(out, "--expires string")
	if !ok {
		t.Fatalf("token create --help does not list --expires:\n%s", out)
	}
	line, _, _ := strings.Cut(rest, "--project")
	if !strings.Contains(line, `(default "90d")`) {
		t.Errorf("the --expires help does not state its default:\n%s", line)
	}
	if !strings.Contains(line, "never") {
		t.Errorf("the --expires help does not name the way to opt out:\n%s", line)
	}
}

func TestParseExpiry(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		value string
		want  string
		none  bool
		fails bool
	}{
		{name: "omitted takes the default", value: "", want: "2026-05-30"},
		{name: "never has no expiry", value: "never", none: true},
		{name: "never is case insensitive", value: "Never", none: true},
		{name: "a duration counts from now", value: "7d", want: "2026-03-08"},
		{name: "the default spelled out", value: "90d", want: "2026-05-30"},
		{name: "a date is absolute", value: "2027-01-01", want: "2027-01-01"},
		{name: "a timestamp is absolute", value: "2027-01-01T06:00:00Z", want: "2027-01-01"},
		{name: "nonsense is refused", value: "soonish", fails: true},
		{name: "a past duration is refused", value: "-5d", fails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseExpiry(tc.value, now)
			switch {
			case tc.fails:
				if err == nil {
					t.Fatalf("parseExpiry(%q) = %v, want a refusal", tc.value, got)
				}
				return
			case err != nil:
				t.Fatalf("parseExpiry(%q): %v", tc.value, err)
			case tc.none:
				if got != nil {
					t.Fatalf("parseExpiry(%q) = %s, want no expiry", tc.value, got)
				}
				return
			case got == nil:
				t.Fatalf("parseExpiry(%q) = no expiry, want %s", tc.value, tc.want)
			}
			if day := got.UTC().Format("2006-01-02"); day != tc.want {
				t.Errorf("parseExpiry(%q) falls on %s, want %s", tc.value, day, tc.want)
			}
		})
	}
}
