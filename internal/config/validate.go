// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/logging"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/webhook"
)

// Validate rejects configuration that cannot be used, naming the offending key.
func Validate(cfg *Config, sources map[string]Layer) error {
	checks := []struct {
		key           string
		value         string
		set           []string
		unimplemented []string
	}{
		{"auth.mode", cfg.Auth.Mode, AuthModes, UnimplementedAuthModes},
		{"hooks.mode", cfg.Hooks.Mode, HookModes, UnimplementedHookModes},
		{"log.level", cfg.Log.Level, LogLevels, nil},
		{"log.format", cfg.Log.Format, LogFormats, nil},
		{"output.format", cfg.Output.Format, OutputFormats, nil},
		{"output.color", cfg.Output.Color, OutputColors, nil},
		{"output.time_format", cfg.Output.TimeFormat, OutputTimeFormats, nil},
		{"server.cookie_security", cfg.Server.CookieSecurity, CookieSecurities, nil},
	}
	for _, check := range checks {
		if allowed(check.value, check.set) {
			continue
		}
		if allowed(check.value, check.unimplemented) {
			return unimplementedKey(check.key, check.value, sources).
				WithDetail("supported", check.set)
		}
		return invalidKey(check.key, sources).WithDetail("value", check.value).
			WithDetail("allowed", check.set)
	}
	if _, err := webhook.ParseMode(cfg.Webhooks.DrainMode); err != nil {
		return invalidKey("webhooks.drain_mode", sources).
			WithDetail("value", cfg.Webhooks.DrainMode).
			WithDetail("allowed", WebhookDrainModes)
	}
	// A timezone is not an enumeration, so it is validated by resolving it
	// rather than by membership. A name the system cannot load would otherwise
	// only fail much later, while rendering something.
	if _, err := output.NewTimeStyle(cfg.Output.TimeFormat, cfg.Output.Timezone); err != nil {
		return invalidKey("output.timezone", sources).WithDetail("value", cfg.Output.Timezone).
			WithDetail("reason", err.Error())
	}
	// A trusted proxy list decides whether a forwarded header is believed, so
	// an address that does not parse must fail at startup rather than quietly
	// trust nothing.
	if _, err := auth.NewProxyPolicy(cfg.Server.TrustedProxies); err != nil {
		return invalidKey("server.trusted_proxies", sources).
			WithDetail("value", strings.Join(cfg.Server.TrustedProxies, ",")).
			WithDetail("reason", err.Error())
	}
	if cfg.Server.URL != "" {
		if _, err := url.Parse(cfg.Server.URL); err != nil {
			return invalidKey("server.url", sources).WithDetail("value", cfg.Server.URL)
		}
	}
	if strings.TrimSpace(cfg.Tenant) == "" {
		return invalidKey("tenant", sources)
	}
	// A connect timeout of zero would not mean "wait forever" but "give up
	// immediately", which no operator writes on purpose, so it is refused here
	// rather than turned into an unreachable database at startup.
	if cfg.Database.ConnectTimeout <= 0 {
		return invalidKey("database.connect_timeout", sources).
			WithDetail("value", cfg.Database.ConnectTimeout.String()).
			WithDetail("reason", "a database connect timeout must be positive")
	}
	if err := validateLog(cfg, sources); err != nil {
		return err
	}
	for _, window := range []struct {
		key string
		d   core.Duration
	}{
		{"retention.audit", cfg.Retention.Audit},
		{"retention.events", cfg.Retention.Events},
		{"retention.webhook_deliveries", cfg.Retention.WebhookDeliveries},
	} {
		if window.d < 0 {
			return invalidKey(window.key, sources).WithDetail("value", window.d.String()).
				WithDetail("reason", "retention windows must not be negative")
		}
	}
	// The ssh listener's limits are numbers rather than enumerations, so what
	// can be wrong about one is its sign: a negative window or a negative cap
	// would otherwise be accepted here and quietly replaced by a default much
	// later, which is the same silence a security setting must never have.
	for _, window := range []struct {
		key string
		d   core.Duration
	}{
		{"ssh.tenant_ttl", cfg.SSH.TenantTTL},
		{"ssh.reap_interval", cfg.SSH.ReapInterval},
		{"ssh.lease_ttl", cfg.SSH.LeaseTTL},
		{"ssh.idle_timeout", cfg.SSH.IdleTimeout},
		{"ssh.keepalive_interval", cfg.SSH.KeepaliveInterval},
	} {
		if window.d < 0 {
			return invalidKey(window.key, sources).WithDetail("value", window.d.String()).
				WithDetail("reason", "an ssh duration must not be negative")
		}
	}
	for _, limit := range []struct {
		key string
		n   int
	}{
		{"ssh.max_tenants", cfg.SSH.MaxTenants},
		{"ssh.max_tasks", cfg.SSH.MaxTasks},
		{"ssh.rate_per_hour", cfg.SSH.RatePerHour},
		{"ssh.rate_burst", cfg.SSH.RateBurst},
		{"ssh.keepalive_max_missed", cfg.SSH.KeepaliveMaxMissed},
		{"ssh.max_sessions_per_key", cfg.SSH.MaxSessionsPerKey},
		{"ssh.max_sessions", cfg.SSH.MaxSessions},
	} {
		if limit.n < 0 {
			return invalidKey(limit.key, sources).WithDetail("value", strconv.Itoa(limit.n)).
				WithDetail("reason", "an ssh limit must not be negative")
		}
	}
	// A cap on one key that exceeds the cap on the listener is not wrong so
	// much as unreachable, and an operator who wrote it meant something else.
	if cfg.SSH.MaxSessions > 0 && cfg.SSH.MaxSessionsPerKey > cfg.SSH.MaxSessions {
		return invalidKey("ssh.max_sessions_per_key", sources).
			WithDetail("value", strconv.Itoa(cfg.SSH.MaxSessionsPerKey)).
			WithDetail("reason", "one key may not be allowed more sessions than the whole listener")
	}
	for name, ctx := range cfg.Contexts {
		if err := ctx.validate(name); err != nil {
			return err
		}
	}
	return nil
}

// validateLog refuses a destination that cannot be written and rotation limits
// that would either never rotate or never keep anything. A file destination
// that turns out to be undeliverable at the first record is a server that
// started and then logged nothing, so the path is opened at startup instead.
func validateLog(cfg *Config, sources map[string]Layer) error {
	if strings.TrimSpace(cfg.Log.Output) == "" {
		return invalidKey("log.output", sources).WithDetail("value", cfg.Log.Output).
			WithDetail("reason", "name stderr, stdout or a file path")
	}
	if logging.Stream(cfg.Log.Output) {
		return nil
	}
	if cfg.Log.File.MaxSizeMB <= 0 {
		return invalidKey("log.file.max_size_mb", sources).
			WithDetail("value", strconv.Itoa(cfg.Log.File.MaxSizeMB)).
			WithDetail("reason", "a rotation size must be positive; zero would rotate on every record")
	}
	if cfg.Log.File.MaxBackups < 0 {
		return invalidKey("log.file.max_backups", sources).
			WithDetail("value", strconv.Itoa(cfg.Log.File.MaxBackups)).
			WithDetail("reason", "a backup count must not be negative")
	}
	if cfg.Log.File.MaxAge < 0 {
		return invalidKey("log.file.max_age", sources).
			WithDetail("value", cfg.Log.File.MaxAge.String()).
			WithDetail("reason", "a log retention window must not be negative")
	}
	return nil
}

// unimplementedKey refuses a value this build documents but does not implement.
func unimplementedKey(key, value string, sources map[string]Layer) *core.Error {
	layer := sources[key]
	if layer == "" {
		layer = LayerDefault
	}
	err := core.Invalid("%s %q is not implemented by this build; supported: %s (set from the %s layer, %s)",
		key, value, strings.Join(supportedFor(key), ", "), layer, EnvName(key))
	return err.WithDetail("key", key).WithDetail("value", value).WithDetail("layer", string(layer))
}

// supportedFor returns the implemented value set of an enumerated key.
func supportedFor(key string) []string {
	switch key {
	case "auth.mode":
		return AuthModes
	case "hooks.mode":
		return HookModes
	default:
		return nil
	}
}

func invalidKey(key string, sources map[string]Layer) *core.Error {
	layer := sources[key]
	if layer == "" {
		layer = LayerDefault
	}
	err := core.Invalid("invalid value for %q from the %s layer (%s)", key, layer, EnvName(key))
	return err.WithDetail("key", key).WithDetail("layer", string(layer))
}

func (c Context) validate(name string) error {
	hasDSN := strings.TrimSpace(c.Database) != ""
	hasURL := strings.TrimSpace(c.Server) != ""
	if hasDSN && hasURL {
		return core.Invalid("context %q sets both a database dsn and a server url", name).
			WithDetail("context", name)
	}
	for _, field := range []struct {
		name          string
		value         string
		set           []string
		unimplemented []string
	}{
		{"auth_mode", c.AuthMode, AuthModes, UnimplementedAuthModes},
		{"hook_mode", c.HookMode, HookModes, UnimplementedHookModes},
	} {
		if field.value == "" || allowed(field.value, field.set) {
			continue
		}
		if allowed(field.value, field.unimplemented) {
			return core.Invalid("context %q sets %s %q, which is not implemented by this build; supported: %s",
				name, field.name, field.value, strings.Join(field.set, ", ")).
				WithDetail("context", name).WithDetail("supported", field.set)
		}
		return core.Invalid("context %q has an invalid %s %q", name, field.name, field.value).
			WithDetail("allowed", field.set)
	}
	return nil
}
