package cmd

import (
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
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
}

// newServeCmd builds the command that serves the HTTP API.
func newServeCmd(g *globals) *cobra.Command {
	var o serveOptions
	cmd := &cobra.Command{
		Use:     "serve",
		Short:   "Serve the tix HTTP API",
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

	srv, err := server.Assemble(server.Options{
		Service:         conn.Service,
		WebHandler:      web.Handler(conn.Service, web.WithSecureCookies(o.certFile != ""), web.WithTimeStyle(g.timeStyle()), web.WithTargetDescribe(conn.Info.Target.Describe())),
		Store:           conn.Store,
		Clock:           clock.New(),
		TenantID:        conn.Info.TenantID,
		Addr:            o.listen,
		CertFile:        o.certFile,
		KeyFile:         o.keyFile,
		AllowInsecure:   o.insecure,
		MaxBodyBytes:    o.maxBody,
		RequestTimeout:  o.requestTimeout,
		ShutdownTimeout: o.shutdownTimeout,
		SweepInterval:   o.sweepInterval,
		PruneInterval:   o.pruneInterval,
		DisableSweep:    o.disableSweep,
		DisableDispatch: o.disableDispatch,
		DisablePrune:    o.disablePrune,
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
	return srv.Serve(ctx)
}

func init() { builders = append(builders, newServeCmd) }
