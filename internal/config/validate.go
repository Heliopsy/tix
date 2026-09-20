package config

import (
	"net/url"
	"strings"

	"github.com/thereisnotime/tix/internal/core"
)

// Validate rejects configuration that cannot be used, naming the offending key.
func Validate(cfg *Config, sources map[string]Layer) error {
	checks := []struct {
		key   string
		value string
		set   []string
	}{
		{"auth.mode", cfg.Auth.Mode, AuthModes},
		{"hooks.mode", cfg.Hooks.Mode, HookModes},
		{"log.level", cfg.Log.Level, LogLevels},
		{"output.format", cfg.Output.Format, OutputFormats},
	}
	for _, check := range checks {
		if !allowed(check.value, check.set) {
			return invalidKey(check.key, sources).WithDetail("value", check.value).
				WithDetail("allowed", check.set)
		}
	}
	if cfg.Server.URL != "" {
		if _, err := url.Parse(cfg.Server.URL); err != nil {
			return invalidKey("server.url", sources).WithDetail("value", cfg.Server.URL)
		}
	}
	if strings.TrimSpace(cfg.Tenant) == "" {
		return invalidKey("tenant", sources)
	}
	if cfg.Retention.Audit < 0 || cfg.Retention.Events < 0 {
		return core.Invalid("retention durations must not be negative")
	}
	for name, ctx := range cfg.Contexts {
		if err := ctx.validate(name); err != nil {
			return err
		}
	}
	return nil
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
	if c.AuthMode != "" && !allowed(c.AuthMode, AuthModes) {
		return core.Invalid("context %q has an invalid auth_mode %q", name, c.AuthMode).
			WithDetail("allowed", AuthModes)
	}
	if c.HookMode != "" && !allowed(c.HookMode, HookModes) {
		return core.Invalid("context %q has an invalid hook_mode %q", name, c.HookMode).
			WithDetail("allowed", HookModes)
	}
	return nil
}
