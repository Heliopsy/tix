package config

import (
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
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
		{"output.format", cfg.Output.Format, OutputFormats, nil},
		{"output.color", cfg.Output.Color, OutputColors, nil},
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
	if cfg.Server.URL != "" {
		if _, err := url.Parse(cfg.Server.URL); err != nil {
			return invalidKey("server.url", sources).WithDetail("value", cfg.Server.URL)
		}
	}
	if strings.TrimSpace(cfg.Tenant) == "" {
		return invalidKey("tenant", sources)
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
	for name, ctx := range cfg.Contexts {
		if err := ctx.validate(name); err != nil {
			return err
		}
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
