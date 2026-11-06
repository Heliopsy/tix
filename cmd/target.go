// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// connectHint is the command that points the command line at a server.
const connectHint = "tix ctx add NAME --server URL --token TOKEN --use"

// confirmTarget stops a command that would create the zero-configuration
// database nobody asked for. On a terminal it asks; anywhere else it refuses,
// because a script or an agent that meant a server would otherwise fill a
// database on this machine and find out much later.
func (g *globals) confirmTarget(cmd *cobra.Command, resolved *config.Resolved, target connect.Target) error {
	if !connect.NeedsOptIn(resolved, target) {
		return nil
	}
	if g.completing || !onATerminal(cmd) {
		return errNoTarget(target.Path)
	}
	errw := cmd.ErrOrStderr()
	_, _ = fmt.Fprintf(errw, "tix has no server or database configured.\n"+
		"  1) create a local database at %s, on this machine only\n"+
		"  2) connect to a tix server\n", target.Path)
	answer, err := g.ask(cmd, "choose 1 or 2: ")
	if err != nil {
		return errNoTarget(target.Path)
	}
	switch strings.ToLower(answer) {
	case "1", "l", "local":
		return nil
	case "2", "s", "server":
		return core.Precondition("nothing was written; connect with `%s`, "+
			"using the URL and a token from the server's /connect page, then run the command again", connectHint)
	default:
		return errNoTarget(target.Path)
	}
}

// errNoTarget refuses to create the zero-configuration database unasked.
func errNoTarget(path string) error {
	return core.Precondition("no server or database is configured, and tix will not create a local database unasked: "+
		"connect to a server with `%s`, or run `tix init` to keep tasks in %s on this machine (or pass --db PATH)",
		connectHint, path)
}

// noticeTarget names the local database a command used when nothing named a
// target. It is not a diagnostic --quiet hides: it is the one line saying the
// work went to this machine rather than to a server.
func (g *globals) noticeTarget(cmd *cobra.Command, resolved *config.Resolved, target connect.Target) {
	if g.completing || !target.Implicit() || !resolved.Config.CLI.TargetNotice {
		return
	}
	errw := cmd.ErrOrStderr()
	line := fmt.Sprintf("using local database %s (no server configured; run `tix ctx add` to use a server)", target.Path)
	_, _ = fmt.Fprintln(errw, output.NewPainter(g.colorMode(), errw).Muted(line))
}

// explainUnauthenticated says why a server refused a command that presented
// no token, which is otherwise a bare "no credential presented".
func explainUnauthenticated(err error, target connect.Target, token string) error {
	if target.Mode != connect.ModeRemote || strings.TrimSpace(token) != "" ||
		!core.IsKind(err, core.KindUnauthenticated) {
		return err
	}
	return core.Unauthenticated("%s needs an API token and none is configured: "+
		"add one with `tix ctx add NAME --server %s --token TOKEN --use`, or pass --token or TIX_TOKEN",
		target.URL, target.URL)
}

// identity is what tix whoami reports: where commands go and who they act as.
type identity struct {
	Target      connect.Mode `json:"target" yaml:"target"`
	Server      string       `json:"server,omitempty" yaml:"server,omitempty"`
	Database    string       `json:"database,omitempty" yaml:"database,omitempty"`
	Context     string       `json:"context,omitempty" yaml:"context,omitempty"`
	Origin      string       `json:"origin" yaml:"origin"`
	Tenant      string       `json:"tenant" yaml:"tenant"`
	TenantID    string       `json:"tenant_id" yaml:"tenant_id"`
	User        string       `json:"user" yaml:"user"`
	Name        string       `json:"name,omitempty" yaml:"name,omitempty"`
	Role        core.Role    `json:"role,omitempty" yaml:"role,omitempty"`
	Token       bool         `json:"token" yaml:"token"`
	TokenSource string       `json:"token_source,omitempty" yaml:"token_source,omitempty"`
}

// newWhoAmICmd builds the command that says where commands go and as whom.
func newWhoAmICmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the server or database commands use, and who they act as",
		Long: "Show where this command line is pointed: the server URL or the local database, the context " +
			"that chose it, the tenant, the user, and whether an API token is presented and where it came from.\n\n" +
			"Run it before any work you expect to land on a server. \"local\" means the tasks stay in a " +
			"database on this machine.\n\n" +
			"Exit codes: 5 no valid token for the server, 6 no target configured, 7 server unreachable.",
		Example: "  tix whoami\n  tix whoami -o json",
		GroupID: "setup",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			report := g.identity(ctx, conn)
			if g.formatName() != output.FormatTable {
				return g.render(cmd, report)
			}
			return writeIdentity(cmd.OutOrStdout(), report)
		},
	}
}

// identity assembles the report for an open connection.
func (g *globals) identity(ctx context.Context, conn *connect.Conn) identity {
	target := conn.Info.Target
	out := identity{
		Target: target.Mode, Context: target.Context, Origin: string(target.Origin),
		Tenant: target.Tenant, TenantID: conn.Info.TenantID,
	}
	switch {
	case target.Mode == connect.ModeRemote:
		out.Server = target.URL
	case target.Engine == connect.EnginePostgres:
		out.Database = config.RedactDSN(target.DSN)
	default:
		out.Database = target.Path
	}
	if conn.Actor != nil {
		out.User, out.Name, out.Role = conn.Actor.Handle, conn.Actor.DisplayName, conn.Actor.Role
		if out.TenantID == "" {
			out.TenantID = conn.Actor.TenantID
		}
	}
	if tenant, err := conn.Service.GetTenant(ctx, out.TenantID); err == nil && tenant != nil {
		out.Tenant = tenant.Key
	}
	if source := g.tokenSource(); source != "" {
		out.Token, out.TokenSource = true, source
	}
	return out
}

// tokenSource names where the presented token came from, or "" for none.
func (g *globals) tokenSource() string {
	if strings.TrimSpace(g.credential()) == "" {
		return ""
	}
	if strings.TrimSpace(g.token) != "" {
		return "--token"
	}
	if lookupEnv(g.environ, EnvToken) != "" {
		return EnvToken
	}
	resolved, err := g.resolve()
	if err != nil {
		return "configuration"
	}
	switch resolved.Source("server.token") {
	case config.LayerEnv:
		return config.EnvName("server.token")
	case config.LayerDotenv:
		return resolved.DotenvFile
	}
	if name := resolved.ContextName; name != "" && strings.TrimSpace(resolved.Config.Contexts[name].Token) != "" {
		return "context " + name
	}
	return "server.token in " + resolved.ConfigFile
}

// writeIdentity renders the report for a person.
func writeIdentity(w io.Writer, r identity) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	where := "server " + r.Server
	if r.Target == connect.ModeLocal {
		where = "local database " + r.Database
	}
	switch {
	case r.Context != "":
		where += " (context " + r.Context + ")"
	case r.Origin == string(connect.OriginDefault):
		where += " (no server configured; tix ctx add connects one)"
	default:
		where += " (from " + r.Origin + ")"
	}
	user := r.User
	if r.Name != "" && r.Name != r.User {
		user += " (" + r.Name + ")"
	}
	if r.Role != "" {
		user += ", " + string(r.Role)
	}
	_, err := fmt.Fprintf(tw, "target\t%s\ntenant\t%s\nuser\t%s\ntoken\t%s\n",
		where, dashIfEmpty(r.Tenant), dashIfEmpty(user), credentialLine(r))
	if err != nil {
		return err
	}
	return tw.Flush()
}

// credentialLine says whether a token is presented and where it came from.
// It names a source, never a value.
func credentialLine(r identity) string {
	switch {
	case r.Token:
		return "yes, from " + r.TokenSource
	case r.Target == connect.ModeLocal:
		return "none; this process opens the database directly"
	default:
		return "none"
	}
}

// initResult is what tix init reports.
type initResult struct {
	Database string `json:"database" yaml:"database"`
	Config   string `json:"config,omitempty" yaml:"config,omitempty"`
}

// newInitCmd builds the command that opts into a local database.
func newInitCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Keep tasks in a local database on this machine",
		Long: "Opt into the local database: write target: local to the configuration file and create the " +
			"database at the default path, or at --db when it is given.\n\n" +
			"A command run with nothing configured refuses to create that database by itself, so tasks meant " +
			"for a server never land on this machine by accident. To use a server instead, run " +
			"`tix ctx add NAME --server URL --token TOKEN --use`.\n\n" +
			"Exit codes: 2 a server is the configured target.",
		Example: "  tix init\n  tix init --db ~/work/tix.db",
		GroupID: "setup",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := g.resolve()
			if err != nil {
				return err
			}
			target, err := connect.Resolve(resolved, g.overrides())
			if err != nil {
				return err
			}
			if target.Mode == connect.ModeRemote {
				return usagef(cmd, "the configured target is the server %s; tix init sets up a local database", target.URL)
			}
			out := initResult{Database: target.Path}
			if target.Engine == connect.EnginePostgres {
				out.Database = config.RedactDSN(target.DSN)
			}
			if target.Implicit() {
				out.Config = g.configFilePath()
			}
			if dryRun {
				return g.render(cmd, newPlan("init", out.Database, map[string]any{"config": out.Config}))
			}
			if out.Config != "" && resolved.Config.Target != config.TargetLocal {
				if err := g.saveConfig(out.Config, func(cfg *config.Config) { cfg.Target = config.TargetLocal }); err != nil {
					return err
				}
				resolved.Config.Target = config.TargetLocal
			}
			if _, _, err := g.dial(cmd); err != nil {
				return err
			}
			return g.render(cmd, out)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

// saveConfig applies one change to the configuration file on disk.
func (g *globals) saveConfig(path string, apply func(*config.Config)) error {
	existing, cfg, err := readConfigFile(path)
	if err != nil {
		return err
	}
	before := *cfg
	apply(cfg)
	if err := config.SaveChanges(path, existing, &before, cfg); err != nil {
		return core.Internal("saving configuration").Wrap(err)
	}
	return nil
}

// describeTarget names the target commands would use, without opening it.
func (g *globals) describeTarget() string {
	resolved, err := g.resolve()
	if err != nil {
		return "an unreadable configuration"
	}
	target, err := connect.Resolve(resolved, g.overrides())
	if err != nil {
		return "no valid target"
	}
	if target.Mode == connect.ModeRemote {
		return "the server " + target.URL
	}
	if target.Engine == connect.EnginePostgres {
		return "the database " + config.RedactDSN(target.DSN)
	}
	return "the local database " + target.Path
}
