// SPDX-License-Identifier: AGPL-3.0-or-later

// Package serving builds the server tix serve runs over an open local
// connection, so the command and the embedding library assemble it in one
// place and cannot drift apart.
package serving

import (
	"log/slog"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/web"
)

// Defaults of the knobs tix serve exposes as flags.
const (
	DefaultSweepInterval = time.Minute
	DefaultPruneInterval = time.Hour
)

// Params are the choices a caller makes that the configuration does not hold:
// tix serve's flags, and the SSH listener it builds itself.
type Params struct {
	Listen          string
	CertFile        string
	KeyFile         string
	Insecure        bool
	MaxBodyBytes    int64
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
	SweepInterval   time.Duration
	PruneInterval   time.Duration
	DisableSweep    bool
	DisableDispatch bool
	DisablePrune    bool
	DisableRegistry bool
	Heartbeat       time.Duration

	// SSHListen is the address the SSH listener binds, shown on the settings
	// page. SSH is the listener itself; nil runs none.
	SSHListen string
	SSH       server.SSHListener

	Logger *slog.Logger
	Clock  clock.Clock
}

// Defaults returns the parameters tix serve runs with when no flag is given.
func Defaults() Params {
	return Params{
		Listen:        server.DefaultAddr,
		SweepInterval: DefaultSweepInterval,
		PruneInterval: DefaultPruneInterval,
		Heartbeat:     core.ServerHeartbeatInterval,
	}
}

// Build assembles the HTTP server, browser interface and background workers
// over a local connection, configured by cfg and p. The server is not bound.
func Build(conn *connect.Conn, cfg *config.Resolved, p Params) (*server.Server, error) {
	if conn == nil || conn.Store == nil {
		return nil, core.Invalid("serving needs a local database connection")
	}
	if p.Logger == nil {
		p.Logger = slog.Default()
	}
	if p.Clock == nil {
		p.Clock = clock.New()
	}
	// Validate rejects a malformed palette at load, so a registry built here
	// cannot fail on colour; an error would mean a themes block that Validate
	// somehow let through, and serving the built-ins is the safe answer.
	themes, err := cfg.Config.ThemeRegistry()
	if err != nil {
		return nil, err
	}
	egressPolicy, err := cfg.Config.Server.Egress()
	if err != nil {
		return nil, err
	}
	c := cfg.Config
	return server.Assemble(server.Options{
		Logger:  p.Logger,
		Service: conn.Service,
		WebHandler: web.Handler(conn.Service,
			web.WithSecureCookies(p.CertFile != ""),
			web.WithTimeStyle(TimeStyle(cfg)),
			web.WithTargetDescribe(conn.Info.Target.Describe()),
			web.WithThemes(themes),
			web.WithPageSize(c.Web.PageSize),
			web.WithLeasePolicy(c.Lease.Policy()),
			web.WithSSH(p.SSHListen, c.SSH.PublicAddress),
			web.WithImportLimit(c.Server.MaxImportBytes),
			web.WithArtifactLimit(c.Artifacts.MaxBytes)),
		Store:                      conn.Store,
		Clock:                      p.Clock,
		TenantID:                   conn.Info.TenantID,
		Addr:                       p.Listen,
		CertFile:                   p.CertFile,
		KeyFile:                    p.KeyFile,
		AllowInsecure:              p.Insecure,
		MaxBodyBytes:               p.MaxBodyBytes,
		MaxImportBytes:             c.Server.MaxImportBytes,
		ImportRoutes:               web.ImportRoutes(),
		MaxArtifactBytes:           c.Artifacts.MaxBytes,
		ArtifactRoutes:             web.ArtifactRoutes(),
		Sessions:                   c.Session.Policy(),
		RequestTimeout:             p.RequestTimeout,
		ShutdownTimeout:            p.ShutdownTimeout,
		TrustedProxies:             c.Server.TrustedProxies,
		CookieSecurity:             c.Server.CookieSecurity,
		PublicURL:                  c.Server.PublicURL,
		AllowPrivateWebhookTargets: c.Webhooks.AllowPrivateTargets,
		Egress:                     egressPolicy,
		SweepInterval:              p.SweepInterval,
		PruneInterval:              p.PruneInterval,
		DisableSweep:               p.DisableSweep,
		DisableDispatch:            p.DisableDispatch,
		DisablePrune:               p.DisablePrune,
		DisableRegistry:            p.DisableRegistry,
		HeartbeatInterval:          p.Heartbeat,
		SSH:                        p.SSH,
	})
}

// TimeStyle is the instant format and zone the configuration sets, or the
// zero style when it sets none or an unreadable one.
func TimeStyle(cfg *config.Resolved) output.TimeStyle {
	if cfg == nil {
		return output.TimeStyle{}
	}
	style, err := output.NewTimeStyle(cfg.Config.Output.TimeFormat, cfg.Config.Output.Timezone)
	if err != nil {
		return output.TimeStyle{}
	}
	return style
}
