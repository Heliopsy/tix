// SPDX-License-Identifier: AGPL-3.0-or-later

// Package config resolves tix configuration from flags, environment
// variables, a .env file, configuration files and built-in defaults.
package config

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/output"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/lease"
	"github.com/heliopsy/tix/internal/logging"
	"github.com/heliopsy/tix/internal/webhook"
)

// Config is the fully resolved tix configuration.
type Config struct {
	// Target is auto or local. Local opts into the zero-configuration database,
	// which a first run otherwise refuses to create on its own.
	Target         string             `yaml:"target"`
	Tenant         string             `yaml:"tenant"`
	Project        string             `yaml:"project"`
	CurrentContext string             `yaml:"current_context"`
	Contexts       map[string]Context `yaml:"contexts,omitempty"`
	Database       Database           `yaml:"database"`
	Server         Server             `yaml:"server"`
	Auth           Auth               `yaml:"auth"`
	Hooks          Hooks              `yaml:"hooks"`
	Webhooks       Webhooks           `yaml:"webhooks"`
	Sync           Sync               `yaml:"sync"`
	SSH            SSH                `yaml:"ssh"`
	Discovery      Discovery          `yaml:"discovery"`
	Lease          Lease              `yaml:"lease"`
	Artifacts      Artifacts          `yaml:"artifacts"`
	Session        Session            `yaml:"session"`
	Retention      Retention          `yaml:"retention"`
	Log            Log                `yaml:"log"`
	Output         Output             `yaml:"output"`
	CLI            CLI                `yaml:"cli"`
	Web            Web                `yaml:"web"`
	TUI            TUI                `yaml:"tui"`
	// Themes are custom palettes, by name, that a tenant may then name.
	//
	// Deployment configuration rather than a table: a palette is something an
	// operator writes once for the whole install, and a per-tenant row would
	// need its own CRUD, authorization and audit trail to say the same thing.
	// A tenant stores only the name, so the palette lives in one place.
	Themes map[string]Theme `yaml:"themes,omitempty"`
}

// Theme is one custom palette. Every colour must be #rrggbb; they end up
// inside a stylesheet, so core validates them before anything renders. The
// dark pair is optional and falls back to the light one.
type Theme struct {
	Accent         string `yaml:"accent"`
	AccentSoft     string `yaml:"accent_soft"`
	AccentDark     string `yaml:"accent_dark,omitempty"`
	AccentSoftDark string `yaml:"accent_soft_dark,omitempty"`
}

// ThemeRegistry builds the resolver every surface reads a tenant's accent
// from, with the configured palettes merged over the built-in ones.
//
// It returns an error rather than dropping a bad palette silently: a colour
// that does not parse is a typo an operator wants told about at start-up, not
// a tenant that quietly renders in the default.
func (c *Config) ThemeRegistry() (*core.ThemeRegistry, error) {
	if c == nil || len(c.Themes) == 0 {
		return core.NewThemeRegistry(nil)
	}
	// Sorted, so a malformed colour in a map reports the same offending theme
	// on every run rather than whichever one iteration reached first.
	names := make([]string, 0, len(c.Themes))
	for name := range c.Themes {
		names = append(names, name)
	}
	sort.Strings(names)
	custom := make([]core.Theme, 0, len(names))
	for _, name := range names {
		t := c.Themes[name]
		custom = append(custom, core.Theme{
			Name: name, Accent: t.Accent, AccentSoft: t.AccentSoft,
			AccentDark: t.AccentDark, AccentSoftDark: t.AccentSoftDark,
		})
	}
	return core.NewThemeRegistry(custom)
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

	// PublicURL is where people reach the browser interface, an absolute
	// http or https address with at most a base path, when that is not the
	// address a request arrives at: a proxy's name for a server bound to a
	// private port. Task links are built under it. It is shown, never bound.
	PublicURL string `yaml:"public_url"`

	// TrustedProxies lists the reverse proxies, as IPs or CIDR blocks, whose
	// X-Forwarded-Proto and X-Forwarded-For are believed. Any client can send
	// those headers, so an empty list believes neither from anybody.
	TrustedProxies []string `yaml:"trusted_proxies,omitempty"`

	// CookieSecurity decides the Secure flag on the session and CSRF cookies.
	// The default derives it from the scheme the client used, which behind a
	// TLS-terminating proxy is the proxy's, not this process's.
	CookieSecurity string `yaml:"cookie_security"`

	// MaxImportBytes caps the body of a project task import, over the API and
	// in the browser. It is its own key because an import is a file and
	// --max-body-bytes, which every other request answers to, is sized for a
	// form.
	MaxImportBytes int64 `yaml:"max_import_bytes"`

	// EgressAllowPrivate lets tix serve reach loopback, link-local and private
	// addresses on a tenant's behalf: webhook and inbox channel deliveries and
	// external sync. Off by default, because a tenant-supplied url reaching
	// internal infrastructure is the request-forgery case.
	EgressAllowPrivate bool `yaml:"egress_allow_private"`
	// EgressAllow lists the addresses, CIDR ranges, host names and .suffixes
	// a tenant-supplied url may reach although they are internal.
	EgressAllow []string `yaml:"egress_allow,omitempty"`
}

// PublicBase is PublicURL without its trailing slash, the base a task's path
// is appended to, or "" when none is set.
func (s Server) PublicBase() string {
	return strings.TrimRight(strings.TrimSpace(s.PublicURL), "/")
}

// Egress is the egress policy the server keys describe. Validate refuses an
// entry it cannot read, so a resolved configuration always yields one.
func (s Server) Egress() (egress.Policy, error) {
	return egress.New(s.EgressAllowPrivate, s.EgressAllow)
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
	// PublicAddress is where people reach the listener, host or host:port,
	// when that is not where it binds: a container listening on 2222 that
	// the host publishes as 2020. It is shown, never bound.
	PublicAddress string `yaml:"public_address"`
	// TrustedProxies lists the L4 proxies, as IPs or CIDR blocks, whose PROXY
	// protocol header names the real client. Any peer can send such a header,
	// so an empty list, the default, reads none from anybody. It is its own
	// key: `server.trusted_proxies` covers the HTTP reverse proxy and neither
	// implies nor is consulted by this listener.
	TrustedProxies []string `yaml:"trusted_proxies,omitempty"`
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
	// TOTPKey seals every enrolled second factor at rest, 32 bytes base64
	// encoded. It lives here and never in the database, so reading the
	// database file does not yield working second factors.
	//
	// It is as load-bearing as the database: lose it and every enrolled
	// secret is undecryptable, and each user has to be cleared by an
	// administrator and enrol again.
	TOTPKey string `yaml:"totp_key" secret:"opaque"`
	// MaxConcurrentHashes caps how many argon2id derivations run at once.
	// 0 sizes it from the soft memory limit; see auth.ConcurrencyFor.
	MaxConcurrentHashes int `yaml:"max_concurrent_hashes"`
	// Argon2MemoryKiB, Argon2Time and Argon2Threads override the password
	// hash cost new hashes are minted at, 0 for the built-in default each.
	Argon2MemoryKiB int `yaml:"argon2_memory_kib"`
	Argon2Time      int `yaml:"argon2_time"`
	Argon2Threads   int `yaml:"argon2_threads"`
}

// PasswordParams is the argon2id policy these settings ask for: the built-in
// default with any configured field replacing its own.
func (a Auth) PasswordParams() auth.Params {
	p := auth.DefaultParams()
	if a.Argon2MemoryKiB > 0 && int64(a.Argon2MemoryKiB) <= math.MaxUint32 {
		p.Memory = uint32(a.Argon2MemoryKiB) // #nosec G115 -- bounded above
	}
	if a.Argon2Time > 0 && int64(a.Argon2Time) <= math.MaxUint32 {
		p.Time = uint32(a.Argon2Time) // #nosec G115 -- bounded above
	}
	if a.Argon2Threads > 0 && int64(a.Argon2Threads) <= math.MaxUint8 {
		p.Threads = uint8(a.Argon2Threads) // #nosec G115 -- bounded above
	}
	return p
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

// Sync holds what external sync may reach on the host that runs it. Both keys
// are operator settings: a tenant's sources never name a server path or
// inherit the server's TIX_SYNC_<SOURCE>_* environment on tix serve unless
// one of these says so.
type Sync struct {
	// FileRoot is the directory a tenant's sources may name files under, in a
	// subdirectory named for the tenant key. Unset, tix serve reads no file a
	// tenant names.
	FileRoot string `yaml:"file_root"`
	// EnvSources binds the TIX_SYNC_<SOURCE>_* environment to the sources
	// that may use it on tix serve, each as TENANT/SOURCE or TENANT/*.
	EnvSources []string `yaml:"env_sources"`
}

// Discovery holds the per-directory context discovery settings.
type Discovery struct {
	Enabled   bool     `yaml:"enabled"`
	Filenames []string `yaml:"filenames"`
}

// Lease bounds how long a claim on a task may hold, for this deployment.
//
// These are the floor a tenant that has chosen nothing falls through to, and
// the ceiling it cannot rise above. Both are zero by default, and zero means
// the built-in: 15 minutes and 24 hours. Setting them here is how an operator
// governs an installation without touching a tenant row, which is the only
// lever available when tenants are created by people other than the operator.
//
// The full order a lease lifetime is decided by, most specific first: the ttl
// on the claim, the task's lease, the project's lease_default, the project's
// workflow default_lease, the tenant's own setting, these, and finally the
// constants. Max is a clamp rather than an
// override at every level, so a tenant may tighten this ceiling and never
// raise it. See lease.Policy.Narrow.
type Lease struct {
	Default core.Duration `yaml:"default"`
	Max     core.Duration `yaml:"max"`
}

// Policy renders the configured bounds as a lease policy. Zero on either side
// leaves the shipped constant in place.
func (l Lease) Policy() lease.Policy {
	p := lease.DefaultPolicy()
	if l.Default > 0 {
		p.Default, p.DefaultFrom = l.Default.D(), core.LeaseFromDeployment
	}
	if l.Max > 0 {
		p.Max, p.MaxFrom = l.Max.D(), core.LeaseFromDeployment
	}
	return p
}

// Artifacts bounds the blobs a task artifact may carry, for this deployment.
//
// A tenant may tighten both and loosen neither. The blob travels inside a
// request, so the API's artifact route and the browser's upload take a body
// sized for MaxBytes rather than --max-body-bytes: see ArtifactBodyLimit.
type Artifacts struct {
	// MaxBytes is the largest blob accepted, 1 MiB by default.
	MaxBytes int64 `yaml:"max_bytes"`
	// AllowedTypes are the media types and patterns such as image/* a blob may
	// carry. Whatever is accepted, a blob is only ever served as a download.
	AllowedTypes []string `yaml:"allowed_types"`
}

// Session bounds how long a sign-in lasts, for this deployment. A tenant may
// tighten each and loosen none.
type Session struct {
	// TTL is how long a sign-in lasts from the moment it is made, and from
	// each use after, 12h by default.
	TTL core.Duration `yaml:"ttl"`
	// IdleTimeout ends a session nobody used for this long. Zero, the
	// default, turns it off.
	IdleTimeout core.Duration `yaml:"idle_timeout"`
	// RememberTTL is how long a "keep me signed in" sign-in lasts, 30d by
	// default. Zero stops offering it.
	RememberTTL core.Duration `yaml:"remember_ttl"`
	// MaxLifetime is how long after the sign-in use may keep a session
	// alive, 30d by default. Zero stops renewal: a session ends TTL after it
	// was made.
	MaxLifetime core.Duration `yaml:"max_lifetime"`
	// RememberMaxLifetime is MaxLifetime for a "keep me signed in" session,
	// 90d by default.
	RememberMaxLifetime core.Duration `yaml:"remember_max_lifetime"`
}

// Policy is the deployment's session settings before any tenant narrows them.
func (s Session) Policy() core.SessionPolicy {
	return core.SessionPolicy{TTL: s.TTL, IdleTimeout: s.IdleTimeout, RememberTTL: s.RememberTTL,
		MaxLifetime: s.MaxLifetime, RememberMaxLifetime: s.RememberMaxLifetime}
}

// ArtifactBodyLimit is the request body an artifact upload of at most maxBytes
// needs: the blob base64 encoded in JSON, plus room for the payload and the
// form around it. It never falls below the general body limit.
func ArtifactBodyLimit(maxBytes, general int64) int64 {
	n := (maxBytes+2)/3*4 + artifactBodyOverhead
	return max(n, general)
}

// artifactBodyOverhead is the share of an artifact upload that is not the
// blob: its name, payload, content type and the form or JSON around them.
const artifactBodyOverhead = 1 << 20

// ArtifactLimitRefusal is the answer to an artifact upload over its body
// limit, naming the key that raises it.
func ArtifactLimitRefusal(maxBytes int64) string {
	return fmt.Sprintf("an artifact file may be at most %s on this server; the server's operator can raise "+
		"artifacts.max_bytes (%s)", core.ByteSize(maxBytes), EnvName("artifacts.max_bytes"))
}

// Retention holds how long history is kept. Each class is governed
// independently, so changing one never disturbs another.
type Retention struct {
	Audit             core.Duration `yaml:"audit"`
	Events            core.Duration `yaml:"events"`
	WebhookDeliveries core.Duration `yaml:"webhook_deliveries"`
	// IdempotencyKeys is how long a kept Idempotency-Key answers repeats of
	// its request. It is not a tenant policy: a key is a client's retry
	// handle, not history anybody reads.
	IdempotencyKeys core.Duration `yaml:"idempotency_keys"`
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
// Motion decides whether the selected row pulses. It is an enumeration rather
// than a boolean so a value nobody chose reads as unset instead of as off, and
// it is validated here because its two values are spelled in this package.
//
// Keys is a map, which the key walker does not enumerate, so it is settable
// from a configuration file rather than from a flag or an environment
// variable. One override per variable is not a shape an env var carries well.
type TUI struct {
	Keymap string            `yaml:"keymap"`
	Motion string            `yaml:"motion"`
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

// CLI holds the settings of the command line's own listings. PageSize is how
// many rows a listing prints when --limit is not typed.
//
// It is per surface rather than one number for the whole install because the
// right row count is a property of the thing doing the showing: a terminal
// window, a browser window and a pipe do not hold the same number of lines.
type CLI struct {
	PageSize int `yaml:"page_size"`
	// TargetNotice prints the one line naming the local database a command
	// used when no target was configured.
	TargetNotice bool `yaml:"target_notice"`
	// RecordHost adds the host name and operating-system user to the access
	// channel of every change a command makes against a database directly.
	RecordHost bool `yaml:"record_host"`
}

// Web holds the settings of the browser interface. PageSize is how many rows
// one page of a listing carries when the request asks for no other number.
type Web struct {
	PageSize int `yaml:"page_size"`
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
	// DefaultRetentionIdempotency is how long a kept idempotency key answers.
	DefaultRetentionIdempotency = "24h"
	// DefaultPageSize is how many rows a listing shows when nothing says
	// otherwise. Fifty filled more than three screens of a browser window,
	// which is more rows than anybody reads before paging.
	DefaultPageSize = core.DefaultDisplayLimit
	// DefaultMaxImportBytes is how large a task import a server takes. It is
	// half the import's own 64 MiB, which is the most a file can usefully be.
	DefaultMaxImportBytes = 32 << 20
	// DefaultTUIKeymap names the keybinding scheme every install starts on.
	DefaultTUIKeymap = "default"
	// DefaultTUIMotion pulses the selected row. The animation is one frame
	// every fraction of a second and stops once a session goes idle, so the
	// shipped value costs an abandoned session nothing.
	DefaultTUIMotion = TUIMotionOn
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
	// TUIMotions lists the values tui.motion takes. The terminal interface
	// reads the same two strings, but it must not be imported from here, so
	// its own constants are held against these by a test.
	TUIMotions = []string{TUIMotionOn, TUIMotionOff}
)

// The values tui.motion takes.
const (
	TUIMotionOn  = "on"
	TUIMotionOff = "off"
)

// Values of the target key.
const (
	// TargetAuto uses whatever target is configured, and refuses to create
	// the zero-configuration database without being asked.
	TargetAuto = "auto"
	// TargetLocal opts into the zero-configuration database.
	TargetLocal = "local"
)

// Targets lists the accepted values of the target key.
var Targets = []string{TargetAuto, TargetLocal}

// Defaults returns the built-in configuration.
func Defaults() Config {
	return Config{
		Target:   TargetAuto,
		Tenant:   DefaultTenant,
		Database: Database{DSN: DefaultDSN, ConnectTimeout: mustDuration(DefaultDatabaseConnectTimeout)},
		Server:   Server{Listen: DefaultListen, CookieSecurity: DefaultCookieSecurity, MaxImportBytes: DefaultMaxImportBytes},
		Auth:     Auth{Mode: DefaultAuthMode},
		Artifacts: Artifacts{
			MaxBytes: core.DefaultArtifactMaxBytes, AllowedTypes: slices.Clone(core.DefaultArtifactTypes)},
		Session: Session{TTL: core.DefaultSessionTTL, RememberTTL: core.DefaultSessionRememberTTL,
			MaxLifetime: core.DefaultSessionMaxLifetime, RememberMaxLifetime: core.DefaultSessionRememberMaxLifetime},
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
			IdempotencyKeys:   mustDuration(DefaultRetentionIdempotency),
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
		CLI: CLI{PageSize: DefaultPageSize, TargetNotice: true},
		Web: Web{PageSize: DefaultPageSize},
		TUI: TUI{Keymap: DefaultTUIKeymap, Motion: DefaultTUIMotion},
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
