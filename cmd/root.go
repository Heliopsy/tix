// Package cmd contains all Cobra CLI commands for tix.
//
// This layer holds flag definitions and wiring only. All business logic lives
// in internal/; no package under internal/ may import Cobra.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/tix/internal/config"
	"github.com/thereisnotime/tix/internal/connect"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/output"
)

// EnvToken names the variable holding a personal access token.
const EnvToken = "TIX_TOKEN"

// globals holds every flag that applies to every command, plus the process
// environment so the whole tree can be exercised without touching the machine.
type globals struct {
	configPath  string
	contextName string
	db          string
	server      string
	token       string
	format      string
	quiet       bool
	verbose     bool
	noDiscovery bool

	environ []string
	dir     string

	resolved *config.Resolved
	conn     *connect.Conn
}

// usageError marks a failure that should print usage and exit 2.
type usageError struct {
	cmd *cobra.Command
	err error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// usagef builds a usage error for a command.
func usagef(cmd *cobra.Command, format string, args ...any) error {
	return &usageError{cmd: cmd, err: core.Invalid(format, args...)}
}

// Execute runs the CLI against the real process and exits with its code.
func Execute() {
	os.Exit(Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Environ(), ""))
}

// Run executes one invocation and returns the process exit code.
func Run(args []string, in io.Reader, out, errw io.Writer, environ []string, dir string) int {
	root, g := newRoot(environ, dir)
	root.SetArgs(args)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errw)

	err := root.Execute()
	if g.conn != nil {
		_ = g.conn.Close()
	}
	if err == nil {
		return core.ExitOK
	}

	var child *exitError
	if errors.As(err, &child) {
		return child.code
	}

	var usage *usageError
	switch {
	case errors.As(err, &usage):
		_, _ = fmt.Fprintln(errw, usage.cmd.UsageString())
	case strings.HasPrefix(err.Error(), "unknown command"), strings.HasPrefix(err.Error(), "unknown flag"),
		strings.HasPrefix(err.Error(), "unknown shorthand"):
		_, _ = fmt.Fprintf(errw, "error: %v\n", err)
		return core.ExitUsage
	}
	_, _ = fmt.Fprintf(errw, "error: %v\n", err)
	return core.KindOf(err).ExitCode()
}

// newRoot builds a fresh command tree bound to the supplied environment.
func newRoot(environ []string, dir string) (*cobra.Command, *globals) {
	g := &globals{environ: environ, dir: dir}
	root := &cobra.Command{
		Use:   "tix",
		Short: "Task management for humans and AI agents",
		Long: "tix manages tasks for humans and AI agents over one shared store, " +
			"via CLI, TUI, HTTP API, WebSocket, and web UI.",
		Example: "  tix task add \"buy milk\"\n  tix task ls -o json\n  tix claim next",
		Annotations: map[string]string{
			"exitCodes": "0 success, 1 error, 2 usage, 3 not found, 4 conflict, 5 permission, 6 precondition",
		},
		SilenceErrors:     true,
		SilenceUsage:      true,
		PersistentPreRunE: g.validateFormat,
	}
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &usageError{cmd: cmd, err: core.Invalid("%v", err)}
	})

	f := root.PersistentFlags()
	f.StringVar(&g.configPath, "config", "", "configuration file to use")
	f.StringVar(&g.contextName, "ctx", "", "named context to use")
	f.StringVar(&g.db, "db", "", "database dsn to use instead of the configured target")
	f.StringVar(&g.server, "server", "", "server url to use instead of the configured target")
	f.StringVar(&g.token, "token", "", "api token to authenticate with")
	f.StringVarP(&g.format, "output", "o", "", "output format: "+strings.Join(output.Formats, "|"))
	f.BoolVarP(&g.quiet, "quiet", "q", false, "suppress diagnostics")
	f.BoolVarP(&g.verbose, "verbose", "v", false, "report how the target was resolved")
	f.BoolVar(&g.noDiscovery, "no-discovery", false, "ignore per-directory context files")

	_ = root.RegisterFlagCompletionFunc("output", fixedCompletion(output.Formats))

	root.AddGroup(
		&cobra.Group{ID: "work", Title: "Working with tasks:"},
		&cobra.Group{ID: "admin", Title: "Administration:"},
		&cobra.Group{ID: "setup", Title: "Configuration and tooling:"},
	)

	for _, add := range builders {
		root.AddCommand(add(g))
	}
	return root, g
}

// builders are the command groups that make up the tree.
var builders = []func(*globals) *cobra.Command{
	newTaskCmd, newDepCmd, newTagCmd, newCommentCmd,
	newProjectCmd, newWorkflowCmd, newFieldCmd, newClaimCmd,
	newTenantCmd, newDomainCmd, newMemberCmd, newUserCmd, newTokenCmd, newLoginCmd,
	newCtxCmd, newConfigCmd, newDoctorCmd, newPruneCmd, newWebhookCmd, newAuditCmd, newSyncCmd,
	newDocsCmd, newCompletionCmd, newVersionCmd,
}

// validateFormat rejects an unsupported -o value before anything is opened.
func (g *globals) validateFormat(cmd *cobra.Command, _ []string) error {
	if g.format == "" {
		return nil
	}
	if !slices.Contains(output.Formats, g.format) {
		return usagef(cmd, "output format %q is not supported; use one of %s",
			g.format, strings.Join(output.Formats, ", "))
	}
	return nil
}

// resolve loads the configuration once per invocation.
func (g *globals) resolve() (*config.Resolved, error) {
	if g.resolved != nil {
		return g.resolved, nil
	}
	environ := slices.Clone(g.environ)
	if g.configPath != "" {
		environ = append(environ, config.EnvConfigFile+"="+g.configPath)
	}
	flags := map[string]string{}
	if g.db != "" {
		flags["database.dsn"] = g.db
	}
	if g.server != "" {
		flags["server.url"] = g.server
	}
	if tok := g.bearer(); tok != "" {
		flags["server.token"] = tok
	}
	resolved, err := config.Load(config.Options{
		Dir:         g.dir,
		Environ:     environ,
		Flags:       flags,
		Context:     g.contextName,
		NoDiscovery: g.noDiscovery,
	})
	if err != nil {
		return nil, err
	}
	g.resolved = resolved
	return resolved, nil
}

// bearer returns the token supplied by flag or by the environment.
func (g *globals) bearer() string {
	if strings.TrimSpace(g.token) != "" {
		return g.token
	}
	return lookupEnv(g.environ, EnvToken)
}

// overrides builds the raw selectors handed to the resolver.
func (g *globals) overrides() connect.Overrides {
	return connect.Overrides{
		DB:      g.db,
		Server:  g.server,
		Token:   g.bearer(),
		Home:    lookupEnv(g.environ, "HOME"),
		Environ: g.environ,
	}
}

// dial opens the single connection this invocation uses.
func (g *globals) dial(cmd *cobra.Command) (*connect.Conn, context.Context, error) {
	if g.conn != nil {
		return g.conn, g.conn.Context(cmd.Context()), nil
	}
	resolved, err := g.resolve()
	if err != nil {
		return nil, nil, err
	}
	conn, err := connect.Dial(cmd.Context(), resolved, g.overrides())
	if err != nil {
		return nil, nil, err
	}
	g.conn = conn
	if g.verbose {
		g.diag(cmd, "target: %s", conn.Info.Target.Describe())
	}
	return conn, conn.Context(cmd.Context()), nil
}

// lookupEnv returns the value of name from an environ slice.
func lookupEnv(environ []string, name string) string {
	prefix := name + "="
	for i := len(environ) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(environ[i], prefix); ok {
			return value
		}
	}
	return ""
}
