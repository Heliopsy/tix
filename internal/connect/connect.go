// SPDX-License-Identifier: AGPL-3.0-or-later

// Package connect resolves the single target every tix command talks to.
package connect

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/postgres"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/webhook"
)

// Mode names the transport a resolved target uses.
type Mode string

// Transport modes.
const (
	ModeLocal  Mode = "local"
	ModeRemote Mode = "remote"
	// ModeSSH is a command the SSH listener runs on a caller's behalf, against
	// the service the listener itself holds.
	ModeSSH Mode = "ssh"
)

// Origin names the layer that decided the target.
type Origin string

// Target origins, in descending precedence.
const (
	OriginFlag    Origin = "flag"
	OriginContext Origin = "context"
	OriginConfig  Origin = "config"
	OriginDefault Origin = "default"
	// OriginSSH is a target the SSH listener chose for a hosted command.
	OriginSSH Origin = "ssh"
)

// Engine names the database engine behind a local target.
type Engine string

// Database engines a local target can name.
const (
	EngineSQLite   Engine = "sqlite"
	EnginePostgres Engine = "postgres"
)

// LocalHandle is the handle given to the actor synthesized for local use.
const LocalHandle = "local"

// KeyDrainMode is the configuration key selecting the webhook drain mode.
const KeyDrainMode = "webhooks.drain_mode"

// DrainModeServer is the mode a process running the dispatcher itself prefers.
const DrainModeServer = string(webhook.ModeServer)

// Overrides are the raw target selectors a command may supply.
type Overrides struct {
	DB      string
	Server  string
	Token   string
	Home    string
	Environ []string

	// DrainMode is the webhook drain mode a process prefers when the operator
	// left the key at its default. A serving process sets it, because it runs
	// the dispatcher itself; an explicit setting still wins.
	DrainMode string

	// AllowNetworkFS opts into opening a database detected on a network
	// filesystem. It is combined with the configured database.allow_network_fs
	// key: either one being set is enough, since both express the same
	// operator decision through a different layer.
	AllowNetworkFS bool

	// JournalMode is the SQLite journal mode, wal or delete; empty means wal.
	// Only the embedded package sets it, for a database a sync tool copies.
	JournalMode string

	// Clock replaces the wall clock this connection runs on. It exists for
	// `tix demo seed`, which replays months of history through the ordinary
	// service calls rather than writing rows behind them: without a clock it
	// can move, every seeded task is created and completed in the same second
	// and the statistics screens it exists to populate have nothing to show.
	// Nil means the real clock, which is every other caller.
	Clock clock.Clock

	// ClientHint names this client to a remote server: cli, or mcp for
	// `tix mcp`. Empty means cli.
	ClientHint string

	// Serving marks a process that answers other people, tix serve. Its
	// service lets a tenant's sync sources reach only the files and
	// environment the operator's sync settings grant; any other process is
	// the operator's own and reaches what they can.
	Serving bool

	// Anonymous skips confirming the credential against a remote server.
	// tix login sets it: it exists to obtain a credential, so asking the
	// server who the caller is first could only fail.
	Anonymous bool
}

// Target is the resolved endpoint a command will talk to.
type Target struct {
	Mode    Mode   `json:"mode" yaml:"mode"`
	Engine  Engine `json:"engine,omitempty" yaml:"engine,omitempty"`
	Path    string `json:"path,omitempty" yaml:"path,omitempty"`
	DSN     string `json:"dsn,omitempty" yaml:"dsn,omitempty"`
	URL     string `json:"url,omitempty" yaml:"url,omitempty"`
	Tenant  string `json:"tenant" yaml:"tenant"`
	Context string `json:"context,omitempty" yaml:"context,omitempty"`
	Origin  Origin `json:"origin" yaml:"origin"`
}

// Describe renders the target for a diagnostic line.
func (t Target) Describe() string {
	endpoint := t.Path
	switch {
	case t.Mode == ModeSSH:
		return "ssh session as " + t.Tenant
	case t.Mode == ModeRemote:
		endpoint = t.URL
	case t.Engine == EnginePostgres:
		endpoint = redactDSN(t.DSN)
	}
	return string(t.Mode) + " " + endpoint + " (from " + string(t.Origin) + ")"
}

// Implicit reports whether the target is the zero-configuration database:
// local, and chosen only because nothing named another target.
func (t Target) Implicit() bool { return t.Mode == ModeLocal && t.Origin == OriginDefault }

// NeedsOptIn reports whether opening the target would create the
// zero-configuration database without anyone having asked for it. A database
// already at that path counts as the request, so an existing install keeps
// working, and so does the target key set to local.
func NeedsOptIn(cfg *config.Resolved, t Target) bool {
	if !t.Implicit() || cfg.Config.Target == config.TargetLocal {
		return false
	}
	_, err := os.Stat(t.Path)
	return errors.Is(err, fs.ErrNotExist)
}

// Info reports what a successful open found.
type Info struct {
	Target        Target `json:"target" yaml:"target"`
	SchemaVersion int    `json:"schema_version,omitempty" yaml:"schema_version,omitempty"`
	TenantID      string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
}

// Conn is an open service together with the identity and target behind it.
type Conn struct {
	Service core.Service
	Actor   *core.Actor
	Info    Info

	// Store is the engine opened for a local target, so a caller that needs the
	// store itself does not open the database a second time. It is nil for a
	// remote target.
	Store store.Store

	// direct is true for a local target opened with no token: this process
	// holds the database, and its calls stand as a global admin.
	direct bool
	// Via is the access channel every call through this connection records
	// when it reaches a local store.
	Via core.Via

	closers []func() error
}

// Hosted is the connection a command run over the SSH listener uses: the
// listener's own service, the session's actor and its ssh channel. It is never
// direct, so no call through it stands as the global admin a local store
// grants, and it holds nothing to close.
func Hosted(svc core.Service, actor *core.Actor, via core.Via, tenant string) *Conn {
	return &Conn{
		Service: svc,
		Actor:   actor,
		Via:     via,
		Info: Info{
			Target:   Target{Mode: ModeSSH, Tenant: tenant, Origin: OriginSSH},
			TenantID: actor.TenantID,
		},
	}
}

// Context returns ctx carrying the connection's actor, the CLI source and
// the access channel.
func (c *Conn) Context(ctx context.Context) context.Context {
	ctx = core.WithVia(core.WithSource(core.WithActor(ctx, c.Actor), core.SourceCLI), c.Via)
	if c.direct {
		ctx = core.WithDirectStore(ctx)
	}
	return ctx
}

// Close releases the service and everything opened beneath it.
func (c *Conn) Close() error {
	var first error
	for i := len(c.closers) - 1; i >= 0; i-- {
		if err := c.closers[i](); err != nil && first == nil {
			first = err
		}
	}
	c.closers = nil
	return first
}

// Open returns the service the resolved configuration and overrides select.
func Open(ctx context.Context, cfg *config.Resolved, ov Overrides) (core.Service, error) {
	conn, err := Dial(ctx, cfg, ov)
	if err != nil {
		return nil, err
	}
	return conn.Service, nil
}

// Dial resolves a target, opens it and returns the connection.
func Dial(ctx context.Context, cfg *config.Resolved, ov Overrides) (*Conn, error) {
	target, err := Resolve(cfg, ov)
	if err != nil {
		return nil, err
	}
	if target.Mode == ModeRemote {
		return dialRemote(ctx, target, ov)
	}
	return dialLocal(ctx, cfg, target, ov)
}

// Resolve selects the target without opening it.
func Resolve(cfg *config.Resolved, ov Overrides) (Target, error) {
	if strings.TrimSpace(ov.DB) != "" && strings.TrimSpace(ov.Server) != "" {
		return Target{}, core.Invalid("--db and --server are mutually exclusive; a target is either a database or a server")
	}
	target := Target{Tenant: cfg.Config.Tenant, Context: cfg.ContextName}

	switch {
	case strings.TrimSpace(ov.Server) != "":
		target.Mode, target.URL, target.Origin = ModeRemote, strings.TrimSpace(ov.Server), OriginFlag
		return target, nil
	case strings.TrimSpace(ov.DB) != "":
		if err := target.setLocal(ov.DB, ov.Home); err != nil {
			return Target{}, err
		}
		target.Origin = OriginFlag
		return target, nil
	}

	target.Origin = originOf(cfg)
	if url := strings.TrimSpace(cfg.Config.Server.URL); url != "" {
		target.Mode, target.URL = ModeRemote, url
		return target, nil
	}

	dsn := cfg.Config.Database.DSN
	if target.Origin == OriginDefault {
		dsn = defaultDSN(ov)
	}
	if err := target.setLocal(dsn, ov.Home); err != nil {
		return Target{}, err
	}
	return target, nil
}

// setLocal classifies dsn as the engine and endpoint it names.
func (t *Target) setLocal(dsn, home string) error {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return core.Invalid("no database is configured")
	}
	if scheme, rest, ok := strings.Cut(dsn, "://"); ok {
		switch scheme {
		case "postgres", "postgresql":
			t.Mode, t.Engine, t.DSN = ModeLocal, EnginePostgres, dsn
			return nil
		case "sqlite", "sqlite3", "file":
			dsn = rest
		default:
			return core.Invalid("database engine %q is not supported by this build", scheme)
		}
	} else if rest, ok := strings.CutPrefix(dsn, "file:"); ok {
		dsn = rest
	}
	if dsn == "" {
		return core.Invalid("database dsn names no file")
	}
	t.Mode, t.Engine, t.Path = ModeLocal, EngineSQLite, expandHome(dsn, home)
	return nil
}

// redactDSN hides the credentials of a connection string shown in a diagnostic.
//
// It defers to config.RedactDSN rather than keeping a second, weaker rule. The
// version that lived here handled only the userinfo form, so a DSN carrying its
// password as a query parameter, which both drivers accept, was printed intact
// wherever a target is described: the diagnostic command, and the settings page
// that now names the database a server is talking to.
func redactDSN(dsn string) string {
	return config.RedactDSN(dsn)
}

// originOf reports which layer decided the endpoint.
func originOf(cfg *config.Resolved) Origin {
	if cfg.ContextName != "" {
		return OriginContext
	}
	if cfg.Source("server.url") != config.LayerDefault && strings.TrimSpace(cfg.Config.Server.URL) != "" {
		return OriginConfig
	}
	if cfg.Source("database.dsn") != config.LayerDefault {
		return OriginConfig
	}
	return OriginDefault
}

// defaultDSN places the zero-configuration database under the XDG data home.
func defaultDSN(ov Overrides) string {
	if data := strings.TrimSpace(lookup(ov.Environ, "XDG_DATA_HOME")); data != "" {
		return filepath.Join(data, "tix", "tix.db")
	}
	home := ov.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return filepath.Join(home, ".local", "share", "tix", "tix.db")
}

// expandHome replaces a leading ~ with home.
func expandHome(path, home string) string {
	if home == "" || path == "" || path[0] != '~' {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// remote is the HTTP client, asserted here so the seam cannot drift.
var _ core.Service = (*client.Client)(nil)

// dialRemote opens the HTTP client and confirms the credential it carries.
func dialRemote(ctx context.Context, target Target, ov Overrides) (*Conn, error) {
	hint := ov.ClientHint
	if hint == "" {
		hint = core.ClientHintCLI
	}
	remote, err := client.New(target.URL, ov.Token, client.WithClientHint(hint))
	if err != nil {
		return nil, err
	}
	conn := &Conn{
		Service: remote, Info: Info{Target: target}, closers: []func() error{remote.Close},
		Via: core.Via{Kind: core.ViaAPI}.Refine(hint),
	}
	if ov.Anonymous {
		return conn, nil
	}
	actor, err := remote.WhoAmI(ctx)
	if err != nil {
		return nil, closeWith(conn, err)
	}
	conn.Actor = actor
	conn.Info.TenantID = actor.TenantID
	return conn, nil
}

// dialLocal opens the store the target names, migrates it, resolves the
// tenant and establishes the actor every subsequent call runs as.
func dialLocal(ctx context.Context, cfg *config.Resolved, target Target, ov Overrides) (*Conn, error) {
	mode, err := drainMode(cfg, ov)
	if err != nil {
		return nil, err
	}
	var clk clock.Clock = clock.New()
	if ov.Clock != nil {
		clk = ov.Clock
	}
	allowNetworkFS := cfg.Config.Database.AllowNetworkFS || ov.AllowNetworkFS
	st, err := openStore(target, clk, allowNetworkFS, ov.JournalMode, cfg.Config.Database.ConnectTimeout.D())
	if err != nil {
		return nil, err
	}
	conn := &Conn{Store: st, Info: Info{Target: target}, closers: []func() error{st.Close}}
	conn.Via = localVia(cfg.Config.CLI.RecordHost)
	ctx = core.WithVia(ctx, conn.Via)

	if err := st.Migrate(ctx); err != nil {
		return nil, closeWith(conn, err)
	}
	egressPolicy, err := cfg.Config.Server.Egress()
	if err != nil {
		return nil, closeWith(conn, err)
	}
	themes, err := cfg.Config.ThemeRegistry()
	if err != nil {
		return nil, closeWith(conn, err)
	}
	version, err := st.SchemaVersion(ctx)
	if err != nil {
		return nil, closeWith(conn, err)
	}
	conn.Info.SchemaVersion = version

	local := service.New(st,
		service.WithClock(clk),
		service.WithHooks(service.HookModeOf(mode)),
		service.WithRetentionDefaults(cfg.Config.Retention.Policy("")),
		service.WithLeaseDefaults(cfg.Config.Lease.Policy()),
		service.WithPrivateWebhookTargets(cfg.Config.Webhooks.AllowPrivateTargets),
		service.WithEgress(egressPolicy),
		service.WithPublicURL(cfg.Config.Server.PublicURL),
		service.WithTOTPKey(cfg.Config.Auth.TOTPKey),
		service.WithThemes(themes),
		service.WithSyncHost(service.SyncHost{
			Trusted: !ov.Serving, FileRoot: expandHome(cfg.Config.Sync.FileRoot, ov.Home),
			EnvSources: cfg.Config.Sync.EnvSources}),
		service.WithTimeDefaults(core.TimePrefs{
			TimeFormat: cfg.Config.Output.TimeFormat, Timezone: cfg.Config.Output.Timezone}),
	)
	tenant, err := resolveTenant(ctx, st, local, target.Tenant)
	if err != nil {
		return nil, closeWith(conn, err)
	}
	conn.Info.TenantID = tenant.ID
	conn.Service = local
	conn.closers = []func() error{local.Close}

	actor, err := localActor(ctx, st, clk, tenant.ID, ov.Token)
	if err != nil {
		return nil, closeWith(conn, err)
	}
	conn.Actor = actor
	conn.direct = strings.TrimSpace(ov.Token) == ""
	conn.Via.TokenID, conn.Via.TokenName = actor.TokenID, actor.TokenName
	return conn, nil
}

// localVia is the channel of a command run against a database directly. The
// host name and operating-system user are recorded only when asked for: they
// say which machine somebody worked from, which an operator may not want in a
// log every tenant member can read.
func localVia(recordHost bool) core.Via {
	v := core.Via{Kind: core.ViaCLILocal}
	if !recordHost {
		return v
	}
	if host, err := os.Hostname(); err == nil {
		v.Host = host
	}
	if u, err := user.Current(); err == nil {
		v.OSUser = u.Username
	}
	return v
}

// drainMode resolves who delivers queued webhooks. The process preference only
// applies while the key still sits on its default layer.
func drainMode(cfg *config.Resolved, ov Overrides) (webhook.Mode, error) {
	raw := cfg.Config.Webhooks.DrainMode
	if preferred := strings.TrimSpace(ov.DrainMode); preferred != "" &&
		cfg.Source(KeyDrainMode) == config.LayerDefault {
		raw = preferred
	}
	return webhook.ParseMode(raw)
}

// openStore opens the engine the target names. allowNetworkFS carries the
// operator's explicit opt-in past SQLite's refusal to open a database
// detected on a network filesystem; it does nothing for PostgreSQL.
// journalMode picks SQLite's journal mode. connectTimeout bounds PostgreSQL's
// startup reachability check, and does nothing for SQLite, which has no
// connection to wait for.
func openStore(target Target, clk clock.Clock, allowNetworkFS bool, journalMode string, connectTimeout time.Duration) (store.Store, error) {
	if target.Engine == EnginePostgres {
		return postgres.Open(target.DSN, clk, postgres.WithConnectTimeout(connectTimeout))
	}
	if dir := filepath.Dir(target.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, core.Internal("creating database directory %q", dir).Wrap(err)
		}
	}
	return sqlite.Open(target.Path, clk, sqlite.WithAllowNetworkFS(allowNetworkFS), sqlite.WithJournalMode(journalMode))
}

// resolveTenant returns the tenant the target selects. The default tenant is
// seeded on first use; any other key must already exist.
func resolveTenant(ctx context.Context, st store.Store, local *service.Local, key string) (*core.Tenant, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" || key == service.DefaultTenantKey {
		return local.EnsureDefaults(ctx)
	}
	var found *core.Tenant
	err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		t, err := u.GetTenantByKey(ctx, key)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		found = t
		return err
	})
	if err != nil {
		return nil, err
	}
	if found == nil || found.DeletedAt != nil {
		return nil, core.NotFound("tenant %q does not exist; create it with tix tenant create %s", key, key)
	}
	return found, nil
}

// closeWith releases a half-built connection and returns the original error.
func closeWith(c *Conn, err error) error {
	_ = c.Close()
	return err
}

// localActor verifies a presented token, or synthesizes the machine-local
// identity so a fresh install needs no credential at all.
func localActor(ctx context.Context, st store.Store, clk clock.Clock, tenantID, token string) (*core.Actor, error) {
	scope := core.TenantScope{TenantID: tenantID}
	if strings.TrimSpace(token) != "" {
		return auth.NewTokenVerifier(tokenLookup{st: st, scope: scope}, clk).Verify(ctx, token)
	}
	return ensureLocalActor(ctx, st, scope)
}

// ensureLocalActor returns the persisted local actor, creating it on first use.
func ensureLocalActor(ctx context.Context, st store.Store, scope core.TenantScope) (*core.Actor, error) {
	handle := LocalHandle
	var found *core.Actor
	err := st.View(ctx, scope, func(tx store.Tx) error {
		a, err := tx.GetActorByHandle(ctx, handle)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		found = a
		return err
	})
	if err != nil {
		return nil, err
	}
	if found == nil {
		created := &core.Actor{TenantID: tenantID(scope), Kind: core.ActorUser, Handle: handle, DisplayName: localName()}
		if err := st.Update(ctx, scope, func(tx store.Tx) error {
			return tx.CreateActor(ctx, created)
		}); err != nil {
			return nil, err
		}
		found = created
	}
	found.TenantID = tenantID(scope)
	found.Role = core.RoleAdmin
	found.Scopes = []core.Scope{core.ScopeAll}
	return found, nil
}

func tenantID(s core.TenantScope) string { return s.TenantID }

// localName describes the operating-system user behind the local actor.
func localName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return LocalHandle
}

// tokenLookup adapts the store to the token verifier.
type tokenLookup struct {
	st    store.Store
	scope core.TenantScope
}

// TokenByHash returns the stored token record for a hash.
func (l tokenLookup) TokenByHash(ctx context.Context, hash string) (*core.APIToken, error) {
	var out *core.APIToken
	err := l.st.View(ctx, l.scope, func(tx store.Tx) error {
		t, err := tx.GetTokenByHash(ctx, hash)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		withdrawn, err := service.StandingWithdrawn(ctx, tx, t.ActorID)
		if err != nil || withdrawn {
			return err
		}
		out = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ConfigWritePath returns where a configuration file should be written.
func ConfigWritePath(environ []string, home string) string {
	if xdg := strings.TrimSpace(lookup(environ, "XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(expandHome(xdg, home), config.RelativeConfigPath)
	}
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return filepath.Join(home, ".config", config.RelativeConfigPath)
}

// lookup returns the value of name in an environ slice.
func lookup(environ []string, name string) string {
	if environ == nil {
		return os.Getenv(name)
	}
	prefix := name + "="
	for i := len(environ) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(environ[i], prefix); ok {
			return value
		}
	}
	return ""
}

// InTenant returns a connection to the same local store that acts as the local
// actor of the tenant key names, created there on first use, as a connection
// dialled with that tenant would. It shares the service and store and holds
// nothing to close.
func (c *Conn) InTenant(ctx context.Context, key string) (*Conn, error) {
	local, ok := c.Service.(*service.Local)
	if !ok || c.Store == nil {
		return nil, core.Invalid("only a local database connection can act in another tenant")
	}
	tenant, err := resolveTenant(core.WithVia(ctx, c.Via), c.Store, local, key)
	if err != nil {
		return nil, err
	}
	actor, err := ensureLocalActor(ctx, c.Store, core.TenantScope{TenantID: tenant.ID})
	if err != nil {
		return nil, err
	}
	out := &Conn{Service: c.Service, Store: c.Store, Actor: actor, Info: c.Info, direct: c.direct, Via: c.Via}
	out.Info.TenantID = tenant.ID
	out.Info.Target.Tenant = tenant.Key
	return out, nil
}
