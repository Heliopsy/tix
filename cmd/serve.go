// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"log/slog"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/sshd"
	"github.com/heliopsy/tix/internal/web"
	"github.com/spf13/cobra"
)

// serveOptions are the flags that only the serve command takes.
type serveOptions struct {
	listen          string
	certFile        string
	keyFile         string
	insecure        bool
	maxBody         int64
	requestTimeout  time.Duration
	shutdownTimeout time.Duration
	sweepInterval   time.Duration
	pruneInterval   time.Duration
	disableSweep    bool
	disableDispatch bool
	disablePrune    bool

	sshListen      string
	sshHostKey     string
	sshAllowPublic bool
	sshIdleTimeout time.Duration
}

// newServeCmd builds the command that serves the HTTP API.
func newServeCmd(g *globals) *cobra.Command {
	var o serveOptions
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the tix HTTP API",
		Long: "Serve the tix HTTP API, the browser interface and, with --ssh-listen, the " +
			"terminal interface over SSH.\n\n" +
			"One process, one database and one shutdown: the SSH listener runs beside the " +
			"lease sweeper, webhook dispatcher and retention pruner, and a change made over " +
			"either surface is immediately visible to the other.\n\n" +
			"--ssh-listen is empty by default, and nothing else turns the listener on. It " +
			"serves the keys enrolled with `tix user key add`; the sandbox mode `tix ssh " +
			"--demo` offers is not available here, because this process serves real work.",
		Example: "  tix serve --db /var/lib/tix/tix.db\n" +
			"  tix serve --db /var/lib/tix/tix.db --ssh-listen 127.0.0.1:2222",
		GroupID: "admin",
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return runServe(cmd, g, o) },
	}

	f := cmd.Flags()
	f.StringVar(&o.listen, "listen", server.DefaultAddr, "address to listen on")
	f.StringVar(&o.certFile, "tls-cert", "", "path to the tls certificate")
	f.StringVar(&o.keyFile, "tls-key", "", "path to the tls key")
	f.BoolVar(&o.insecure, "insecure-no-tls", false,
		"allow binding a non-loopback address without tls")
	f.Int64Var(&o.maxBody, "max-body-bytes", 0, "maximum request body size")
	f.DurationVar(&o.requestTimeout, "request-timeout", 0, "per-request timeout")
	f.DurationVar(&o.shutdownTimeout, "shutdown-timeout", 0,
		"how long shutdown waits for in-flight requests")
	f.DurationVar(&o.sweepInterval, "sweep-interval", time.Minute,
		"how often expired leases are swept")
	f.DurationVar(&o.pruneInterval, "prune-interval", time.Hour,
		"how often retention pruning runs")
	f.BoolVar(&o.disableSweep, "no-lease-sweeper", false, "disable the lease sweeper")
	f.BoolVar(&o.disableDispatch, "no-webhook-dispatcher", false, "disable the webhook dispatcher")
	f.BoolVar(&o.disablePrune, "no-retention-pruner", false, "disable the retention pruner")
	f.StringVar(&o.sshListen, "ssh-listen", "",
		"address to serve the terminal interface over ssh on (default: no ssh listener)")
	f.StringVar(&o.sshHostKey, "ssh-host-key", "",
		"path to the persisted ssh host key, generated on first run (default: beside the database)")
	f.BoolVar(&o.sshAllowPublic, "ssh-allow-public", false,
		"allow binding a non-loopback ssh address, which puts the listener on the network")
	f.DurationVar(&o.sshIdleTimeout, "ssh-idle-timeout", sshd.DefaultIdleTimeout,
		"how long an ssh session may sit idle before it is closed, which bounds a revoked key")
	return cmd
}

// runServe opens the configured target and serves it until a signal arrives.
func runServe(cmd *cobra.Command, g *globals, o serveOptions) error {
	if !o.disableDispatch {
		g.drainMode = connect.DrainModeServer
	}
	conn, _, err := g.dial(cmd)
	if err != nil {
		return err
	}
	if conn.Info.Target.Mode != connect.ModeLocal {
		return core.Invalid("serve needs a local database target, not %q", conn.Info.Target.URL)
	}
	resolved, err := g.resolve()
	if err != nil {
		return err
	}
	log, err := g.logger(cmd)
	if err != nil {
		return err
	}
	sshListener, err := buildServeSSH(conn, g, o, log)
	if err != nil {
		return err
	}

	// Validate rejects a malformed palette at load, so a registry built here
	// cannot fail on colour; an error would mean a themes block that Validate
	// somehow let through, and serving the built-ins is the safe answer.
	themes, err := resolved.Config.ThemeRegistry()
	if err != nil {
		return err
	}

	srv, err := server.Assemble(server.Options{
		Logger:                     log,
		Service:                    conn.Service,
		WebHandler:                 web.Handler(conn.Service, web.WithSecureCookies(o.certFile != ""), web.WithTimeStyle(g.timeStyle()), web.WithTargetDescribe(conn.Info.Target.Describe()), web.WithThemes(themes)),
		Store:                      conn.Store,
		Clock:                      clock.New(),
		TenantID:                   conn.Info.TenantID,
		Addr:                       o.listen,
		CertFile:                   o.certFile,
		KeyFile:                    o.keyFile,
		AllowInsecure:              o.insecure,
		MaxBodyBytes:               o.maxBody,
		RequestTimeout:             o.requestTimeout,
		ShutdownTimeout:            o.shutdownTimeout,
		TrustedProxies:             resolved.Config.Server.TrustedProxies,
		CookieSecurity:             resolved.Config.Server.CookieSecurity,
		AllowPrivateWebhookTargets: resolved.Config.Webhooks.AllowPrivateTargets,
		SweepInterval:              o.sweepInterval,
		PruneInterval:              o.pruneInterval,
		DisableSweep:               o.disableSweep,
		DisableDispatch:            o.disableDispatch,
		DisablePrune:               o.disablePrune,
		SSH:                        sshListener,
	})
	if err != nil {
		return err
	}
	if err := srv.Listen(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "tix listening on %s\n", srv.Addr())
	if addr := srv.SSHAddr(); addr != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "tix ssh listening on %s\n", addr)
	}
	return srv.Serve(ctx)
}

// buildServeSSH constructs the SSH listener the serve command runs beside the
// HTTP server, or nothing at all when no address was given.
//
// The nil is returned as an untyped one on purpose: a typed nil in the
// interface would be a listener as far as the server is concerned, and it
// would bind a port.
func buildServeSSH(conn *connect.Conn, g *globals, o serveOptions, log *slog.Logger) (server.SSHListener, error) {
	if strings.TrimSpace(o.sshListen) == "" {
		return nil, nil
	}
	hostKey, err := resolveHostKey(o.sshHostKey, conn.Info.Target, "--ssh-host-key")
	if err != nil {
		return nil, err
	}
	listener, err := sshd.New(sshd.Options{
		Service:       conn.Service,
		Store:         conn.Store,
		Clock:         clock.New(),
		Addr:          o.sshListen,
		HostKeyPath:   hostKey,
		AllowInsecure: o.sshAllowPublic,
		IdleTimeout:   o.sshIdleTimeout,
		TimeStyle:     g.timeStyle(),
		Logger:        log,
	})
	if err != nil {
		return nil, err
	}
	return listener, nil
}

func init() { builders = append(builders, newServeCmd) }
