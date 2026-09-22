package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
)

// newUserKeyCmd builds the user key command group.
func newUserKeyCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage SSH public keys",
		Long: "Enrol the public keys that may reach the terminal interface over SSH.\n\n" +
			"A key is a credential, so it belongs to one actor in one tenant. Enrol the same key in two " +
			"tenants and it speaks for a different actor in each.",
		Args: noArgs,
		RunE: helpRunner,
	}
	cmd.AddCommand(userKeyAddCmd(g), userKeyLsCmd(g), userKeyRmCmd(g))
	return cmd
}

// readPublicKey takes the key from a file, or from standard input when the
// path is "-", so a key can be piped in without ever reaching the process
// table the way an argument would.
func readPublicKey(cmd *cobra.Command, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", core.Invalid("a key file is required; pass --file ~/.ssh/id_ed25519.pub or --file - to read standard input")
	}
	if path == StdinMarker {
		return body(cmd, StdinMarker)
	}
	b, err := os.ReadFile(path) // #nosec G304 -- the operator names the key file to enrol
	if err != nil {
		return "", core.Invalid("reading %s: %v", path, err)
	}
	return string(b), nil
}

func userKeyAddCmd(g *globals) *cobra.Command {
	var file, actor, label string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Enrol an SSH public key",
		Long: "Enrol a public key against an actor.\n\n" +
			"Pass the public half, usually ~/.ssh/id_ed25519.pub, never the private key. " +
			"Options carried on an authorized_keys line are refused rather than dropped, because a " +
			"restriction that was silently discarded is worse than one that was rejected.\n\n" +
			"Exit codes: 2 invalid key, 3 unknown actor, 4 key already enrolled, 5 permission denied.",
		Example: "  tix user key add --file ~/.ssh/id_ed25519.pub --label laptop\n" +
			"  ssh-add -L | head -1 | tix user key add --file -",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			key, err := readPublicKey(cmd, file)
			if err != nil {
				return err
			}
			in := core.EnrolSSHKeyInput{ActorID: actor, PublicKey: key, Label: label}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("sshkey.enrol", label, map[string]any{"actor": actor}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			enrolled, err := conn.Service.EnrolSSHKey(ctx, in)
			if err != nil {
				return err
			}
			g.diag(cmd, "enrolled %s", enrolled.Fingerprint)
			return g.render(cmd, enrolled)
		},
	}
	f := cmd.Flags()
	f.StringVar(&file, "file", "", "public key file to enrol, or - for standard input")
	f.StringVar(&actor, "actor", "", "actor the key speaks for, by handle or identifier; defaults to you")
	f.StringVar(&label, "label", "", "note for telling one key from another")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be enrolled without writing")
	return cmd
}

func userKeyLsCmd(g *globals) *cobra.Command {
	var actor string
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List enrolled SSH keys",
		Long: "List enrolled keys, optionally for one actor.\n\n" +
			"Revoked keys are listed too, marked as revoked: a key that stopped working is usually the " +
			"one being looked for.\n\nExit codes: 3 unknown actor, 5 permission denied.",
		Example: "  tix user key ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			keys, err := conn.Service.ListSSHKeys(ctx, actor)
			if err != nil {
				return err
			}
			return g.render(cmd, keys)
		},
	}
	cmd.Flags().StringVar(&actor, "actor", "", "actor whose keys to list, by handle or identifier; defaults to you")
	return cmd
}

func userKeyRmCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm ID",
		Aliases: []string{"revoke"},
		Short:   "Revoke an enrolled SSH key",
		Long: "Revoke a key so it stops authenticating.\n\n" +
			"Sessions the key already holds are not cut. They end on the listener's idle timeout; " +
			"restart the listener if you need them gone now.\n\n" +
			"Exit codes: 3 unknown key, 5 permission denied.",
		Example: "  tix user key rm 01JB2K3M4N5P6Q7R8S9T",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RevokeSSHKey(ctx, args[0]); err != nil {
				return err
			}
			g.diag(cmd, "sessions this key already holds end on the listener's idle timeout")
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	return cmd
}
