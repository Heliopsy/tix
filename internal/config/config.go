// SPDX-License-Identifier: AGPL-3.0-or-later

// Package config resolves tix configuration from flags, environment
// variables, a .env file, configuration files and built-in defaults.
package config

import (
	"github.com/heliopsy/tix/internal/output"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/logging"
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
	SSH            SSH                `yaml:"ssh"`
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
	// ConnectTimeout bounds the reachability check a PostgreSQL target makes
	// before the process will serve anything. It is a key rather than a
	// constant because the right wait is a property of the deployment: a
	// database still starting beside this process needs longer than the
	// default, and a process behind a supervisor that retries wants shorter.
	// SQLite ignores it, having no connection to wait for.
	ConnectTimeout core.Duration `yaml:"connect_timeout"`
}

// Server holds the remote endpoint and the listen address of `tix serve`.
type Server struct {
	URL    string `yaml:"url"`
	Listen string `yaml:"listen"`
	Token  string `yaml:"token" secret:"opaque"`

	// TrustedProxies lists the reverse proxies, as IPs or CIDR blocks, whose
	// X-Forwarded-Proto and X-Forwarded-For are believed. Any client can send
	// those headers, so an empty list believes neither from anybody.
	TrustedProxies []string `yaml:"trusted_proxies,omitempty"`

	// CookieSecurity decides the Secure flag on the session and CSRF cookies.
	// The default derives it from the scheme the client used, which behind a
	// TLS-terminating proxy is the proxy's, not this process's.
	CookieSecurity string `yaml:"cookie_security"`
}

// SSH holds the settings of the listener `tix ssh` runs: where it binds, the
// identity it presents, and the limits that keep a listener strangers reach
// from becoming an availability problem.
//
// Every one of them is a key rather than only a flag, because the deployment
// most likely to run this listener is a container, where a command line is the
// hardest layer to reach and an environment variable the easiest.
type SSH struct {
	Listen  string `yaml:"listen"`
	HostKey string `yaml:"host_key"`
	// AllowPublic permits binding a non-loopback address, which is the same
	// explicit choice `tix serve` demands before it faces a network.
	AllowPublic bool `yaml:"allow_public"`
	// Demo opts in to sandbox provisioning, where any key is accepted and
	// given an ephemeral tenant. It is off by default: a listener that hands
	// a tenant to any stranger is a deliberate choice, not an inherited one.
	Demo bool `yaml:"demo"`

	// TenantTTL is how long a sandbox survives without a visit. It slides from
	// the last connection, so a returning visitor keeps their board.
	TenantTTL    core.Duration `yaml:"tenant_ttl"`
	ReapInterval core.Duration `yaml:"reap_interval"`
	MaxTenants   int           `yaml:"max_tenants"`
	MaxTasks     int           `yaml:"max_tasks"`
	LeaseTTL     core.Duration `yaml:"lease_ttl"`

	RatePerHour int `yaml:"rate_per_hour"`
	RateBurst   int `yaml:"rate_burst"`

	// IdleTimeout closes a session nobody is typing at. It says nothing about
	// a session whose client has gone, which is what the keepalive is for.
	IdleTimeout core.Duration `yaml:"idle_timeout"`
	// KeepaliveInterval is the gap between liveness requests sent to a client.
	KeepaliveInterval core.Duration `yaml:"keepalive_interval"`
	// KeepaliveMaxMissed is how many of those may go unanswered before the
	// connection is dropped, releasing its session slot and its leases.
	KeepaliveMaxMissed int `yaml:"keepalive_max_missed"`

	// MaxSessionsPerKey caps how many sessions one key may hold at once, and
	// MaxSessions caps the listener as a whole. Each session is a program with
	// its own event subscription, so the rate limit on new connections is not
	// a limit on live ones.
	MaxSessionsPerKey int `yaml:"max_sessions_per_key"`
	MaxSessions       int `yaml:"max_sessions"`
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
	// AllowPrivateTargets permits a webhook endpoint to point at loopback, a
	// link-local address or a private range. Off by default, and deliberately
	// an operator setting rather than a tenant one: a tenant-supplied URL
	// reaching internal infrastructure is the classic request-forgery shape,
	// and the cloud metadata endpoint is a link-local address.
	AllowPrivateTargets bool `yaml:"allow_private_targets"`
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

// Log holds the logging settings: how much is emitted, how it is formatted,
// and where it lands.
//
// Output is the key that makes the rest matter. It is stderr by default, which
// is what a supervisor collects and what an operator sees, and the rotation
// settings then govern nothing. Naming a path instead turns this process into
// the thing that owns the file, so the file keys below are what stops an
// unattended `tix serve` filling the disk it runs on.
type Log struct {
	Level  string  `yaml:"level"`
	Format string  `yaml:"format"`
	Output string  `yaml:"output"`
	File   LogFile `yaml:"file"`
}

// LogFile bounds what a file log destination may leave on disk. It is read
// only when log.output names a path.
//
// The shipped values bound the worst case at MaxBackups+1 files of MaxSizeMB
// each, which is 800 MiB, and that number is the point of the defaults: it is
// large enough that nobody loses an incident to rotation and small enough that
// no reasonable disk is filled by a server nobody is watching.
type LogFile struct {
	// MaxSizeMB is the size one file may reach before it is archived.
	MaxSizeMB int `yaml:"max_size_mb"`
	// MaxAge is how long an archive is kept. Zero keeps them until MaxBackups
	// evicts them, which is what a deployment shipping logs elsewhere wants.
	MaxAge core.Duration `yaml:"max_age"`
	// MaxBackups is how many archives are kept. Zero keeps every archive that
	// is still inside MaxAge, and setting both to zero keeps everything, which
	// is a deliberate choice rather than the default.
	MaxBackups int `yaml:"max_backups"`
	// Compress gzips an archive once it is closed. It is off by default
	// because it spends CPU on the machine already busy writing the log, and
	// the disk bound is enforced by size and count either way.
	Compress bool `yaml:"compress"`
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
	DefaultDSN = "sqlite://~/.local/share/tix/tix.db"
	// DefaultDatabaseConnectTimeout is the wait a PostgreSQL target is given
	// to answer before it is called unreachable. It is what the engine used
	// before the wait was configurable.
	DefaultDatabaseConnectTimeout = "15s"
	DefaultListen                 = "127.0.0.1:8080"
	DefaultTenant                 = "default"
	// DefaultCookieSecurity follows the effective request scheme.
	DefaultCookieSecurity = CookieSecurityAuto
	DefaultAuthMode       = "token"
	DefaultHookMode       = "off"
	// DefaultWebhookDrainMode names the process that delivers queued webhooks.
	DefaultWebhookDrainMode = string(webhook.DefaultMode)
	DefaultLogLevel         = "info"
	// DefaultLogFormat is the handler a person reads at a terminal. Machines
	// that want to parse the stream ask for json.
	DefaultLogFormat = logging.FormatText
	// DefaultLogOutput keeps logs on stderr, where a supervisor already
	// collects them. A default that wrote files would create one on a machine
	// whose operator never asked for it.
	DefaultLogOutput = logging.DestStderr
	// DefaultLogFileMaxSizeMB keeps one file small enough to open in an editor
	// and to move off the box, while still holding hours of request logs.
	DefaultLogFileMaxSizeMB = 100
	// DefaultLogFileMaxAge is a week, the shortest window that still answers
	// "what happened over the weekend" on the Monday somebody asks.
	DefaultLogFileMaxAge = "168h"
	// DefaultLogFileMaxBackups is seven, so the age bound and the count bound
	// agree on the usual shape of roughly one rotation a day.
	DefaultLogFileMaxBackups = 7
	DefaultOutputFormat      = "table"
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

// Default settings of the SSH listener. They mirror internal/sshd, which holds
// the same defaults for a caller that assembles a listener directly; a test
// there keeps the two from drifting.
const (
	// DefaultSSHListen is loopback and port 2222. Port 22 is a deployment
	// concern, not something this process asks for the privilege to bind.
	DefaultSSHListen       = "127.0.0.1:2222"
	DefaultSSHTenantTTL    = "6h"
	DefaultSSHReapInterval = "10m"
	DefaultSSHMaxTenants   = 200
	DefaultSSHMaxTasks     = 200
	DefaultSSHLeaseTTL     = "2m"
	DefaultSSHRatePerHour  = 60
	DefaultSSHRateBurst    = 5
	DefaultSSHIdleTimeout  = "30m"
	// DefaultSSHKeepaliveInterval and DefaultSSHKeepaliveMaxMissed together
	// notice a vanished client in about two minutes, which is the length of a
	// seeded lease rather than the length of the idle timeout.
	DefaultSSHKeepaliveInterval  = "30s"
	DefaultSSHKeepaliveMaxMissed = 3
	DefaultSSHMaxSessionsPerKey  = 3
	DefaultSSHMaxSessions        = 100
)

// Cookie security settings, deciding whether a cookie is marked Secure.
const (
	// CookieSecurityAuto follows the effective scheme of each request.
	CookieSecurityAuto = "auto"
	// CookieSecurityAlways marks every cookie Secure.
	CookieSecurityAlways = "always"
	// CookieSecurityNever marks none, for a deliberately plaintext deployment.
	CookieSecurityNever = "never"
)

// CookieSecurities lists the settings server.cookie_security accepts.
var CookieSecurities = []string{CookieSecurityAuto, CookieSecurityAlways, CookieSecurityNever}

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
	// LogLevels and LogFormats mirror what internal/logging implements, so
	// configuration cannot accept a level or a handler that does not exist.
	LogLevels  = logging.Levels
	LogFormats = logging.Formats
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
		Database: Database{DSN: DefaultDSN, ConnectTimeout: mustDuration(DefaultDatabaseConnectTimeout)},
		Server:   Server{Listen: DefaultListen, CookieSecurity: DefaultCookieSecurity},
		Auth:     Auth{Mode: DefaultAuthMode},
		Hooks:    Hooks{Mode: DefaultHookMode},
		Webhooks: Webhooks{DrainMode: DefaultWebhookDrainMode},
		SSH: SSH{
			Listen:             DefaultSSHListen,
			TenantTTL:          mustDuration(DefaultSSHTenantTTL),
			ReapInterval:       mustDuration(DefaultSSHReapInterval),
			MaxTenants:         DefaultSSHMaxTenants,
			MaxTasks:           DefaultSSHMaxTasks,
			LeaseTTL:           mustDuration(DefaultSSHLeaseTTL),
			RatePerHour:        DefaultSSHRatePerHour,
			RateBurst:          DefaultSSHRateBurst,
			IdleTimeout:        mustDuration(DefaultSSHIdleTimeout),
			KeepaliveInterval:  mustDuration(DefaultSSHKeepaliveInterval),
			KeepaliveMaxMissed: DefaultSSHKeepaliveMaxMissed,
			MaxSessionsPerKey:  DefaultSSHMaxSessionsPerKey,
			MaxSessions:        DefaultSSHMaxSessions,
		},
		Discovery: Discovery{
			Enabled:   true,
			Filenames: append([]string(nil), DefaultDiscoveryFilenames...),
		},
		Retention: Retention{
			Audit:             mustDuration(DefaultRetentionAudit),
			Events:            mustDuration(DefaultRetentionEvent),
			WebhookDeliveries: mustDuration(DefaultRetentionDelivery),
		},
		Log: Log{
			Level:  DefaultLogLevel,
			Format: DefaultLogFormat,
			Output: DefaultLogOutput,
			File: LogFile{
				MaxSizeMB:  DefaultLogFileMaxSizeMB,
				MaxAge:     mustDuration(DefaultLogFileMaxAge),
				MaxBackups: DefaultLogFileMaxBackups,
			},
		},
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
