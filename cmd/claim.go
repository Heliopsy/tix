// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// claimView is the table rendering of a claim. Machine formats emit the claim
// itself, whose field names are part of the stable contract.
type claimView struct {
	Ref            string `json:"ref" yaml:"ref"`
	Title          string `json:"title" yaml:"title"`
	Status         string `json:"status" yaml:"status"`
	LeaseToken     string `json:"lease_token" yaml:"lease_token"`
	LeaseExpiresAt string `json:"lease_expires_at" yaml:"lease_expires_at"`
}

// sweepResult reports how many leases a sweep expired.
type sweepResult struct {
	Swept int `json:"swept" yaml:"swept"`
}

// execResult reports what a wrapped command did under its lease.
type execResult struct {
	Ref      string `json:"ref" yaml:"ref"`
	ExitCode int    `json:"exit_code" yaml:"exit_code"`
	Status   string `json:"status,omitempty" yaml:"status,omitempty"`
}

// renderClaim writes a claim in the selected format.
func (g *globals) renderClaim(cmd *cobra.Command, claim *core.Claim) error {
	if g.formatName() != output.FormatTable {
		return g.render(cmd, claim)
	}
	view := claimView{
		LeaseToken:     claim.LeaseToken,
		LeaseExpiresAt: output.FormatTimestamp(claim.LeaseExpiresAt),
	}
	if claim.Task != nil {
		view.Ref, view.Title, view.Status = claim.Task.Ref, claim.Task.Title, claim.Task.Status
	}
	return g.render(cmd, view)
}

// renewDivisor sets the renewal interval to a fraction of the lease, so a slow
// renewal still lands well before the lease expires.
const renewDivisor = 3

// exitError carries a child process's exit status out to the process exit code.
type exitError struct{ code int }

func (e *exitError) Error() string { return "command exited non-zero" }

// newClaimCmd builds the claim command group.
func newClaimCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "claim",
		Short:   "Take and hold leases on tasks",
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(claimTaskCmd(g), claimNextCmd(g), claimRenewCmd(g),
		claimReleaseCmd(g), claimSweepCmd(g), claimExecCmd(g))
	return cmd
}

func claimTaskCmd(g *globals) *cobra.Command {
	var ttl, onBehalf string
	cmd := &cobra.Command{
		Use:     "task REF",
		Short:   "Claim one named task",
		Long:    "Take a lease on one task.\n\nExit codes: 3 unknown reference, 4 already claimed.",
		Example: "  tix claim task default-1 --ttl 15m",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			lease, err := parseDuration(ttl)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			claim, err := conn.Service.ClaimTask(ctx, ref, core.ClaimInput{TTL: lease, ActorID: onBehalf})
			if err != nil {
				return err
			}
			return g.renderClaim(cmd, claim)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().StringVar(&ttl, "ttl", "", "lease duration, defaulting to the workflow's")
	cmd.Flags().StringVar(&onBehalf, "actor", "", "claim on behalf of another actor, by handle or identifier")
	return cmd
}

func claimNextCmd(g *globals) *cobra.Command {
	var ttl, onBehalf string
	var projects, tags, statuses []string
	cmd := &cobra.Command{
		Use:     "next",
		Short:   "Claim the next eligible task",
		Long:    "Claim the highest-priority unblocked task matching the filter.\n\nExit codes: 3 no eligible task, 5 permission denied.",
		Example: "  tix claim next\n  tix claim next -p infra --tag ops --ttl 30m",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			in, err := claimNextInput(projects, tags, statuses, ttl, onBehalf)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			claim, err := conn.Service.ClaimNext(ctx, in)
			if err != nil {
				return err
			}
			return g.renderClaim(cmd, claim)
		},
	}
	bindClaimFilter(g, cmd, &projects, &tags, &statuses, &ttl, &onBehalf)
	return cmd
}

func claimRenewCmd(g *globals) *cobra.Command {
	var token, ttl string
	cmd := &cobra.Command{
		Use:     "renew REF",
		Short:   "Extend a lease",
		Long:    "Extend the lease held on a task.\n\nExit codes: 3 unknown reference, 4 lease expired or held elsewhere.",
		Example: "  tix claim renew default-1 --token $TIX_LEASE --ttl 15m",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			lease, err := parseDuration(ttl)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			claim, err := conn.Service.RenewLease(ctx, ref, token, lease)
			if err != nil {
				return err
			}
			return g.renderClaim(cmd, claim)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().StringVar(&token, "token", "", "lease token returned by the claim")
	cmd.Flags().StringVar(&ttl, "ttl", "", "new lease duration")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func claimReleaseCmd(g *globals) *cobra.Command {
	var token, status, comment string
	var fields []string
	cmd := &cobra.Command{
		Use:     "release REF",
		Short:   "Give up a lease",
		Long:    "Release the lease on a task, optionally transitioning it.\n\nExit codes: 3 unknown reference, 4 lease expired.",
		Example: "  tix claim release default-1 --token $TIX_LEASE --status done",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			result, err := parseFields(fields)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			in := core.ReleaseInput{Status: status, Result: result, Comment: comment}
			if err := conn.Service.ReleaseLease(ctx, ref, token, in); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: ref.String(), Status: statusOK})
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().StringVar(&token, "token", "", "lease token returned by the claim")
	cmd.Flags().StringVar(&status, "status", "", "status to move the task to on release")
	cmd.Flags().StringVar(&comment, "comment", "", "comment recorded with the release")
	cmd.Flags().StringSliceVar(&fields, "result", nil, "result value as key=value, repeatable")
	_ = cmd.MarkFlagRequired("token")
	registerCompletions(g, cmd, "status")
	return cmd
}

func claimSweepCmd(g *globals) *cobra.Command {
	var limit int
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "sweep",
		Short:   "Expire leases that have run out",
		Long:    "Expire leases past their deadline and revert their tasks where the workflow says to.\n\nExit codes: 5 permission denied.",
		Example: "  tix claim sweep --limit 100",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("claim.sweep", "", map[string]any{"limit": limit}))
			}
			swept, err := conn.Service.SweepLeases(ctx, limit)
			if err != nil {
				return err
			}
			return g.render(cmd, sweepResult{Swept: swept})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum leases to expire, zero for the default")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be swept without writing")
	return cmd
}

func claimExecCmd(g *globals) *cobra.Command {
	var ttl, onBehalf, ref, startStatus, okStatus, failStatus string
	var projects, tags, statuses []string
	cmd := &cobra.Command{
		Use:   "exec -- COMMAND [ARG...]",
		Short: "Claim a task, run a command under the lease, then release it",
		Long: "Claim a task, renew its lease while a command runs, and release it with the command's exit status " +
			"recorded as the result. The release status must be reachable from the task's current state, so pass " +
			"--on-start when the workflow needs an intermediate step.\n\n" +
			"Exit codes: 3 no eligible task, 4 lease lost, otherwise the command's own exit status.",
		Example: "  tix claim exec --on-start doing -- ./worker.sh\n  tix claim exec --ref default-1 --on-start doing -- make test",
		Args:    minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := claimNextInput(projects, tags, statuses, ttl, onBehalf)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			claim, err := acquire(ctx, conn.Service, ref, in)
			if err != nil {
				return err
			}
			target := core.TaskRef{ID: claim.Task.ID}
			g.diag(cmd, "claimed %s until %s", claim.Task.Ref, claim.LeaseExpiresAt.Format(time.RFC3339))
			if startStatus != "" {
				if _, err := conn.Service.TransitionTask(ctx, target, core.TransitionInput{
					To: startStatus, LeaseToken: claim.LeaseToken,
				}); err != nil {
					return err
				}
			}

			stop := renewInBackground(ctx, conn.Service, target, claim, in.TTL)
			code, runErr := runChild(cmd, args)
			stop()

			status := okStatus
			if code != 0 {
				status = failStatus
			}
			release := core.ReleaseInput{
				Status: status,
				Result: map[string]any{"exit_code": code, "command": args[0]},
			}
			if err := conn.Service.ReleaseLease(ctx, target, claim.LeaseToken, release); err != nil {
				return err
			}
			if err := g.render(cmd, execResult{Ref: claim.Task.Ref, ExitCode: code, Status: status}); err != nil {
				return err
			}
			if runErr != nil && code == 0 {
				return runErr
			}
			if code != 0 {
				return &exitError{code: code}
			}
			return nil
		},
	}
	bindClaimFilter(g, cmd, &projects, &tags, &statuses, &ttl, &onBehalf)
	cmd.Flags().StringVar(&ref, "ref", "", "claim this task instead of the next eligible one")
	cmd.Flags().StringVar(&startStatus, "on-start", "", "status to move the task to before running")
	cmd.Flags().StringVar(&okStatus, "on-success", "done", "status to release to when the command succeeds")
	cmd.Flags().StringVar(&failStatus, "on-failure", "", "status to release to when the command fails")
	return cmd
}

// bindClaimFilter attaches the flags shared by claim next and claim exec.
func bindClaimFilter(g *globals, cmd *cobra.Command, projects, tags, statuses *[]string, ttl, actor *string) {
	f := cmd.Flags()
	f.StringSliceVarP(projects, "project", "p", nil, "restrict to project keys")
	f.StringSliceVarP(tags, "tag", "l", nil, "restrict to tags")
	f.StringSliceVarP(statuses, "status", "s", nil, "restrict to statuses")
	f.StringVar(ttl, "ttl", "", "lease duration, defaulting to the workflow's")
	f.StringVar(actor, "actor", "", "claim on behalf of another actor, by handle or identifier")
	registerCompletions(g, cmd, "project", "tag", "status")
}

// claimNextInput builds the filter shared by claim next and claim exec.
func claimNextInput(projects, tags, statuses []string, ttl, actor string) (core.ClaimNextInput, error) {
	lease, err := parseDuration(ttl)
	if err != nil {
		return core.ClaimNextInput{}, err
	}
	return core.ClaimNextInput{
		ProjectRefs: projects, Tags: tags, Statuses: statuses,
		TTL: lease, ActorID: actor,
	}, nil
}

// acquire claims a named task, or the next eligible one when no ref is given.
func acquire(ctx context.Context, svc core.Service, ref string, in core.ClaimNextInput) (*core.Claim, error) {
	if ref == "" {
		return svc.ClaimNext(ctx, in)
	}
	parsed, err := parseRef(ref)
	if err != nil {
		return nil, err
	}
	return svc.ClaimTask(ctx, parsed, core.ClaimInput{TTL: in.TTL, ActorID: in.ActorID})
}

// renewInBackground keeps the lease alive until the returned function is called.
func renewInBackground(ctx context.Context, svc core.Service, ref core.TaskRef, claim *core.Claim, ttl core.Duration) func() {
	interval := renewInterval(claim, ttl)
	ticker := clock.New().NewTicker(interval)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C():
				if _, err := svc.RenewLease(ctx, ref, claim.LeaseToken, ttl); err != nil {
					return
				}
			}
		}
	}()
	return func() {
		close(done)
		ticker.Stop()
	}
}

// renewInterval picks how often to renew, from the lease actually granted.
func renewInterval(claim *core.Claim, ttl core.Duration) time.Duration {
	granted := ttl.D()
	if granted <= 0 && claim.Task != nil && claim.Task.ClaimedAt != nil {
		granted = claim.LeaseExpiresAt.Sub(*claim.Task.ClaimedAt)
	}
	if granted <= 0 {
		granted = time.Minute
	}
	interval := granted / renewDivisor
	if interval < time.Second {
		interval = time.Second
	}
	return interval
}

// runChild runs the wrapped command with the CLI's own streams.
func runChild(cmd *cobra.Command, args []string) (int, error) {
	child := exec.CommandContext(cmd.Context(), args[0], args[1:]...) // #nosec G204 -- the command is the user's own
	child.Stdin = cmd.InOrStdin()
	child.Stdout = cmd.ErrOrStderr()
	child.Stderr = cmd.ErrOrStderr()
	err := child.Run()
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 1, core.Internal("running %q", args[0]).Wrap(err)
}
