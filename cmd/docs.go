// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/version"
	"github.com/spf13/cobra"
)

// buildInfo is the structured form of the version banner.
type buildInfo struct {
	Version   string `json:"version" yaml:"version"`
	Commit    string `json:"commit" yaml:"commit"`
	BuildDate string `json:"build_date" yaml:"build_date"`
	GoVersion string `json:"go_version" yaml:"go_version"`
	OS        string `json:"os" yaml:"os"`
	Arch      string `json:"arch" yaml:"arch"`
}

// versionInfo returns the build metadata this binary was stamped with.
func versionInfo() buildInfo {
	return buildInfo{
		Version:   version.Version,
		Commit:    version.Commit,
		BuildDate: version.Date,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// newVersionCmd builds the version command.
func newVersionCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print version information",
		Long:    "Print the version banner, or structured build metadata with -o json.\n\nExit codes: 0 always.",
		Example: "  tix version\n  tix version -o json",
		GroupID: "setup",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.formatName() == "table" {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), version.String())
				return err
			}
			return g.render(cmd, versionInfo())
		},
	}
}

// newCompletionCmd builds the shell completion command.
func newCompletionCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion SHELL",
		Short: "Generate a shell completion script",
		Long: "Generate a completion script for bash, zsh or fish. The script offers dynamic completion of " +
			"task references, project keys, tags and statuses.\n\nExit codes: 2 unsupported shell.",
		Example: "  tix completion bash > /etc/bash_completion.d/tix\n  tix completion zsh > \"${fpath[1]}/_tix\"",
		GroupID: "setup",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			out := cmd.OutOrStdout()
			switch strings.ToLower(args[0]) {
			case "bash":
				return root.GenBashCompletionV2(out, true)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			default:
				return usagef(cmd, "shell %q is not supported; use bash, zsh or fish", args[0])
			}
		},
		ValidArgsFunction: fixedCompletion([]string{"bash", "zsh", "fish"}),
	}
	cmd.AddCommand(newCompletionInstallCmd(g))
	return cmd
}

// newDocsCmd builds the documentation generator.
func newDocsCmd(_ *globals) *cobra.Command {
	var (
		dir   string
		table bool
		depth int
	)
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Emit the command tree as Markdown",
		Long: "Emit reference documentation for every command as Markdown.\n\n" +
			"--table emits the command summary table README.md carries, grouped the way " +
			"the root help groups commands.\n\n" +
			"Exit codes: 1 the output directory could not be written, 2 an unusable flag combination.",
		Example: "  tix docs > docs/cli.md\n  tix docs --dir docs/cli\n  tix docs --table --depth 2",
		GroupID: "setup",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if table {
				if dir != "" {
					return usagef(cmd, "--table writes one document and cannot be combined with --dir")
				}
				if depth < 1 {
					return usagef(cmd, "--depth must be at least 1")
				}
				return writeCommandTable(cmd.OutOrStdout(), cmd.Root(), depth)
			}
			if dir == "" {
				return writeMarkdown(cmd.OutOrStdout(), cmd.Root())
			}
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return core.Internal("creating %q", dir).Wrap(err)
			}
			return writeMarkdownTree(dir, cmd.Root())
		},
	}
	f := cmd.Flags()
	f.StringVar(&dir, "dir", "", "write one file per command into this directory")
	f.BoolVar(&table, "table", false, "emit the grouped command summary table instead of full pages")
	f.IntVar(&depth, "depth", 1, "levels of the command tree the table covers")
	return cmd
}

// writeCommandTable renders the command tree as one Markdown table per group.
func writeCommandTable(w io.Writer, root *cobra.Command, depth int) error {
	b := &strings.Builder{}
	for _, group := range root.Groups() {
		rows := tableRows(root, group.ID, depth)
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(b, "### %s\n\n", strings.TrimSuffix(group.Title, ":"))
		b.WriteString("| Command | Does |\n| --- | --- |\n")
		for _, r := range rows {
			fmt.Fprintf(b, "| `%s` | %s |\n", r.path, r.short)
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(w, strings.TrimRight(b.String(), "\n")+"\n")
	return err
}

// tableRow is one command's line in the summary table.
type tableRow struct {
	path  string
	short string
}

// tableRows collects the commands of one group down to the requested depth.
func tableRows(root *cobra.Command, groupID string, depth int) []tableRow {
	var rows []tableRow
	var walk func(cmd *cobra.Command, level int)
	walk = func(cmd *cobra.Command, level int) {
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() || sub.Name() == "help" {
				continue
			}
			rows = append(rows, tableRow{path: sub.CommandPath(), short: sub.Short})
			if level < depth {
				walk(sub, level+1)
			}
		}
	}
	for _, top := range root.Commands() {
		if !top.IsAvailableCommand() || top.GroupID != groupID {
			continue
		}
		rows = append(rows, tableRow{path: top.CommandPath(), short: top.Short})
		if depth > 1 {
			walk(top, 2)
		}
	}
	return rows
}

// writeMarkdownTree writes one Markdown file per command.
func writeMarkdownTree(dir string, cmd *cobra.Command) error {
	name := strings.ReplaceAll(cmd.CommandPath(), " ", "_") + ".md"
	path := filepath.Join(dir, name)
	f, err := os.Create(path) // #nosec G304 -- the directory is chosen by the user
	if err != nil {
		return core.Internal("creating %q", path).Wrap(err)
	}
	if err := writeSection(f, cmd); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return core.Internal("closing %q", path).Wrap(err)
	}
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() {
			continue
		}
		if err := writeMarkdownTree(dir, sub); err != nil {
			return err
		}
	}
	return nil
}

// writeMarkdown writes the whole tree as one Markdown document.
func writeMarkdown(w io.Writer, cmd *cobra.Command) error {
	if err := writeSection(w, cmd); err != nil {
		return err
	}
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() {
			continue
		}
		if err := writeMarkdown(w, sub); err != nil {
			return err
		}
	}
	return nil
}

// writeSection renders one command as a Markdown section.
func writeSection(w io.Writer, cmd *cobra.Command) error {
	depth := strings.Count(cmd.CommandPath(), " ") + 1
	b := &strings.Builder{}
	fmt.Fprintf(b, "%s %s\n\n", strings.Repeat("#", depth), cmd.CommandPath())
	if cmd.Short != "" {
		fmt.Fprintf(b, "%s\n\n", cmd.Short)
	}
	if cmd.Long != "" && cmd.Long != cmd.Short {
		fmt.Fprintf(b, "%s\n\n", cmd.Long)
	}
	fmt.Fprintf(b, "```\n%s\n```\n\n", cmd.UseLine())
	if cmd.Example != "" {
		fmt.Fprintf(b, "Examples:\n\n```\n%s\n```\n\n", strings.TrimRight(cmd.Example, "\n"))
	}
	if flags := cmd.NonInheritedFlags().FlagUsages(); strings.TrimSpace(flags) != "" {
		fmt.Fprintf(b, "Flags:\n\n```\n%s```\n\n", flags)
	}
	if inherited := cmd.InheritedFlags().FlagUsages(); strings.TrimSpace(inherited) != "" {
		fmt.Fprintf(b, "Global flags:\n\n```\n%s```\n\n", inherited)
	}
	if codes := cmd.Root().Annotations["exitCodes"]; codes != "" && depth == 1 {
		fmt.Fprintf(b, "Exit codes: %s\n\n", codes)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// itoa renders an integer for a diagnostic string.
func itoa(n int) string { return strconv.Itoa(n) }
