// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// noteDenied tells the request log why the policy refused actor, when it
// did. The client's answer is the policy's error, unchanged. A project-pinned
// token holding the scope the action needs was refused for its project; any
// other refusal is its scopes or its role.
func noteDenied(ctx context.Context, actor *core.Actor, err error) {
	var e *core.Error
	if actor == nil || !errors.As(err, &e) || e.Kind != core.KindForbidden {
		return
	}
	reason := auth.RefusedScopeDenied
	scope, _ := e.Details["scope"].(string)
	if actor.ScopedToProject() && (scope == "" || actor.HasScope(core.Scope(scope))) {
		reason = auth.RefusedProjectDenied
	}
	noteRefused(ctx, actor, reason)
}

// noteRefused tells the request log that actor was refused for reason.
func noteRefused(ctx context.Context, actor *core.Actor, reason string) {
	auth.NoteRefusal(ctx, auth.Refusal{Reason: reason, TokenID: actor.TokenID, TokenName: actor.TokenName,
		Session: actor.SessionID})
}

// noteLoginRefused tells the request log a password was refused: for an
// account usable here whose password did not match, or for no such account.
// The caller is told neither apart.
func noteLoginRefused(ctx context.Context, usable bool) {
	reason := auth.RefusedUnknownAccount
	if usable {
		reason = auth.RefusedBadPassword
	}
	auth.NoteRefusal(ctx, auth.Refusal{Reason: reason})
}

// noteCodeRefused tells the request log a second-factor code was refused,
// and whether it was because verification is locked.
func noteCodeRefused(ctx context.Context, err error) {
	reason := auth.RefusedTOTPInvalid
	if core.IsTOTPLocked(err) {
		reason = auth.RefusedTOTPLocked
	}
	auth.NoteRefusal(ctx, auth.Refusal{Reason: reason})
}

// sshSignInRefusal names why enrolledPerson refuses a key, in the order it
// tests, or "" when it does not.
func sshSignInRefusal(key *core.SSHKey, cand candidate) string {
	switch {
	case key == nil:
		return auth.RefusedSSHUnknownKey
	case !key.Active():
		return auth.RefusedSSHKeyRevoked
	case cand.actor != nil && cand.user == nil:
		return auth.RefusedSSHAgentKey
	case cand.user == nil || cand.user.DisabledAt != nil || !cand.member():
		return auth.RefusedUnknownAccount
	}
	return ""
}
