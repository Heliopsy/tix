// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/tui"
	"github.com/heliopsy/tix/internal/version"
	"github.com/spf13/cobra"
)

// newTUICmd builds the command that runs the terminal interface.
// schemeNames lists the keybinding schemes the terminal interface ships.
func schemeNames() []string {
	out := make([]string, 0, len(tui.Schemes()))
	for _, s := range tui.Schemes() {
		out = append(out, string(s))
	}
	return out
}

func newTUICmd(g *globals) *cobra.Command {
	var project, filter, scheme string
	var overrides map[string]string
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Browse and work on tasks in a terminal interface",
		Long: "Open the terminal interface against the configured target.\n\n" +
			"T switches the session to another tenant, by key: an actor belongs to one tenant and cannot " +
			"list another, so there is no list to pick from and the key is checked by opening it. " +
			"The switch lasts for the session; tix tenant use writes it down.\n" +
			"v opens the activity tail and / filters it with the expression tix audit ls --filter takes.\n\n" +
			"Exit codes: 1 fatal error, 130 interrupted.",
		Example: "  tix tui\n  tix tui -p infra --filter \"status:todo is:unclaimed\"",
		GroupID: "work",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fallback, err := g.projectFallback(cmd)
			if err != nil {
				return err
			}
			if fallback != "" {
				project = fallback
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			// The flag wins, the configured scheme is the fallback, so a
			// choice made in the settings view survives a restart without
			// taking the flag away from a one-off run.
			resolved, err := g.resolve()
			if err != nil {
				return err
			}
			if scheme == "" {
				scheme = resolved.Config.TUI.Keymap
			}
			overrides = mergeOverrides(resolved.Config.TUI.Keys, overrides)
			// Refuse an unknown scheme before opening a terminal, so a typo in
			// tui.keymap reports the typo rather than a TTY error.
			if _, err := tui.ParseScheme(scheme); err != nil {
				return err
			}
			// The tenant's accent, resolved the same way the browser resolves
			// it, so one tenant is one colour on both surfaces. A tenant that
			// cannot be read (no membership, a server that refuses) leaves the
			// zero theme, which is the built-in accent: a board that will not
			// open over a branding lookup would be a poor trade.
			brand := core.Theme{}
			if reg, err := resolved.Config.ThemeRegistry(); err == nil {
				if tenant, err := conn.Service.GetTenant(ctx, resolved.Config.Tenant); err == nil {
					brand = reg.Resolve(tenant)
				}
			}
			code := tui.Run(tui.Options{
				Brand:     brand,
				TimeStyle: g.timeStyle(),
				Prefs: tui.Preferences{
					Keymap:     scheme,
					TimeFormat: resolved.Config.Output.TimeFormat,
					Timezone:   resolved.Config.Output.Timezone,
					Color:      resolved.Config.Output.Color,
				},
				Sources:   preferenceSources(resolved),
				SavePrefs: g.savePreferences(),
				Session:   g.sessionInfo(resolved),
				Tenant:    resolved.Config.Tenant,
				Dial:      g.tenantDialer(cmd),
				Service:   conn.Service,
				Actor:     conn.Actor,
				Access:    capability.TUIAccess(conn.Actor),
				Context:   ctx,
				Project:   project,
				Filter:    filter,
				Scheme:    scheme,
				Overrides: overrides,
				In:        cmd.InOrStdin(),
				Out:       cmd.OutOrStdout(),
				Err:       cmd.ErrOrStderr(),
				Environ:   g.environ,
			})
			if code == core.ExitOK {
				return nil
			}
			return &exitError{code: code}
		},
	}
	f := cmd.Flags()
	f.StringVarP(&project, "project", "p", "", "project key to open the board for")
	f.StringVar(&filter, "filter", "", "filter expression to start with")
	f.StringVar(&scheme, "keys", "", "keybinding scheme: "+strings.Join(schemeNames(), "|"))
	f.StringToStringVar(&overrides, "key", nil, "rebind one action, such as --key New=o")
	_ = cmd.RegisterFlagCompletionFunc("keys", fixedCompletion(schemeNames()))
	return cmd
}

func init() { builders = append(builders, newTUICmd) }

// tenantDialer opens a connection pinned to another tenant, which is what the
// terminal interface's tenant view needs and cannot build for itself: which
// database or server a key resolves against is configuration, and reading
// configuration is this layer's job. It is the same second connection
// `tix tenant use` opens to check a key, for the same reason: an actor
// belongs to one tenant and cannot list another, so the target has to answer
// for itself.
func (g *globals) tenantDialer(cmd *cobra.Command) tui.TenantDialer {
	return func(_ context.Context, key string) (tui.TenantConn, error) {
		target := &globals{
			configPath: g.configPath, contextName: g.contextName,
			db: g.db, server: g.server, tenant: key, token: g.token,
			noDiscovery: g.noDiscovery, allowNetworkFS: g.allowNetworkFS,
			environ: g.environ, dir: g.dir,
		}
		conn, ctx, err := target.dial(cmd)
		if err != nil {
			return tui.TenantConn{}, err
		}
		return tui.TenantConn{Service: conn.Service, Context: ctx, Close: conn.Close}, nil
	}
}

// mergeOverrides layers per-invocation rebindings over the configured ones.
// A flag naming an action the configuration also rebinds wins for that action
// alone, rather than discarding the rest of the configured set.
func mergeOverrides(configured, flags map[string]string) map[string]string {
	if len(configured) == 0 {
		return flags
	}
	out := make(map[string]string, len(configured)+len(flags))
	for action, key := range configured {
		out[action] = key
	}
	for action, key := range flags {
		out[action] = key
	}
	return out
}

// preferenceSources names the configuration layer each display preference
// arrived from, so the settings screen can warn that writing one down will not
// change what this run is using.
func preferenceSources(resolved *config.Resolved) tui.Preferences {
	return tui.Preferences{
		Keymap:     string(resolved.Source(tui.KeyTUIKeymap)),
		TimeFormat: string(resolved.Source(tui.KeyOutputTimeFormat)),
		Timezone:   string(resolved.Source(tui.KeyOutputTimezone)),
		Color:      string(resolved.Source(tui.KeyOutputColor)),
	}
}

// sessionInfo is what the settings screen states about this run. The target is
// taken from the redacted view of the resolved configuration, so a DSN
// carrying a password is never drawn on a screen somebody is sharing.
func (g *globals) sessionInfo(resolved *config.Resolved) tui.SessionInfo {
	path := resolved.ConfigFile
	if path == "" {
		path = g.configFilePath()
	}
	return tui.SessionInfo{
		Target:     redactedTarget(resolved),
		Tenant:     resolved.Config.Tenant,
		Version:    shortVersion(),
		ConfigFile: path,
	}
}

// shortVersion names this build in what a settings row has room for. The full
// version.String carries the build date and the platform and ran off the side
// of a 120 column terminal, truncated mid-commit.
func shortVersion() string {
	commit, dirty := version.Commit, ""
	if trimmed, cut := strings.CutSuffix(commit, "-dirty"); cut {
		commit, dirty = trimmed, "-dirty"
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	return "tix " + version.Version + " (" + commit + dirty + ")"
}

// redactedTarget names what this run is talking to, preferring the server over
// the database because a run with both configured uses the server.
func redactedTarget(resolved *config.Resolved) string {
	values := map[string]string{}
	for _, entry := range resolved.Sources() {
		values[entry.Key] = entry.Value
	}
	if url := strings.TrimSpace(values["server.url"]); url != "" {
		return url
	}
	return values["database.dsn"]
}

// savePreferences writes the settings screen's choices to the configuration
// file. It is the same in-place edit `tix ctx use` performs, for the same
// reason: writing a whole Config would pin every default and every value the
// environment happened to be supplying at that moment.
func (g *globals) savePreferences() tui.PreferenceWriter {
	return func(p tui.Preferences) error {
		path := g.configFilePath()
		existing, cfg, err := readConfigFile(path)
		if err != nil {
			return err
		}
		before := *cfg
		cfg.TUI.Keymap = p.Keymap
		cfg.Output.TimeFormat = p.TimeFormat
		cfg.Output.Timezone = p.Timezone
		cfg.Output.Color = p.Color
		if err := config.SaveChanges(path, existing, &before, cfg); err != nil {
			return core.Internal("saving configuration").Wrap(err)
		}
		return nil
	}
}
