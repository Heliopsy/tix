// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// generatedSecret matches a freshly minted signing secret: base32 without
// padding over 32 bytes of entropy.
var generatedSecret = regexp.MustCompile(`[a-z2-7]{52}`)

// putWebhook registers an endpoint in one output format.
func putWebhook(t *testing.T, c *cli, url, format string) string {
	t.Helper()
	return c.mustRun("webhook", "put", url, "--event", "task.*", "-o", format).out
}

// A generated signing secret is the operator's only chance to configure the
// receiver, so every format must print it on creation.
func TestWebhookPutPrintsTheGeneratedSecret(t *testing.T) {
	for _, format := range []string{"table", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			c := newCLI(t)
			out := putWebhook(t, c, "https://hooks.example.com/tix", format)
			if !generatedSecret.MatchString(out) {
				t.Fatalf("%s output carries no generated secret:\n%s", format, out)
			}
			if !strings.Contains(strings.ToLower(out), "secret") {
				t.Fatalf("%s output does not label the secret:\n%s", format, out)
			}
		})
	}
}

// The secret is disclosed once. A later read must not carry it in any format,
// which is the regression that matters: yaml leaked it once already.
func TestWebhookSecretIsNeverShownAgain(t *testing.T) {
	c := newCLI(t)
	secret := secretFromJSON(t, putWebhook(t, c, "https://hooks.example.com/tix", "json"))
	if secret == "" {
		t.Fatal("webhook put printed no secret")
	}

	for _, format := range []string{"table", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			got := c.mustRun("webhook", "ls", "-o", format).out
			if strings.Contains(got, secret) {
				t.Fatalf("webhook ls leaked the secret in %s:\n%s", format, got)
			}
			if generatedSecret.MatchString(got) || strings.Contains(strings.ToLower(got), "secret") {
				t.Fatalf("webhook ls names a secret field in %s:\n%s", format, got)
			}
		})
	}
}

// An endpoint registered with a secret the operator already holds prints none.
func TestWebhookPutWithASuppliedSecretPrintsNothingBack(t *testing.T) {
	c := newCLI(t)
	const supplied = "operator-chosen-secret"
	got := c.mustRun("webhook", "put", "https://hooks.example.com/tix",
		"--secret", supplied, "-o", "json").out
	if strings.Contains(got, supplied) {
		t.Fatalf("webhook put echoed a supplied secret:\n%s", got)
	}
}

// secretFromJSON reads the secret out of a webhook put rendered as json.
func secretFromJSON(t *testing.T, out string) string {
	t.Helper()
	var payload struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("webhook put is not json: %v\n%s", err, out)
	}
	if payload.ID == "" {
		t.Fatalf("webhook put returned no identifier:\n%s", out)
	}
	return payload.Secret
}

// The drain mode is its own key, reported by config show and refused when the
// value is not one the dispatcher implements.
func TestWebhookDrainModeConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		environ    []string
		wantCode   int
		wantValue  string
		wantSource string
	}{
		{name: "default", wantCode: core.ExitOK, wantValue: "inline", wantSource: "default"},
		{
			name:       "environment wins",
			environ:    []string{"TIX_WEBHOOKS_DRAIN_MODE=server"},
			wantCode:   core.ExitOK,
			wantValue:  "server",
			wantSource: "environment",
		},
		{
			name:     "a bad value is invalid",
			environ:  []string{"TIX_WEBHOOKS_DRAIN_MODE=sometimes"},
			wantCode: core.ExitUsage,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			got := runEnv(c, tc.environ, "config", "show", "--sources", "-o", "json")
			if got.code != tc.wantCode {
				t.Fatalf("exit = %d, want %d\n%s", got.code, tc.wantCode, got.err)
			}
			if tc.wantCode != core.ExitOK {
				if !strings.Contains(got.err, "webhooks.drain_mode") {
					t.Fatalf("stderr = %q, want it to name the key", got.err)
				}
				return
			}
			entry := sourceEntry(t, got.out, "webhooks.drain_mode")
			if entry.Value != tc.wantValue || entry.Source != tc.wantSource {
				t.Fatalf("drain mode = %q from %q, want %q from %q",
					entry.Value, entry.Source, tc.wantValue, tc.wantSource)
			}
			if entry.Env != "TIX_WEBHOOKS_DRAIN_MODE" {
				t.Fatalf("env = %q, want TIX_WEBHOOKS_DRAIN_MODE", entry.Env)
			}
		})
	}
}

// sourceEntry picks one key out of config show --sources.
func sourceEntry(t *testing.T, out, key string) struct {
	Key    string `json:"key"`
	Env    string `json:"env"`
	Value  string `json:"value"`
	Source string `json:"source"`
} {
	t.Helper()
	var entries []struct {
		Key    string `json:"key"`
		Env    string `json:"env"`
		Value  string `json:"value"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("config show is not json: %v\n%s", err, out)
	}
	for _, e := range entries {
		if e.Key == key {
			return e
		}
	}
	t.Fatalf("config show does not report %q:\n%s", key, out)
	return entries[0]
}
