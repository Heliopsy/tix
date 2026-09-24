// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// completionShells lists the shells install understands, in the order the
// refusal message names them.
var completionShells = []string{"bash", "zsh", "fish"}

// completionLayout is where one shell loads a per-user completion script from,
// and what the user still has to do afterwards.
type completionLayout struct {
	// dir is joined onto the base directory, never onto an absolute prefix:
	// writing to /etc or /usr/share needs a privilege the command must not
	// assume it has, and a tool that writes outside $HOME unasked is a tool
	// people stop running.
	dir  []string
	file string
	// xdgConfig selects $XDG_CONFIG_HOME over $XDG_DATA_HOME as the base.
	xdgConfig bool
	note      string
	generate  func(root *cobra.Command, w *bytes.Buffer) error
}

// completionLayouts maps a shell onto its per-user completion location.
var completionLayouts = map[string]completionLayout{
	"bash": {
		dir:      []string{"bash-completion", "completions"},
		file:     "tix",
		note:     "bash-completion must be installed for bash to load it",
		generate: func(root *cobra.Command, w *bytes.Buffer) error { return root.GenBashCompletionV2(w, true) },
	},
	"zsh": {
		dir:      []string{"zsh", "site-functions"},
		file:     "_tix",
		note:     "that directory must be on your fpath",
		generate: func(root *cobra.Command, w *bytes.Buffer) error { return root.GenZshCompletion(w) },
	},
	"fish": {
		dir:       []string{"fish", "completions"},
		file:      "tix.fish",
		xdgConfig: true,
		generate:  func(root *cobra.Command, w *bytes.Buffer) error { return root.GenFishCompletion(w, true) },
	},
}

// newCompletionInstallCmd builds the completion install command.
func newCompletionInstallCmd(g *globals) *cobra.Command {
	var (
		shell     string
		dryRun    bool
		uninstall bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the completion script where the shell looks for it",
		Long: "Write the completion script into the invoking user's own completion directory for bash, " +
			"zsh or fish, and report the path.\n\n" +
			"The shell is taken from $SHELL unless --shell names one. No startup file is ever edited: " +
			"appending to somebody's .zshrc is the kind of help that gets found months later in a bisect.\n\n" +
			"Exit codes: 1 the file could not be written, 2 an unsupported or undetectable shell.",
		Example: "  tix completion install\n  tix completion install --shell zsh\n" +
			"  tix completion install --dry-run\n  tix completion install --uninstall",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, err := resolveCompletionShell(cmd, shell, lookupEnv(g.environ, "SHELL"))
			if err != nil {
				return err
			}
			layout := completionLayouts[name]
			path, err := completionPath(g.environ, layout)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			switch {
			case uninstall:
				return removeCompletion(out, path)
			case dryRun:
				_, err := fmt.Fprintf(out, "would write %s\n", path)
				return err
			default:
				return writeCompletion(out, cmd.Root(), layout, path)
			}
		},
	}
	f := cmd.Flags()
	f.StringVar(&shell, "shell", "", "shell to install for instead of the one $SHELL names")
	f.BoolVar(&dryRun, "dry-run", false, "report the path that would be written and write nothing")
	f.BoolVar(&uninstall, "uninstall", false, "remove the script this command writes")
	_ = cmd.RegisterFlagCompletionFunc("shell", fixedCompletion(completionShells))
	return cmd
}

// resolveCompletionShell picks the shell to install for. $SHELL is the login
// shell rather than the one running the command, so --shell wins outright.
func resolveCompletionShell(cmd *cobra.Command, flag, env string) (string, error) {
	name := flag
	if name == "" {
		name = filepath.Base(env)
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "", usagef(cmd, "no shell given and $SHELL names none; use --shell with %s",
			strings.Join(completionShells, ", "))
	}
	if _, ok := completionLayouts[name]; !ok {
		return "", usagef(cmd, "shell %q is not supported; use %s", name, strings.Join(completionShells, ", "))
	}
	return name, nil
}

// completionPath resolves the file one layout owns under the user's own
// directories.
func completionPath(environ []string, layout completionLayout) (string, error) {
	base := lookupEnv(environ, "XDG_DATA_HOME")
	fallback := filepath.Join(".local", "share")
	if layout.xdgConfig {
		base, fallback = lookupEnv(environ, "XDG_CONFIG_HOME"), ".config"
	}
	if base == "" {
		home := lookupEnv(environ, "HOME")
		if home == "" {
			return "", core.Invalid("neither HOME nor the XDG base directory is set")
		}
		base = filepath.Join(home, fallback)
	}
	return filepath.Join(append([]string{base}, append(layout.dir, layout.file)...)...), nil
}

// writeCompletion renders the script and replaces the file at path. The write
// is a plain truncating one so a second install leaves the same bytes as the
// first rather than appending to what is already there.
func writeCompletion(out io.Writer, root *cobra.Command, layout completionLayout, path string) error {
	var buf bytes.Buffer
	if err := layout.generate(root, &buf); err != nil {
		return core.Internal("generating the completion script").Wrap(err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return core.Internal("creating %q", dir).Wrap(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return core.Internal("writing %q", path).Wrap(err)
	}
	if _, err := fmt.Fprintf(out, "wrote %s\n", path); err != nil {
		return err
	}
	if layout.note == "" {
		return nil
	}
	_, err := fmt.Fprintf(out, "%s\n", layout.note)
	return err
}

// removeCompletion deletes the file this command writes. An absent file is
// reported rather than failed: uninstalling twice, or uninstalling a shell that
// was never installed for, is not a mistake worth a non-zero exit.
func removeCompletion(out io.Writer, path string) error {
	err := os.Remove(path)
	switch {
	case err == nil:
		_, err = fmt.Fprintf(out, "removed %s\n", path)
		return err
	case errors.Is(err, fs.ErrNotExist):
		_, err = fmt.Fprintf(out, "nothing to remove at %s\n", path)
		return err
	default:
		return core.Internal("removing %q", path).Wrap(err)
	}
}
