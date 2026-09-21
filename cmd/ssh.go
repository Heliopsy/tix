package cmd

import (
	"fmt"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sshd"
	"github.com/spf13/cobra"
)

// hostKeyName is the file a generated host key is written to, beside the
// database it serves, so the listener's identity and its data move together.
const hostKeyName = "ssh_host_ed25519_key"

// sshOptions are the flags that only the ssh command takes.
type sshOptions struct {
	listen       string
	hostKey      string
	allowPublic  bool
	tenantTTL    time.Duration
	reapInterval time.Duration
	maxTenants   int
	maxTasks     int
	leaseTTL     time.Duration
	ratePerHour  int
	rateBurst    int
	idleTimeout  time.Duration
}

// newSSHCmd builds the command that serves the terminal interface over SSH.
func newSSHCmd(g *globals) *cobra.Command {
	var o sshOptions
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Serve the terminal interface over SSH",
		Long: "Serve the terminal interface over SSH, giving every connecting key its own " +
			"ephemeral tenant.\n\n" +
			"Any public key is accepted: the fingerprint is the identity, so there is no signup " +
			"and no password. A key connecting for the first time gets a seeded sandbox of its " +
			"own; the same key connecting again gets that sandbox back. A sandbox nobody has " +
			"visited for the time to live is deleted with everything in it.\n\n" +
			"This listener faces strangers, so it takes its own database: pass --db or --server " +
			"explicitly. It refuses to run against the zero-configuration store, which holds " +
			"somebody's real work.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid configuration, %d fatal error.",
				core.KindInvalid.ExitCode(), core.ExitError),
		Example: "  tix ssh --db /var/lib/tix/demo.db\n" +
			"  tix ssh --db /var/lib/tix/demo.db --listen 0.0.0.0:2222 --allow-public\n" +
			"  tix ssh --db /var/lib/tix/demo.db --tenant-ttl 24h --max-tenants 500",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return runSSH(cmd, g, o) },
	}

	f := cmd.Flags()
	f.StringVar(&o.listen, "listen", sshd.DefaultAddr, "address to listen on")
	f.StringVar(&o.hostKey, "host-key", "",
		"path to the persisted host key, generated on first run (default: beside the database)")
	f.BoolVar(&o.allowPublic, "allow-public", false,
		"allow binding a non-loopback address, which puts the demo on the network")
	f.DurationVar(&o.tenantTTL, "tenant-ttl", sshd.DefaultTenantTTL,
		"how long a sandbox survives without a visit")
	f.DurationVar(&o.reapInterval, "reap-interval", sshd.DefaultReapInterval,
		"how often expired sandboxes are deleted")
	f.IntVar(&o.maxTenants, "max-tenants", sshd.DefaultMaxTenants,
		"how many sandboxes may be live before a new key is refused")
	f.IntVar(&o.maxTasks, "max-tasks", sshd.DefaultMaxTasks, "how many tasks one sandbox may hold")
	f.DurationVar(&o.leaseTTL, "lease-ttl", sshd.DefaultLeaseTTL,
		"how long a claim holds in a seeded sandbox")
	f.IntVar(&o.ratePerHour, "rate-per-hour", sshd.DefaultRatePerHour,
		"connections allowed per hour from one source address")
	f.IntVar(&o.rateBurst, "rate-burst", sshd.DefaultRateBurst,
		"connections one source address may make back to back")
	f.DurationVar(&o.idleTimeout, "idle-timeout", sshd.DefaultIdleTimeout,
		"how long a session may sit idle before it is closed")
	return cmd
}

// runSSH opens the configured target and serves it until a signal arrives.
func runSSH(cmd *cobra.Command, g *globals, o sshOptions) error {
	conn, _, err := g.dial(cmd)
	if err != nil {
		return err
	}
	if conn.Info.Target.Mode != connect.ModeLocal {
		return core.Invalid("ssh needs a local database target, not %q", conn.Info.Target.URL)
	}
	// A listener anybody can reach must not share a process with real tenants,
	// and the target nobody named is the one holding somebody's real work.
	if conn.Info.Target.Origin == connect.OriginDefault {
		return core.Invalid(
			"ssh refuses the zero-configuration store, which is somebody's real work: " +
				"name a database of its own with --db")
	}
	hostKey, err := resolveHostKey(o.hostKey, conn.Info.Target)
	if err != nil {
		return err
	}

	srv, err := sshd.New(sshd.Options{
		Service:       conn.Service,
		Store:         conn.Store,
		Clock:         clock.New(),
		Addr:          o.listen,
		HostKeyPath:   hostKey,
		AllowInsecure: o.allowPublic,
		TenantTTL:     o.tenantTTL,
		ReapInterval:  o.reapInterval,
		MaxTenants:    o.maxTenants,
		MaxTasks:      o.maxTasks,
		LeaseTTL:      o.leaseTTL,
		RatePerHour:   o.ratePerHour,
		RateBurst:     o.rateBurst,
		IdleTimeout:   o.idleTimeout,
		TimeStyle:     g.timeStyle(),
	})
	if err != nil {
		return err
	}
	if err := srv.Listen(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "tix ssh listening on %s\n", srv.Addr())
	return srv.Serve(ctx)
}

// resolveHostKey places a generated host key beside the database it serves, so
// restarting the listener does not hand every returning visitor a changed host
// key warning.
func resolveHostKey(flag string, target connect.Target) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if target.Engine != connect.EngineSQLite || target.Path == "" {
		return "", core.Invalid(
			"a host key path is required for this target; pass --host-key")
	}
	return filepath.Join(filepath.Dir(target.Path), hostKeyName), nil
}

func init() { builders = append(builders, newSSHCmd) }
