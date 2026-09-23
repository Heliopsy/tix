// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
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
	demo         bool
	tenantTTL    time.Duration
	reapInterval time.Duration
	maxTenants   int
	maxTasks     int
	leaseTTL     time.Duration
	ratePerHour  int
	rateBurst    int
	idleTimeout  time.Duration

	keepaliveInterval  time.Duration
	keepaliveMaxMissed int
	maxSessionsPerKey  int
	maxSessions        int
}

// fromConfig lays the resolved configuration under the flags: every setting is
// a key, and a flag is the layer above every other. A flag nobody gave carries
// its declared default, which would otherwise silently outrank a configured
// value, so only a flag the operator actually typed is read.
func (o sshOptions) fromConfig(cmd *cobra.Command, cfg config.SSH) sshOptions {
	f := cmd.Flags()
	paths := []struct {
		flag string
		into *string
		from string
	}{
		{"listen", &o.listen, cfg.Listen},
		{"host-key", &o.hostKey, cfg.HostKey},
	}
	for _, s := range paths {
		if !f.Changed(s.flag) {
			*s.into = s.from
		}
	}
	durations := []struct {
		flag string
		into *time.Duration
		from core.Duration
	}{
		{"tenant-ttl", &o.tenantTTL, cfg.TenantTTL},
		{"reap-interval", &o.reapInterval, cfg.ReapInterval},
		{"lease-ttl", &o.leaseTTL, cfg.LeaseTTL},
		{"idle-timeout", &o.idleTimeout, cfg.IdleTimeout},
		{"keepalive-interval", &o.keepaliveInterval, cfg.KeepaliveInterval},
	}
	for _, d := range durations {
		if !f.Changed(d.flag) {
			*d.into = time.Duration(d.from)
		}
	}
	counts := []struct {
		flag string
		into *int
		from int
	}{
		{"max-tenants", &o.maxTenants, cfg.MaxTenants},
		{"max-tasks", &o.maxTasks, cfg.MaxTasks},
		{"rate-per-hour", &o.ratePerHour, cfg.RatePerHour},
		{"rate-burst", &o.rateBurst, cfg.RateBurst},
		{"keepalive-max-missed", &o.keepaliveMaxMissed, cfg.KeepaliveMaxMissed},
		{"max-sessions-per-key", &o.maxSessionsPerKey, cfg.MaxSessionsPerKey},
		{"max-sessions", &o.maxSessions, cfg.MaxSessions},
	}
	for _, c := range counts {
		if !f.Changed(c.flag) {
			*c.into = c.from
		}
	}
	flags := []struct {
		flag string
		into *bool
		from bool
	}{
		{"allow-public", &o.allowPublic, cfg.AllowPublic},
		{"demo", &o.demo, cfg.Demo},
	}
	for _, b := range flags {
		if !f.Changed(b.flag) {
			*b.into = b.from
		}
	}
	return o
}

// newSSHCmd builds the command that serves the terminal interface over SSH.
func newSSHCmd(g *globals) *cobra.Command {
	var o sshOptions
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Serve the terminal interface over SSH",
		Long: "Serve the terminal interface over SSH to the keys you have enrolled.\n\n" +
			"A public key is the only credential: enrol one with `tix user key add` and its " +
			"holder reaches the board as that actor, with exactly the authority their " +
			"membership grants. A key nobody enrolled is refused, and so is one that has " +
			"been revoked.\n\n" +
			"The username selects the tenant. A key enrolled in one tenant needs none, so " +
			"`ssh -p 2222 " + sshd.NeutralUser + "@host` is enough; a key enrolled in several " +
			"is resolved by naming the tenant, as in `ssh -p 2222 acme@host`.\n\n" +
			"Revoking a key stops it authenticating at once. It does not cut sessions it " +
			"already holds: those are bounded by --idle-timeout. Restart the listener for a " +
			"hard cut.\n\n" +
			"--demo serves a different thing entirely: any key is accepted and given a seeded " +
			"ephemeral tenant of its own, reaped after --tenant-ttl without a visit. That " +
			"listener faces strangers, so it takes its own database and refuses the " +
			"zero-configuration store that holds somebody's real work.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid configuration, %d fatal error.",
				core.KindInvalid.ExitCode(), core.ExitError),
		Example: "  tix ssh\n" +
			"  tix ssh --db /var/lib/tix/tix.db --listen 0.0.0.0:2222 --allow-public\n" +
			"  tix ssh --demo --db /var/lib/tix/demo.db\n" +
			"  tix ssh --demo --db /var/lib/tix/demo.db --tenant-ttl 24h --max-tenants 500",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return runSSH(cmd, g, o) },
	}

	registerSSHFlags(cmd, &o)
	return cmd
}

// registerSSHFlags declares the ssh flags on cmd, binding them to o.
func registerSSHFlags(cmd *cobra.Command, o *sshOptions) {
	f := cmd.Flags()
	f.StringVar(&o.listen, "listen", sshd.DefaultAddr, "address to listen on")
	f.StringVar(&o.hostKey, "host-key", "",
		"path to the persisted host key, generated on first run (default: beside the database)")
	f.BoolVar(&o.allowPublic, "allow-public", false,
		"allow binding a non-loopback address, which puts the listener on the network")
	f.BoolVar(&o.demo, "demo", false,
		"accept any key and give it a seeded ephemeral tenant, instead of serving enrolled keys")
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
	f.DurationVar(&o.keepaliveInterval, "keepalive-interval", sshd.DefaultKeepaliveInterval,
		"how often a client is asked whether it is still there")
	f.IntVar(&o.keepaliveMaxMissed, "keepalive-max-missed", sshd.DefaultKeepaliveMaxMissed,
		"unanswered keepalives before the connection is dropped")
	f.IntVar(&o.maxSessionsPerKey, "max-sessions-per-key", sshd.DefaultMaxSessionsPerKey,
		"how many sessions one key may hold at once")
	f.IntVar(&o.maxSessions, "max-sessions", sshd.DefaultMaxSessions,
		"how many sessions the listener may hold at once")
}

// checkSSHTarget refuses a target this listener must not serve, without
// opening it.
func checkSSHTarget(resolved *config.Resolved, g *globals, demo bool) error {
	target, err := connect.Resolve(resolved, g.overrides())
	if err != nil {
		return err
	}
	if target.Mode != connect.ModeLocal {
		return core.Invalid("ssh needs a local database target, not %q", target.URL)
	}
	// A listener that provisions a tenant for any stranger must not share a
	// store with real work, and the target nobody named is the one holding it.
	// Serving enrolled keys is the opposite case: the real store is the point,
	// so that listener takes the configured target like any other command.
	if demo && target.Origin == connect.OriginDefault {
		return core.Invalid(
			"ssh --demo refuses the database tix keeps your own work in: " +
				"give the demo a database of its own with --db")
	}
	return nil
}

// runSSH opens the configured target and serves it until a signal arrives.
func runSSH(cmd *cobra.Command, g *globals, o sshOptions) error {
	resolved, err := g.resolve()
	if err != nil {
		return err
	}
	// The target is judged before it is opened. Opening creates the database
	// and migrates it, so refusing afterwards would still have brought into
	// existence the very file this guard protects: on a machine that had never
	// run tix, "ssh refuses the zero-configuration store" left that store
	// behind on disk.
	// The mode is read before the target is judged, because the guard below
	// applies to one mode and not the other.
	o = o.fromConfig(cmd, resolved.Config.SSH)
	if err := checkSSHTarget(resolved, g, o.demo); err != nil {
		return err
	}
	conn, _, err := g.dial(cmd)
	if err != nil {
		return err
	}
	hostKey, err := resolveHostKey(o.hostKey, conn.Info.Target, "--host-key")
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
		Demo:          o.demo,
		TenantTTL:     o.tenantTTL,
		ReapInterval:  o.reapInterval,
		MaxTenants:    o.maxTenants,
		MaxTasks:      o.maxTasks,
		LeaseTTL:      o.leaseTTL,
		RatePerHour:   o.ratePerHour,
		RateBurst:     o.rateBurst,
		IdleTimeout:   o.idleTimeout,

		KeepaliveInterval:  o.keepaliveInterval,
		KeepaliveMaxMissed: o.keepaliveMaxMissed,
		MaxSessionsPerKey:  o.maxSessionsPerKey,
		MaxSessions:        o.maxSessions,

		TimeStyle: g.timeStyle(),
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
// The flag's name differs by command, so callers pass their own: tix ssh
// spells it --host-key and tix serve --ssh-host-key, and naming the wrong one
// sends an operator looking for a flag that command does not have.
func resolveHostKey(flag string, target connect.Target, flagName string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if target.Engine != connect.EngineSQLite || target.Path == "" {
		return "", core.Invalid(
			"a host key path is required for this target; pass %s", flagName)
	}
	return filepath.Join(filepath.Dir(target.Path), hostKeyName), nil
}

func init() { builders = append(builders, newSSHCmd) }
