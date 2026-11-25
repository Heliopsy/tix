// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sshd"
	"github.com/heliopsy/tix/internal/tui"
	"github.com/spf13/cobra"
)

// localOnlyFlags pick a target, a credential or a file on the machine running
// the command. Over SSH that machine is the server, so each is refused.
var localOnlyFlags = []string{
	"db", "server", "token", "ctx", "config", "tenant", "no-discovery", "allow-network-fs",
	"log-level", "log-format", "log-output", "log-max-size-mb", "log-max-age", "log-max-backups", "log-compress",
}

// hostedKey marks the context of a command run over the SSH listener.
type hostedKey struct{}

// isHosted reports whether cmd runs on behalf of an SSH session.
func isHosted(cmd *cobra.Command) bool {
	ctx := cmd.Context()
	if ctx == nil {
		return false
	}
	hosted, _ := ctx.Value(hostedKey{}).(bool)
	return hosted
}

// localFile refuses a path on the serving machine when cmd runs over SSH.
// Every place a command opens, reads, writes or inspects a path calls it
// first; TestEveryFileCallIsGuarded fails on one that does not.
func localFile(cmd *cobra.Command, what, path string) error {
	if path == "" || path == StdinMarker || !isHosted(cmd) {
		return nil
	}
	return core.Invalid("%s names a file, and over ssh the server opens no file of yours; "+
		"pass - (or leave it out) and redirect instead: ssh HOST %s ... < FILE, or > FILE",
		what, strings.TrimPrefix(cmd.CommandPath(), "tix "))
}

// goGuarded starts fn in a goroutine. Over SSH a panic in it would end every
// session the server holds, because no recover above a goroutine can catch
// what happens inside it, so there the panic is recovered and the goroutine
// ends. Run locally it panics as any goroutine would.
// TestEveryGoroutineIsGuarded fails on a bare go statement in cmd/.
func goGuarded(ctx context.Context, fn func()) {
	hosted := false
	if ctx != nil {
		hosted, _ = ctx.Value(hostedKey{}).(bool)
	}
	go func() {
		if hosted {
			defer func() { _ = recover() }()
		}
		fn()
	}()
}

// RunHosted runs one tix command for an SSH session, in this process, as the
// session's actor, and returns its exit code. It is the listener's
// sshd.CommandRunner.
//
// The command tree is the one the binary runs, pruned to the commands the
// capability registry hosts over SSH. Its connection is set before anything
// executes, so no command can open a database, dial a server or read a token,
// and its configuration is the defaults alone: the serving machine's files and
// environment belong to its operator, not to the caller.
func RunHosted(ctx context.Context, inv sshd.Invocation) int {
	root, g := newRoot([]string{}, "")
	g.resolved = config.Defaulted()
	g.conn = connect.Hosted(inv.Service, inv.Actor, inv.Via, inv.Tenant)
	pruneForSSH(root, capability.SSHCommands())
	for _, name := range localOnlyFlags {
		_ = root.PersistentFlags().MarkHidden(name)
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpCommand(&cobra.Command{Use: "no-help", Hidden: true})
	root.SetArgs(inv.Args)
	root.SetIn(inv.Stdin)
	root.SetOut(inv.Stdout)
	root.SetErr(inv.Stderr)

	if err := checkHosted(root, inv.Args); err != nil {
		_, _ = fmt.Fprintf(inv.Stderr, "tix: %v\n", err)
		return core.ExitUsage
	}
	err := root.ExecuteContext(context.WithValue(ctx, hostedKey{}, true))
	return g.exitCode(err, inv.Stderr)
}

// checkHosted refuses an invocation the pruned tree should not even resolve:
// one whose first word is not a hosted command, or that resolves to anything
// but a hosted command or a group above one. The pruning is the enforcement;
// this check names the refusal and holds the line if the pruning ever slips.
func checkHosted(root *cobra.Command, args []string) error {
	allowed := capability.SSHCommands()
	if len(args) > 0 {
		if _, ok := tui.LookupInterface(args[0]); ok {
			return core.Invalid("%q is an interface the listener opens itself, not a command", args[0])
		}
	}
	if len(args) == 0 || !slices.Contains(topWords(allowed), args[0]) {
		word := ""
		if len(args) > 0 {
			word = args[0]
		}
		return core.Invalid("%q is not a command this listener runs over ssh; it runs %s",
			word, strings.Join(topWords(allowed), ", "))
	}
	found, _, err := root.Find(args)
	if err != nil {
		return core.Invalid("%v", err)
	}
	path := strings.TrimPrefix(found.CommandPath(), "tix ")
	if slices.Contains(allowed, path) || (!found.Runnable() && keeps(allowed, path)) {
		return nil
	}
	return core.Invalid("%q is not a command this listener runs over ssh", path)
}

// pruneForSSH removes every command that is neither hosted nor a group above
// a hosted command, so the tree cannot resolve it at all.
func pruneForSSH(c *cobra.Command, allowed []string) {
	for _, child := range c.Commands() {
		path := strings.TrimPrefix(child.CommandPath(), "tix ")
		if !keeps(allowed, path) {
			c.RemoveCommand(child)
			continue
		}
		pruneForSSH(child, allowed)
	}
}

// keeps reports whether path is hosted or lies above a hosted command.
func keeps(allowed []string, path string) bool {
	for _, a := range allowed {
		if a == path || strings.HasPrefix(a, path+" ") {
			return true
		}
	}
	return false
}

// topWords are the distinct first words of the hosted commands, sorted.
func topWords(allowed []string) []string {
	var out []string
	for _, a := range allowed {
		word, _, _ := strings.Cut(a, " ")
		if !slices.Contains(out, word) {
			out = append(out, word)
		}
	}
	slices.Sort(out)
	return out
}

// refuseLocalFlags refuses the flags that pick something on the serving
// machine. It reads cobra's own record of what was set, after parsing, so a
// flag is judged the way the command line read it.
func refuseLocalFlags(cmd *cobra.Command) error {
	if !isHosted(cmd) {
		return nil
	}
	for _, name := range localOnlyFlags {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
			return usagef(cmd, "--%s picks a target, credential or file on the server and is refused over ssh; "+
				"an ssh session works in its key's tenant", name)
		}
	}
	return nil
}

// CommandWords lists the first word of every command and alias the command
// line has, with cobra's own help and completion entries, for the listener to
// answer as commands. Most are refused there; the point is that a session
// naming one is told so rather than having it read as a project key.
func CommandWords() []string {
	root, _ := newRoot([]string{}, "")
	words := []string{"help", "completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd}
	for _, c := range root.Commands() {
		words = append(words, c.Name())
		words = append(words, c.Aliases...)
	}
	return words
}

// maxImportBytes is the same server.max_import_bytes an import over HTTP
// answers to.
func (g *globals) maxImportBytes() int64 {
	if g.resolved == nil {
		return config.DefaultMaxImportBytes
	}
	return g.resolved.Config.Server.MaxImportBytes
}

// maxInputBytes is the most standard input a hosted command may read: the
// import limit, or artifacts.max_bytes where that is larger, so tix artifact
// put --file - is never refused a file the artifact limit allows.
func (g *globals) maxInputBytes() int64 {
	if g.resolved == nil {
		return max(config.DefaultMaxImportBytes, core.DefaultArtifactMaxBytes)
	}
	return max(g.maxImportBytes(), g.resolved.Config.Artifacts.MaxBytes)
}
