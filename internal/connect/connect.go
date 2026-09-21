// Package connect resolves the single target every tix command talks to.
package connect

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"strings"

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
)

// Origin names the layer that decided the target.
type Origin string

// Target origins, in descending precedence.
const (
	OriginFlag    Origin = "flag"
	OriginContext Origin = "context"
	OriginConfig  Origin = "config"
	OriginDefault Origin = "default"
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
	case t.Mode == ModeRemote:
		endpoint = t.URL
	case t.Engine == EnginePostgres:
		endpoint = redactDSN(t.DSN)
	}
	return string(t.Mode) + " " + endpoint + " (from " + string(t.Origin) + ")"
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

	closers []func() error
}

// Context returns ctx carrying the connection's actor and the CLI source.
func (c *Conn) Context(ctx context.Context) context.Context {
	return core.WithSource(core.WithActor(ctx, c.Actor), core.SourceCLI)
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
	remote, err := client.New(target.URL, ov.Token)
	if err != nil {
		return nil, err
	}
	conn := &Conn{Service: remote, Info: Info{Target: target}, closers: []func() error{remote.Close}}
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
	clk := clock.New()
	allowNetworkFS := cfg.Config.Database.AllowNetworkFS || ov.AllowNetworkFS
	st, err := openStore(target, clk, allowNetworkFS)
	if err != nil {
		return nil, err
	}
	conn := &Conn{Store: st, Info: Info{Target: target}, closers: []func() error{st.Close}}

	if err := st.Migrate(ctx); err != nil {
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
		service.WithPrivateWebhookTargets(cfg.Config.Webhooks.AllowPrivateTargets),
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
	return conn, nil
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
func openStore(target Target, clk clock.Clock, allowNetworkFS bool) (store.Store, error) {
	if target.Engine == EnginePostgres {
		return postgres.Open(target.DSN, clk)
	}
	if dir := filepath.Dir(target.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, core.Internal("creating database directory %q", dir).Wrap(err)
		}
	}
	return sqlite.Open(target.Path, clk, sqlite.WithAllowNetworkFS(allowNetworkFS))
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
		out = t
		return err
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
