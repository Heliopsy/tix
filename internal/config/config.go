// Package config resolves tix configuration from flags, environment
// variables, a .env file, configuration files and built-in defaults.
package config

import (
	"github.com/heliopsy/tix/internal/output"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/webhook"
)

// Config is the fully resolved tix configuration.
type Config struct {
	Tenant         string             `yaml:"tenant"`
	Project        string             `yaml:"project"`
	CurrentContext string             `yaml:"current_context"`
	Contexts       map[string]Context `yaml:"contexts,omitempty"`
	Database       Database           `yaml:"database"`
	Server         Server             `yaml:"server"`
	Auth           Auth               `yaml:"auth"`
	Hooks          Hooks              `yaml:"hooks"`
	Webhooks       Webhooks           `yaml:"webhooks"`
	Discovery      Discovery          `yaml:"discovery"`
	Retention      Retention          `yaml:"retention"`
	Log            Log                `yaml:"log"`
	Output         Output             `yaml:"output"`
	TUI            TUI                `yaml:"tui"`
}

// Database holds the local storage settings.
type Database struct {
	DSN string `yaml:"dsn" secret:"dsn"`
	// AllowNetworkFS overrides the refusal to open a SQLite database
	// detected on a network filesystem. The default is refuse, because that
	// placement corrupts a SQLite database. Set only when the operator has
	// accepted the risk.
	AllowNetworkFS bool `yaml:"allow_network_fs"`
}

// Server holds the remote endpoint and the listen address of `tix serve`.
type Server struct {
	URL    string `yaml:"url"`
	Listen string `yaml:"listen"`
	Token  string `yaml:"token" secret:"opaque"`
}

// Auth holds the authentication settings.
type Auth struct {
	Mode string `yaml:"mode"`
}

// Hooks holds the git hook settings.
type Hooks struct {
	Mode string `yaml:"mode"`
}

// Webhooks holds the outgoing webhook settings. It is unrelated to Hooks,
// which governs git hooks.
type Webhooks struct {
	DrainMode string `yaml:"drain_mode"`
}

// Discovery holds the per-directory context discovery settings.
type Discovery struct {
	Enabled   bool     `yaml:"enabled"`
	Filenames []string `yaml:"filenames"`
}

// Retention holds how long history is kept. Each class is governed
// independently, so changing one never disturbs another.
type Retention struct {
	Audit             core.Duration `yaml:"audit"`
	Events            core.Duration `yaml:"events"`
	WebhookDeliveries core.Duration `yaml:"webhook_deliveries"`
}

// Policy renders the configured windows as a retention policy for a tenant.
// It is the default a tenant without an explicit stored policy is pruned by.
func (r Retention) Policy(tenantID string) core.RetentionPolicy {
	return core.RetentionPolicy{
		TenantID:          tenantID,
		Events:            r.Events,
		AuditEntries:      r.Audit,
		WebhookDeliveries: r.WebhookDeliveries,
	}
}

// TUI holds the terminal interface settings. Keymap names a shipped
// keybinding scheme; Keys rebinds individual actions on top of it. Neither is
// validated here: the schemes live in internal/tui, which must not be imported
// from configuration, so `tix tui` refuses an unknown name at startup.
//
// Keys is a map, which the key walker does not enumerate, so it is settable
// from a configuration file rather than from a flag or an environment
// variable. One override per variable is not a shape an env var carries well.
type TUI struct {
	Keymap string            `yaml:"keymap"`
	Keys   map[string]string `yaml:"keys,omitempty"`
}

// Log holds the logging settings.
type Log struct {
	Level string `yaml:"level"`
}

// Output holds the rendering settings.
//
// TimeFormat and Timezone govern how an instant is shown to a person. They do
// not touch how an instant is stored or how a machine-readable format renders
// one: JSON, YAML and NDJSON always carry RFC 3339 in UTC, because a consumer
// parsing a timestamp must never have to guess which zone a deployment
// configured.
type Output struct {
	Format     string `yaml:"format"`
	Color      string `yaml:"color"`
	TimeFormat string `yaml:"time_format"`
	Timezone   string `yaml:"timezone"`
}

// Context is a named bundle of connection and identity settings.
type Context struct {
	Database string `yaml:"database,omitempty"`
	Server   string `yaml:"server,omitempty"`
	Token    string `yaml:"token,omitempty"`
	Tenant   string `yaml:"tenant,omitempty"`
	Project  string `yaml:"project,omitempty"`
	AuthMode string `yaml:"auth_mode,omitempty"`
	HookMode string `yaml:"hook_mode,omitempty"`
}

// Remote reports whether the context targets a server rather than a database.
func (c Context) Remote() bool { return strings.TrimSpace(c.Server) != "" }

// Default configuration values.
const (
	DefaultDSN      = "sqlite://~/.local/share/tix/tix.db"
	DefaultListen   = "127.0.0.1:8080"
	DefaultTenant   = "default"
	DefaultAuthMode = "token"
	DefaultHookMode = "off"
	// DefaultWebhookDrainMode names the process that delivers queued webhooks.
	DefaultWebhookDrainMode = string(webhook.DefaultMode)
	DefaultLogLevel         = "info"
	DefaultOutputFormat     = "table"
	// DefaultOutputColor colours a terminal and nothing else.
	DefaultOutputColor    = output.ColorAuto
	DefaultRetentionAudit = "8760h"
	DefaultRetentionEvent = "720h"
	// DefaultRetentionDelivery matches the shipped webhook delivery window.
	DefaultRetentionDelivery = "720h"
	// DefaultTUIKeymap names the keybinding scheme every install starts on.
	DefaultTUIKeymap = "default"
	// DefaultOutputTimeFormat is short and unambiguous: a date nobody reads
	// backwards and a time without a meridiem.
	DefaultOutputTimeFormat = "iso"
	// DefaultOutputTimezone follows the machine rather than imposing UTC on
	// somebody reading their own task list.
	DefaultOutputTimezone = "local"
)

// DefaultDiscoveryFilenames are the per-directory context files looked for.
var DefaultDiscoveryFilenames = []string{".tix.yaml", ".tix/config.yaml"}

// Allowed value sets for the enumerated keys.
var (
	// AuthModes lists the authentication modes this build implements.
	AuthModes = []string{"token"}
	// UnimplementedAuthModes are documented modes with no implementation
	// behind them. They are refused rather than accepted and ignored,
	// because a mode that reads as a security setting must never be silent.
	UnimplementedAuthModes = []string{"none", "oidc"}
	// HookModes lists the git hook modes this build implements.
	HookModes = []string{"off"}
	// UnimplementedHookModes are documented modes with no implementation.
	UnimplementedHookModes = []string{"warn", "enforce"}
	// WebhookDrainModes lists the webhook drain modes this build implements.
	WebhookDrainModes = drainModeNames()
	LogLevels         = []string{"debug", "info", "warn", "error"}
	// OutputFormats mirrors the formats the renderer actually implements, so
	// config cannot accept one it cannot render or reject one it can.
	OutputFormats = output.Formats
	// OutputColors mirrors the colour modes the renderer implements.
	OutputColors = output.ColorModes
	// OutputTimeFormats mirrors the named time layouts the renderer implements.
	OutputTimeFormats = output.TimeFormats
)

// Defaults returns the built-in configuration.
func Defaults() Config {
	return Config{
		Tenant:   DefaultTenant,
		Database: Database{DSN: DefaultDSN},
		Server:   Server{Listen: DefaultListen},
		Auth:     Auth{Mode: DefaultAuthMode},
		Hooks:    Hooks{Mode: DefaultHookMode},
		Webhooks: Webhooks{DrainMode: DefaultWebhookDrainMode},
		Discovery: Discovery{
			Enabled:   true,
			Filenames: append([]string(nil), DefaultDiscoveryFilenames...),
		},
		Retention: Retention{
			Audit:             mustDuration(DefaultRetentionAudit),
			Events:            mustDuration(DefaultRetentionEvent),
			WebhookDeliveries: mustDuration(DefaultRetentionDelivery),
		},
		Log: Log{Level: DefaultLogLevel},
		Output: Output{
			Format:     DefaultOutputFormat,
			Color:      DefaultOutputColor,
			TimeFormat: DefaultOutputTimeFormat,
			Timezone:   DefaultOutputTimezone,
		},
		TUI: TUI{Keymap: DefaultTUIKeymap},
	}
}

// drainModeNames renders every implemented drain mode as a string.
func drainModeNames() []string {
	modes := webhook.Modes()
	out := make([]string, 0, len(modes))
	for _, m := range modes {
		out = append(out, string(m))
	}
	return out
}
