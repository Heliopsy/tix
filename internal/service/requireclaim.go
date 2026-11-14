// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
)

// requireClaim refuses a change to a task by a caller whose API token was
// minted with require_claim, unless the token's actor holds a live claim on
// the task. A session, an SSH sign-in, a shell and any other token pass.
//
// The token is read here, inside the change's own transaction, rather than
// copied onto the actor at sign-in, so every authenticator and transport is
// held to it without carrying the flag. The read is skipped when the caller is
// no token or already holds the claim.
func (m *mutation) requireClaim(ctx context.Context, task *core.Task) error {
	if m.actor == nil || m.actor.TokenID == "" {
		return nil
	}
	if task.ClaimedAtTime(m.now) && task.ClaimedByActorID == m.actor.ID {
		return nil
	}
	tok, err := m.tx.GetToken(ctx, m.actor.TokenID)
	switch {
	case core.IsKind(err, core.KindNotFound):
		return nil
	case err != nil:
		return err
	case !tok.RequireClaim:
		return nil
	}
	return core.Forbidden("this token requires a claim on %s before changing it; run: tix claim task %s --start",
		task.Ref, task.Ref).WithDetail("claim_required", task.Ref)
}
