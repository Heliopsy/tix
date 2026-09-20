package cmd

import (
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/tix/internal/config"
	"github.com/thereisnotime/tix/internal/connect"
	"github.com/thereisnotime/tix/internal/core"
)

// contextRow is one row of a context listing.
type contextRow struct {
	Name     string `json:"name" yaml:"name"`
	Current  bool   `json:"current" yaml:"current"`
	Database string `json:"database,omitempty" yaml:"database,omitempty"`
	Server   string `json:"server,omitempty" yaml:"server,omitempty"`
	Tenant   string `json:"tenant,omitempty" yaml:"tenant,omitempty"`
	Project  string `json:"project,omitempty" yaml:"project,omitempty"`
}

// newCtxCmd builds the context command group.
func newCtxCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ctx",
		Aliases: []string{"context"},
		Short:   "Manage named connection contexts",
		GroupID: "setup",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(ctxListCmd(g), ctxUseCmd(g), ctxShowCmd(g), ctxAddCmd(g), ctxRmCmd(g))
	return cmd
}

func ctxListCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configured contexts",
		Long:    "List every named context and mark the current one.\n\nExit codes: 2 unreadable configuration.",
		Example: "  tix ctx list",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := g.resolve()
			if err != nil {
				return err
			}
			names := contextNames(resolved.Config.Contexts)
			rows := make([]contextRow, 0, len(names))
			for _, name := range names {
				rows = append(rows, row(name, resolved.Config.Contexts[name], name == resolved.ContextName))
			}
			return g.render(cmd, rows)
		},
	}
}

func ctxShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "show [NAME]",
		Short:   "Show one context",
		Long:    "Show a context, defaulting to the current one.\n\nExit codes: 3 unknown context.",
		Example: "  tix ctx show\n  tix ctx show work",
		Args:    rangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := g.resolve()
			if err != nil {
				return err
			}
			name := resolved.ContextName
			if len(args) == 1 {
				name = args[0]
			}
			if name == "" {
				return core.NotFound("no context is selected")
			}
			ctx, ok := resolved.Config.Contexts[name]
			if !ok {
				return core.NotFound("context %q is not defined", name)
			}
			return g.render(cmd, row(name, ctx, name == resolved.ContextName))
		},
		ValidArgsFunction: g.completeContexts,
	}
}

func ctxUseCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "use NAME",
		Short:   "Select the current context",
		Long:    "Write the named context into the configuration file as the current one.\n\nExit codes: 3 unknown context.",
		Example: "  tix ctx use work",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.editConfig(cmd, dryRun, "ctx.use", args[0], func(cfg *config.Config) error {
				if _, ok := cfg.Contexts[args[0]]; !ok {
					return core.NotFound("context %q is not defined", args[0])
				}
				cfg.CurrentContext = args[0]
				return nil
			})
		},
		ValidArgsFunction: g.completeContexts,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func ctxAddCmd(g *globals) *cobra.Command {
	var db, server, token, tenant, project string
	var use, dryRun bool
	cmd := &cobra.Command{
		Use:     "add NAME",
		Short:   "Define a context",
		Long:    "Define a named context pointing at a database or a server.\n\nExit codes: 2 both a database and a server were given.",
		Example: "  tix ctx add work --server https://tix.example.com --use",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if db != "" && server != "" {
				return usagef(cmd, "a context names either a database or a server, not both")
			}
			entry := config.Context{Database: db, Server: server, Token: token, Tenant: tenant, Project: project}
			return g.editConfig(cmd, dryRun, "ctx.add", args[0], func(cfg *config.Config) error {
				if cfg.Contexts == nil {
					cfg.Contexts = map[string]config.Context{}
				}
				cfg.Contexts[args[0]] = entry
				if use {
					cfg.CurrentContext = args[0]
				}
				return nil
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&db, "db", "", "database dsn the context points at")
	f.StringVar(&server, "server", "", "server url the context points at")
	f.StringVar(&token, "token", "", "api token the context authenticates with")
	f.StringVar(&tenant, "tenant", "", "tenant key the context selects")
	f.StringVar(&project, "project", "", "default project key")
	f.BoolVar(&use, "use", false, "also make the context current")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func ctxRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm NAME",
		Aliases: []string{"delete"},
		Short:   "Remove a context",
		Long:    "Remove a named context from the configuration file.\n\nExit codes: 3 unknown context.",
		Example: "  tix ctx rm work",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.editConfig(cmd, dryRun, "ctx.remove", args[0], func(cfg *config.Config) error {
				if _, ok := cfg.Contexts[args[0]]; !ok {
					return core.NotFound("context %q is not defined", args[0])
				}
				delete(cfg.Contexts, args[0])
				if cfg.CurrentContext == args[0] {
					cfg.CurrentContext = ""
				}
				return nil
			})
		},
		ValidArgsFunction: g.completeContexts,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

// completeContexts offers the names of configured contexts.
func (g *globals) completeContexts(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	resolved, err := g.resolve()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveError
	}
	return contextNames(resolved.Config.Contexts), cobra.ShellCompDirectiveNoFileComp
}

// editConfig applies a change to the configuration file on disk.
func (g *globals) editConfig(cmd *cobra.Command, dryRun bool, action, target string, apply func(*config.Config) error) error {
	path := g.configFilePath()
	cfg, err := readConfigFile(path)
	if err != nil {
		return err
	}
	if err := apply(cfg); err != nil {
		return err
	}
	if dryRun {
		return g.render(cmd, newPlan(action, target, map[string]any{"file": path}))
	}
	if err := config.Save(path, cfg); err != nil {
		return core.Internal("saving configuration").Wrap(err)
	}
	g.diag(cmd, "wrote %s", path)
	return g.render(cmd, outcome{Ref: target, Status: statusOK})
}

// configFilePath returns the file that configuration changes are written to.
func (g *globals) configFilePath() string {
	if g.configPath != "" {
		return g.configPath
	}
	if resolved, err := g.resolve(); err == nil && resolved.ConfigFile != "" {
		return resolved.ConfigFile
	}
	return connect.ConfigWritePath(g.environ, lookupEnv(g.environ, "HOME"))
}

// readConfigFile loads an existing configuration file, or the defaults.
func readConfigFile(path string) (*config.Config, error) {
	cfg := config.Defaults()
	if !exists(path) {
		return &cfg, nil
	}
	file, err := config.LoadFile(path)
	if err != nil {
		return nil, err
	}
	for name, raw := range file.Values {
		key, ok := config.Lookup(name)
		if !ok {
			continue
		}
		if err := key.Set(&cfg, raw); err != nil {
			return nil, err
		}
	}
	cfg.Contexts = file.Contexts
	cfg.CurrentContext = file.CurrentContext
	return &cfg, nil
}

// exists reports whether path names a readable file.
func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// contextNames returns the configured context names in a stable order.
func contextNames(contexts map[string]config.Context) []string {
	names := make([]string, 0, len(contexts))
	for name := range contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// row renders one context for display, never exposing its token.
func row(name string, ctx config.Context, current bool) contextRow {
	return contextRow{
		Name: name, Current: current, Database: ctx.Database,
		Server: ctx.Server, Tenant: ctx.Tenant, Project: ctx.Project,
	}
}

// newConfigCmd builds the configuration command group.
func newConfigCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Inspect the resolved configuration",
		GroupID: "setup",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(configShowCmd(g))
	return cmd
}

func configShowCmd(g *globals) *cobra.Command {
	var sources bool
	cmd := &cobra.Command{
		Use:     "show",
		Short:   "Show the effective configuration",
		Long:    "Show the effective configuration, with --sources naming the layer each value came from.\n\nExit codes: 2 unreadable configuration.",
		Example: "  tix config show\n  tix config show --sources -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := g.resolve()
			if err != nil {
				return err
			}
			if sources {
				return g.render(cmd, resolved.Sources())
			}
			return g.render(cmd, resolved.Config)
		},
	}
	cmd.Flags().BoolVar(&sources, "sources", false, "report which layer supplied each value")
	return cmd
}
