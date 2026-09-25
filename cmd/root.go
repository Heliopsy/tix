// SPDX-License-Identifier: AGPL-3.0-or-later

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
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/logging"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
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
	tenant      string
	token       string
	format      string
	color       bool
	noColor     bool
	quiet       bool
	verbose     bool
	noDiscovery bool

	// drainMode is the webhook drain mode this process prefers while the
	// operator has left the key on its default layer.
	drainMode string

	// allowNetworkFS opts into opening a database detected on a network
	// filesystem, which is refused by default because it corrupts SQLite.
	allowNetworkFS bool

	// log holds the flag layer of the logging keys. Rotation is not gated
	// behind `tix serve`: a destination is a property of the process, and an
	// operator who points TIX_LOG_OUTPUT at a file expects every command that
	// logs to land there. The file is only opened by a command that asks for a
	// logger, so `tix task add` never creates one.
	log       logFlags
	logCloser io.Closer

	environ []string
	dir     string

	resolved *config.Resolved
	conn     *connect.Conn
}

// logFlags are the logging settings as the command line carries them.
type logFlags struct {
	// changed reports whether the operator actually typed a flag, which is
	// what separates the flag layer from a declared default.
	changed    func(string) bool
	level      string
	format     string
	output     string
	maxSizeMB  int
	maxAge     time.Duration
	maxBackups int
	compress   bool
}

// values renders the flags the operator actually typed as a configuration
// layer. A flag nobody gave carries its declared default, which would
// otherwise silently outrank every other layer.
func (l logFlags) values() map[string]string {
	out := map[string]string{}
	if l.changed == nil {
		return out
	}
	for _, f := range []struct {
		flag string
		key  string
		val  func() string
	}{
		{"log-level", "log.level", func() string { return l.level }},
		{"log-format", "log.format", func() string { return l.format }},
		{"log-output", "log.output", func() string { return l.output }},
		{"log-max-size-mb", "log.file.max_size_mb", func() string { return strconv.Itoa(l.maxSizeMB) }},
		{"log-max-age", "log.file.max_age", func() string { return l.maxAge.String() }},
		{"log-max-backups", "log.file.max_backups", func() string { return strconv.Itoa(l.maxBackups) }},
		{"log-compress", "log.file.compress", func() string { return strconv.FormatBool(l.compress) }},
	} {
		if l.changed(f.flag) {
			out[f.key] = f.val()
		}
	}
	return out
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
	if g.logCloser != nil {
		_ = g.logCloser.Close()
	}
	if err == nil {
		return core.ExitOK
	}

	var child *exitError
	if errors.As(err, &child) {
		return child.code
	}

	label := output.NewPainter(g.colorMode(), errw).Error("error:")
	var usage *usageError
	switch {
	case errors.As(err, &usage):
		_, _ = fmt.Fprintln(errw, usage.cmd.UsageString())
	case strings.HasPrefix(err.Error(), "unknown command"), strings.HasPrefix(err.Error(), "unknown flag"),
		strings.HasPrefix(err.Error(), "unknown shorthand"):
		_, _ = fmt.Fprintf(errw, "%s %v\n", label, err)
		return core.ExitUsage
	}
	_, _ = fmt.Fprintf(errw, "%s %v\n", label, err)
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
			"exitCodes": "0 success, 1 error, 2 usage, 3 not found, 4 conflict, 5 permission, 6 precondition, 7 upstream",
		},
		SilenceErrors:     true,
		SilenceUsage:      true,
		PersistentPreRunE: g.validateGlobals,
	}
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &usageError{cmd: cmd, err: core.Invalid("%v", err)}
	})

	f := root.PersistentFlags()
	f.StringVar(&g.configPath, "config", "", "configuration file to use")
	f.StringVar(&g.contextName, "ctx", "", "named context to use")
	f.StringVar(&g.db, "db", "", "database dsn to use instead of the configured target")
	f.StringVar(&g.server, "server", "", "server url to use instead of the configured target")
	f.StringVar(&g.tenant, "tenant", "", "tenant key to work in instead of the configured one")
	f.StringVar(&g.token, "token", "", "api token to authenticate with")
	f.StringVarP(&g.format, "output", "o", "", "output format: "+strings.Join(output.Formats, "|"))
	f.BoolVar(&g.noColor, "no-color", false, "disable coloured output")
	f.BoolVar(&g.color, "color", false, "force coloured output, even when not writing to a terminal")
	f.BoolVarP(&g.quiet, "quiet", "q", false, "suppress diagnostics")
	f.BoolVarP(&g.verbose, "verbose", "v", false, "report how the target was resolved")
	f.BoolVar(&g.noDiscovery, "no-discovery", false, "ignore per-directory context files")
	f.BoolVar(&g.allowNetworkFS, "allow-network-fs", false, "allow opening a database detected on a network filesystem, which risks corruption")
	registerLogFlags(root, &g.log)

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
	newTenantCmd, newDomainCmd, newMemberCmd, newActorCmd, newUserCmd, newTokenCmd, newLoginCmd,
	newCtxCmd, newConfigCmd, newDoctorCmd, newPruneCmd, newRetentionCmd, newWebhookCmd,
	newAuditCmd, newSyncCmd, newArtifactCmd, newWatchCmd, newLogoutCmd, newConnectionCmd,
	newDocsCmd, newCompletionCmd, newVersionCmd,
}

// validateGlobals rejects contradictory global flags before anything is opened.
func (g *globals) validateGlobals(cmd *cobra.Command, _ []string) error {
	if g.color && g.noColor {
		return usagef(cmd, "--color and --no-color contradict each other")
	}
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
	if g.tenant != "" {
		flags["tenant"] = g.tenant
	}
	if tok := g.bearer(); tok != "" {
		flags["server.token"] = tok
	}
	if mode := g.colorFlag(); mode != "" {
		flags[config.KeyOutputColor] = mode
	}
	for key, value := range g.log.values() {
		flags[key] = value
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

// registerLogFlags declares the logging flags, so every logging key has the
// flag layer the other keys have rather than being configuration-only.
func registerLogFlags(cmd *cobra.Command, l *logFlags) {
	f := cmd.PersistentFlags()
	defaults := config.Defaults().Log
	l.changed = f.Changed
	f.StringVar(&l.level, "log-level", config.DefaultLogLevel,
		"log level: "+strings.Join(config.LogLevels, "|"))
	f.StringVar(&l.format, "log-format", config.DefaultLogFormat,
		"log format: "+strings.Join(config.LogFormats, "|"))
	f.StringVar(&l.output, "log-output", config.DefaultLogOutput,
		"where logs go: stderr, stdout or a file path to rotate")
	f.IntVar(&l.maxSizeMB, "log-max-size-mb", config.DefaultLogFileMaxSizeMB,
		"size one log file may reach before it is rotated")
	f.DurationVar(&l.maxAge, "log-max-age", time.Duration(defaults.File.MaxAge),
		"how long a rotated log file is kept (0 keeps it until the backup count evicts it)")
	f.IntVar(&l.maxBackups, "log-max-backups", config.DefaultLogFileMaxBackups,
		"how many rotated log files are kept (0 keeps every file still inside the age)")
	f.BoolVar(&l.compress, "log-compress", defaults.File.Compress, "gzip a rotated log file")
}

// logger builds the process logger from the resolved configuration, opening a
// file destination at most once per invocation. The closer is released by Run.
func (g *globals) logger(cmd *cobra.Command) (*slog.Logger, error) {
	resolved, err := g.resolve()
	if err != nil {
		return nil, err
	}
	cfg := resolved.Config.Log
	log, closer, err := logging.New(logging.Options{
		Level:  cfg.Level,
		Format: cfg.Format,
		Output: cfg.Output,
		Stderr: cmd.ErrOrStderr(),
		Stdout: cmd.OutOrStdout(),
		File: logging.FileOptions{
			MaxSizeMB:  cfg.File.MaxSizeMB,
			MaxAge:     cfg.File.MaxAge,
			MaxBackups: cfg.File.MaxBackups,
			Compress:   cfg.File.Compress,
		},
		Clock: clock.New(),
	})
	if err != nil {
		return nil, err
	}
	g.logCloser = closer
	return log, nil
}

// colorFlag returns the colour mode requested on the command line, if any.
func (g *globals) colorFlag() string {
	switch {
	case g.noColor:
		return output.ColorNever
	case g.color:
		return output.ColorAlways
	default:
		return ""
	}
}

// colorMode returns the colour mode for this invocation. It prefers the fully
// resolved configuration, and falls back to flags and the environment for the
// failures reported before configuration could be loaded.
func (g *globals) colorMode() output.Mode {
	if name := g.colorFlag(); name != "" {
		mode, _ := output.ParseMode(name)
		return mode
	}
	if g.resolved != nil {
		if mode, ok := output.ParseMode(g.resolved.Config.Output.Color); ok {
			return mode
		}
	}
	if name := lookupEnv(g.environ, config.EnvName(config.KeyOutputColor)); name != "" {
		if mode, ok := output.ParseMode(name); ok {
			return mode
		}
	}
	if output.NoColorSet(func(name string) string { return lookupEnv(g.environ, name) }) {
		return output.ModeNever
	}
	return output.ModeAuto
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
		DB:             g.db,
		Server:         g.server,
		Token:          g.bearer(),
		Home:           lookupEnv(g.environ, "HOME"),
		Environ:        g.environ,
		DrainMode:      g.drainMode,
		AllowNetworkFS: g.allowNetworkFS,
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
