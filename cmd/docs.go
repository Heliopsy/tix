package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/version"
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
func newCompletionCmd(_ *globals) *cobra.Command {
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
	return cmd
}

// newDocsCmd builds the documentation generator.
func newDocsCmd(_ *globals) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:     "docs",
		Short:   "Emit the command tree as Markdown",
		Long:    "Emit reference documentation for every command as Markdown.\n\nExit codes: 1 the output directory could not be written.",
		Example: "  tix docs > docs/cli.md\n  tix docs --dir docs/cli",
		GroupID: "setup",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dir == "" {
				return writeMarkdown(cmd.OutOrStdout(), cmd.Root())
			}
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return core.Internal("creating %q", dir).Wrap(err)
			}
			return writeMarkdownTree(dir, cmd.Root())
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "write one file per command into this directory")
	return cmd
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
