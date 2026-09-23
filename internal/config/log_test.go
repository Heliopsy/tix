// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/logging"
)

func TestLogDefaultsAreTheOnesShipped(t *testing.T) {
	defaults := Defaults()
	cases := []struct {
		key  string
		got  string
		want string
		why  string
	}{
		{"log.level", defaults.Log.Level, "info", "an unattended server should say what it is doing without saying everything"},
		{"log.format", defaults.Log.Format, logging.FormatText, "a person reads the default, a machine asks for json"},
		{"log.output", defaults.Log.Output, logging.DestStderr, "a default that wrote files would create one nobody asked for"},
		{"log.file.max_age", defaults.Log.File.MaxAge.String(), (168 * time.Hour).String(), "a week still answers a Monday question about the weekend"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("%s default = %q, want %q: %s", tc.key, tc.got, tc.want, tc.why)
			}
		})
	}
	if defaults.Log.File.MaxSizeMB != 100 {
		t.Fatalf("log.file.max_size_mb default = %d, want 100", defaults.Log.File.MaxSizeMB)
	}
	if defaults.Log.File.MaxBackups != 7 {
		t.Fatalf("log.file.max_backups default = %d, want 7", defaults.Log.File.MaxBackups)
	}
	if defaults.Log.File.Compress {
		t.Fatal("log.file.compress should default to off; compression spends cpu on the machine already writing the log")
	}
	// The defaults exist to bound a disk. A change that lets them grow past
	// what deployment.md promises has to change that promise too.
	const budgetMB = 800
	if got := defaults.Log.File.MaxSizeMB * (defaults.Log.File.MaxBackups + 1); got != budgetMB {
		t.Fatalf("the default rotation budget is %d MiB, want the %d MiB the documentation promises", got, budgetMB)
	}
}

func TestLogKeysResolveFromEveryLayer(t *testing.T) {
	home := t.TempDir()
	body := "log:\n  level: warn\n  format: json\n  file:\n    max_backups: 3\n    compress: true\n"
	path := write(t, filepath.Join(home, "config.yaml"), body)
	env := map[string]string{
		"HOME":                     home,
		EnvConfigFile:              path,
		"TIX_LOG_FILE_MAX_AGE":     "24h",
		"TIX_LOG_FILE_MAX_SIZE_MB": "5",
	}
	got := mustLoad(t, Options{
		Dir: t.TempDir(), Home: home, Environ: environ(env),
		Flags: map[string]string{"log.output": "stdout"},
	})

	cases := []struct {
		key        string
		value      string
		wantSource Layer
	}{
		{"log.output", "stdout", LayerFlag},
		{"log.file.max_age", "24h0m0s", LayerEnv},
		{"log.file.max_size_mb", "5", LayerEnv},
		{"log.level", "warn", LayerFile},
		{"log.format", "json", LayerFile},
		{"log.file.max_backups", "3", LayerFile},
		{"log.file.compress", "true", LayerFile},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			key, ok := Lookup(tc.key)
			if !ok {
				t.Fatalf("key %q is not registered, so it is reachable from no layer at all", tc.key)
			}
			if value := key.Get(&got.Config); value != tc.value {
				t.Fatalf("%s = %q, want %q", tc.key, value, tc.value)
			}
			if src := got.Source(tc.key); src != tc.wantSource {
				t.Fatalf("%s source = %q, want %q", tc.key, src, tc.wantSource)
			}
		})
	}
}

func TestLogValidationRefusesUnusableSettings(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"unknown format", func(c *Config) { c.Log.Format = "logfmt" }, "log.format"},
		{"empty output", func(c *Config) { c.Log.Output = "  " }, "log.output"},
		{"zero rotation size", func(c *Config) {
			c.Log.Output = "/tmp/tix.log"
			c.Log.File.MaxSizeMB = 0
		}, "log.file.max_size_mb"},
		{"negative rotation size", func(c *Config) {
			c.Log.Output = "/tmp/tix.log"
			c.Log.File.MaxSizeMB = -1
		}, "log.file.max_size_mb"},
		{"negative backups", func(c *Config) {
			c.Log.Output = "/tmp/tix.log"
			c.Log.File.MaxBackups = -1
		}, "log.file.max_backups"},
		{"negative age", func(c *Config) {
			c.Log.Output = "/tmp/tix.log"
			c.Log.File.MaxAge = core.Duration(-time.Hour)
		}, "log.file.max_age"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults()
			tc.edit(&cfg)
			err := Validate(&cfg, map[string]Layer{})
			if err == nil {
				t.Fatalf("%s validated, want a refusal naming %s", tc.name, tc.want)
			}
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("err = %v, want an invalid-kind error", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to name %q", err, tc.want)
			}
		})
	}
}

// A stream destination has no file to bound, so rotation limits that would be
// refused for a path are left alone rather than forced on an operator who is
// not writing files at all.
func TestRotationLimitsAreIgnoredForAStreamDestination(t *testing.T) {
	for _, output := range []string{logging.DestStderr, logging.DestStdout} {
		t.Run(output, func(t *testing.T) {
			cfg := Defaults()
			cfg.Log.Output = output
			cfg.Log.File = LogFile{}
			if err := Validate(&cfg, map[string]Layer{}); err != nil {
				t.Fatalf("a %s destination with no rotation limits was refused: %v", output, err)
			}
		})
	}
}

// TestConfiguredRetentionMatchesTheShippedPolicy ties the retention defaults in
// this package to core.DefaultRetention, which is what a tenant falls back to
// when a window resolves to zero. Two sets of numbers for one promise is how a
// documented default stops being the real one.
func TestConfiguredRetentionMatchesTheShippedPolicy(t *testing.T) {
	configured := Defaults().Retention.Policy("acme")
	shipped := core.DefaultRetention("acme")
	cases := []struct {
		key        string
		configured core.Duration
		shipped    core.Duration
	}{
		{"retention.audit", configured.AuditEntries, shipped.AuditEntries},
		{"retention.events", configured.Events, shipped.Events},
		{"retention.webhook_deliveries", configured.WebhookDeliveries, shipped.WebhookDeliveries},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			if tc.configured != tc.shipped {
				t.Fatalf("%s defaults to %s but core.DefaultRetention uses %s; a window left at zero would silently change meaning",
					tc.key, tc.configured, tc.shipped)
			}
		})
	}
	// The whole point of the audit window is that it outlives the transport
	// buffer beside it, which the retention spec states as a requirement.
	if configured.AuditEntries <= configured.Events {
		t.Fatalf("retention.audit is %s and retention.events is %s; audit is the compliance record and must outlive the event buffer",
			configured.AuditEntries, configured.Events)
	}
	if configured.WebhookDeliveries > configured.AuditEntries {
		t.Fatalf("retention.webhook_deliveries is %s, longer than the %s audit window",
			configured.WebhookDeliveries, configured.AuditEntries)
	}
}
