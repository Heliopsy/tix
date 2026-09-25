// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/selfupdate"
	"github.com/heliopsy/tix/internal/version"
	"github.com/spf13/cobra"
)

// newUpdateCmd builds the self-update command.
func newUpdateCmd(g *globals) *cobra.Command {
	var check bool
	var want string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Replace this binary with a published release",
		Long: "Replace this binary with a release built for this platform.\n\n" +
			"The archive is checked against the checksum published with it, then written\n" +
			"beside this binary and moved over it, so an interrupted update cannot leave a\n" +
			"truncated file where tix was.\n\n" +
			"What that checksum proves: the download is the file the release published.\n" +
			"It is not a supply-chain guarantee, because the archive and the checksum come\n" +
			"from the same host; the trust anchor is TLS to that host. See docs/upgrading.md.\n\n" +
			"A binary installed by the Go toolchain or owned by a package manager is not\n" +
			"this command's to replace, and it says which command to use instead.\n\n" +
			"Exit codes: 1 the release could not be fetched or installed, 2 this binary is\n" +
			"not one this command may replace.",
		Example: "  tix update\n  tix update --check\n  tix update --version v0.2.0",
		Args:    noArgs,
		GroupID: "admin",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUpdate(cmd, g, check, want)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&check, "check", false, "report whether a newer release exists, without installing it")
	f.StringVar(&want, "version", "", "install this release instead of the newest, including an older one")
	return cmd
}

func runUpdate(cmd *cobra.Command, g *globals, check bool, want string) error {
	out := cmd.OutOrStdout()
	client := selfupdate.Client{}
	ctx := cmd.Context()

	target, err := os.Executable()
	if err != nil {
		return core.Internal("finding this binary").Wrap(err)
	}

	// Which release, before anything is refused: --check must work on a
	// binary this command could not replace, because "is there a newer one"
	// is a fair question whoever installed it.
	release := want
	if release == "" {
		latest, err := client.Latest(ctx)
		if err != nil {
			return core.Internal("looking up the newest release").Wrap(err)
		}
		release = latest.Tag
	}

	current := version.Version
	if check {
		if want == "" && !selfupdate.Newer(current, release) {
			_, _ = fmt.Fprintf(out, "tix %s is the newest release\n", current)
			return nil
		}
		_, _ = fmt.Fprintf(out, "tix %s is available; this is %s\n", release, current)
		return nil
	}

	if want == "" && !selfupdate.Newer(current, release) {
		_, _ = fmt.Fprintf(out, "tix %s is already the newest release\n", current)
		return nil
	}

	// Refusals before the download. Finding out afterwards wastes it and,
	// worse, reports the problem at the moment the binary is being replaced,
	// which reads like the replacement half-happened.
	if err := refuseUnlessOurs(target); err != nil {
		return err
	}
	if err := selfupdate.Writable(target); err != nil {
		return core.Invalid("%s", err)
	}

	binary, err := client.Fetch(ctx, release)
	if err != nil {
		return core.Internal("fetching %s", release).Wrap(err)
	}
	if err := selfupdate.Replace(target, binary); err != nil {
		return core.Internal("installing %s", release).Wrap(err)
	}

	_, _ = fmt.Fprintf(out, "updated tix %s -> %s\n", current, release)
	if runtime.GOOS == "windows" {
		// The running binary had to be moved aside; Windows releases it when
		// the process exits, and saying so beats leaving a stray file that
		// looks like a failed update.
		_, _ = fmt.Fprintf(out, "the previous binary is at %s.old and can be deleted\n", target)
	}
	g.diag(cmd, "run tix version to confirm")
	return nil
}

// refuseUnlessOurs stops this command overwriting a binary another tool owns.
//
// The refusals name the command that would do the upgrade, because "refused"
// on its own leaves someone with a tix they cannot update and no idea why.
func refuseUnlessOurs(target string) error {
	moduleVersion, revision := "", ""
	if info, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = info.Main.Version
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value
			}
		}
	}
	switch selfupdate.Detect(target, moduleVersion, revision) {
	case selfupdate.MethodGoInstall:
		return core.Invalid(
			"this tix was installed with the Go toolchain, which owns it; upgrade with: go install github.com/%s@latest",
			selfupdate.Repo)
	case selfupdate.MethodManaged:
		return core.Invalid(
			"this tix at %s is managed by a package manager; upgrade it the way you installed it", target)
	}
	return nil
}

func init() { builders = append(builders, newUpdateCmd) }
