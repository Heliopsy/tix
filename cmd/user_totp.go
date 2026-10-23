// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import "github.com/spf13/cobra"

// newUserTOTPCmd builds the second factor command group.
func newUserTOTPCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "totp",
		Aliases: []string{"2fa"},
		Short:   "Manage the second factor",
		Long: "Enrol, inspect and remove the time-based second factor on your own account, " +
			"and clear somebody else's.\n\n" +
			"A code is never taken from a flag or from an environment variable: a live second factor " +
			"passed as an argument lands in shell history and in the process table. Unattended callers " +
			"use an API token, which is exempt from the second factor.\n\n" +
			"The enrolled secret is sealed with auth.totp_key (TIX_AUTH_TOTP_KEY). Without that key " +
			"enrolment is refused rather than stored in the clear, and losing the key means everybody " +
			"enrols again.",
		Args: noArgs,
		RunE: helpRunner,
	}
	cmd.AddCommand(userTOTPEnrolCmd(g), userTOTPStatusCmd(g), userTOTPRmCmd(g),
		userTOTPCodesCmd(g), userTOTPClearCmd(g))
	return cmd
}

// enrolment is what the enrolling commands render.
//
// Neither the secret nor the otpauth URI is in it. Both are drawn on the
// terminal while enrolling and are finished with by the time this is
// rendered, and the formats this type exists for are the ones a caller
// redirects into a file. The recovery codes are here because nothing can
// produce them a second time, so a run that captured its output has to have
// captured them.
type enrolment struct {
	Enrolled      bool     `json:"enrolled" yaml:"enrolled"`
	Issuer        string   `json:"issuer,omitempty" yaml:"issuer,omitempty"`
	Account       string   `json:"account,omitempty" yaml:"account,omitempty"`
	RecoveryCodes []string `json:"recovery_codes" yaml:"recovery_codes"`
}

func userTOTPEnrolCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "enrol",
		Aliases: []string{"enroll", "add"},
		Short:   "Enrol a second factor",
		Long: "Generate a secret, show it as an otpauth:// uri and, where the terminal can draw one, " +
			"as a scannable code, then confirm it with a code from the authenticator.\n\n" +
			"The ten recovery codes are printed once on success and cannot be retrieved again.\n\n" +
			"Exit codes: 4 already enrolled, 5 invalid code, 6 no sealing key or no terminal.",
		Example: "  tix user totp enrol",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !onATerminal(cmd) {
				return errNoTerminal("enrolling a second factor")
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			started, err := conn.Service.BeginTOTPEnrolment(ctx)
			if err != nil {
				return err
			}
			g.showEnrolment(cmd, started)
			code, err := g.ask(cmd, "code from the authenticator: ")
			if err != nil {
				return err
			}
			codes, err := conn.Service.ConfirmTOTPEnrolment(ctx, code)
			if err != nil {
				return err
			}
			g.diag(cmd, "the recovery codes are shown once and cannot be retrieved again")
			return g.render(cmd, enrolment{
				Enrolled: true, Issuer: started.Issuer,
				Account: started.Account, RecoveryCodes: codes.Codes,
			})
		},
	}
}

func userTOTPStatusCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report your second factor",
		Long: "Report whether a second factor is enrolled, how many recovery codes are left, " +
			"and which surfaces this tenant requires one on. It never reports the secret.\n\n" +
			"Exit codes: 2 the caller is not a person, 5 permission denied.",
		Example: "  tix user totp status -o json",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			status, err := conn.Service.TOTPStatus(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, status)
		},
	}
}

func userTOTPRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm",
		Aliases: []string{"remove", "delete"},
		Short:   "Remove your second factor",
		Long: "Drop your own enrolment and every recovery code beside it.\n\n" +
			"Sign-in on a surface this tenant requires a factor on will take you through enrolment " +
			"again rather than refuse you.\n\n" +
			"Exit codes: 3 nothing is enrolled, 5 permission denied.",
		Example: "  tix user totp rm",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRun {
				return g.render(cmd, newPlan("totp.remove", "self", nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RemoveTOTP(ctx); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: "totp", Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be removed without writing")
	return cmd
}

func userTOTPCodesCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "codes",
		Aliases: []string{"recovery-codes"},
		Short:   "Replace your recovery codes",
		Long: "Mint ten fresh recovery codes and invalidate the previous set.\n\n" +
			"They are shown once and stored hashed, so nothing can produce them again.\n\n" +
			"Exit codes: 5 permission denied, 6 nothing is enrolled or the enrolment is unconfirmed.",
		Example: "  tix user totp codes",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			codes, err := conn.Service.RegenerateRecoveryCodes(ctx)
			if err != nil {
				return err
			}
			g.diag(cmd, "the recovery codes are shown once and cannot be retrieved again")
			return g.render(cmd, enrolment{Enrolled: true, RecoveryCodes: codes.Codes})
		},
	}
}

func userTOTPClearCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "clear ID",
		Short: "Clear another user's second factor",
		Long: "Return another user to an unenrolled state, which is the way back in for somebody " +
			"who has lost both their device and their recovery codes.\n\n" +
			"It needs the administrative scope for second factors; a member is refused.\n\n" +
			"Exit codes: 3 unknown user or nothing enrolled, 5 permission denied.",
		Example: "  tix user totp clear 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("totp.clear", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.ClearUserTOTP(ctx, args[0]); err != nil {
				return err
			}
			g.diag(cmd, "the user enrols again on their next sign-in to a surface that requires one")
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be cleared without writing")
	return cmd
}
