// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Event types for API token lifecycle.
const (
	eventTokenCreated core.EventType = "token.created"
	eventTokenRevoked core.EventType = "token.revoked"
	eventTokenRotated core.EventType = "token.rotated"
)

// Audit actions recorded for API tokens.
//
// Revoking somebody else's credential is its own action rather than a field
// inside token.revoke. An operator reading the trail after a leak asks who
// killed the token and whether it was theirs to kill, and that has to be
// answerable from the action, because the trail is filtered by action and not
// by a payload field.
// #nosec G101 -- audit action names, not credentials.
const (
	auditTokenCreate      = "token.create"
	auditTokenRevoke      = "token.revoke"
	auditTokenRevokeOther = "token.revoke_other"
	auditTokenRotate      = "token.rotate"
	auditTokenRotateOther = "token.rotate_other"
)

// checkScopeGrant refuses a token carrying a scope its creator does not hold,
// because minting one would be a privilege escalation with no audit trail.
func checkScopeGrant(actor *core.Actor, scopes []core.Scope) error {
	for _, s := range scopes {
		if actor.HasScope(s) {
			continue
		}
		return core.Forbidden("a token cannot be granted scopes its creator does not hold").
			WithDetail("scope", string(s))
	}
	return nil
}

// withoutScope returns scopes less one.
func withoutScope(scopes []core.Scope, drop core.Scope) []core.Scope {
	out := make([]core.Scope, 0, len(scopes))
	for _, s := range scopes {
		if s != drop {
			out = append(out, s)
		}
	}
	return out
}

// targetIsOwnPerson refuses an account:read token for anybody but the caller's
// own actor, and for an actor that is not a person. An administrator minting
// tokens for other actors would otherwise read their accounts. A shell holding
// the database already holds every account, so it may mint one for anybody.
func targetIsOwnPerson(ctx context.Context, tx store.Tx, caller, target *core.Actor, shell bool) error {
	refused := core.Forbidden("a token carries %s only for its owner's own account", core.ScopeAccountRead).
		WithDetail("scope", string(core.ScopeAccountRead))
	if target.ID != caller.ID && !shell {
		return refused
	}
	switch _, err := accountOfActor(ctx, tx, target.ID); {
	case err == nil:
		return nil
	case core.IsKind(err, core.KindNotFound):
		return refused
	default:
		return err
	}
}

// targetIsGlobalAdmin refuses a global:admin token for an actor whose user is
// not a global admin. The scope is checked again on every call, but a token
// that could never use it should not be minted to look as if it could.
func targetIsGlobalAdmin(ctx context.Context, tx store.Tx, target *core.Actor) error {
	u, err := accountOfActor(ctx, tx, target.ID)
	if err == nil && u.GlobalAdmin && u.DisabledAt == nil {
		return nil
	}
	if err != nil && !core.IsKind(err, core.KindNotFound) {
		return err
	}
	return core.Forbidden("a token carries global:admin only when the actor it acts as is a global admin").
		WithDetail("scope", string(core.ScopeGlobalAdmin))
}

// tokenActor resolves the actor a token acts as, defaulting to the caller.
func tokenActor(ctx context.Context, tx store.Tx, actor *core.Actor, ref string) (*core.Actor, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return tx.GetActor(ctx, actor.ID)
	}
	return lookupActor(ctx, tx, ref)
}

// CreateToken mints an API token whose value is returned exactly once.
// --project accepts either the project's key or its identifier, resolved the
// same way a task reference resolves a project key.
func (l *Local) CreateToken(ctx context.Context, in core.CreateTokenInput) (*core.IssuedToken, error) {
	// The coarse scoping check runs on the reference as given, before it is
	// resolved, so a project-scoped actor is refused for a mismatched project
	// without disclosing whether that project even exists.
	actor, err := l.authorize(ctx, authz.ActionTokenAdmin, authz.Resource{ProjectID: in.ProjectID})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if err := l.checkMintScopes(ctx, actor, in.Scopes); err != nil {
		return nil, err
	}

	var out *core.IssuedToken
	err = l.write(ctx, actor, func(m *mutation) error {
		target, err := tokenActor(ctx, m.tx, actor, in.ActorID)
		if err != nil {
			return err
		}
		if err := checkMintTarget(ctx, m.tx, actor, target, in.Scopes); err != nil {
			return err
		}
		if strings.TrimSpace(in.ProjectID) != "" {
			project, err := lookupProject(ctx, m.tx, in.ProjectID)
			if err != nil {
				return err
			}
			in.ProjectID = project.ID
		}
		in.ActorID = target.ID
		if err := checkTokenName(ctx, m.tx, in.Name); err != nil {
			return err
		}

		minted, err := auth.MintAPIToken(l.clock, actor.TenantID, in)
		if err != nil {
			return err
		}
		tok := minted.Issued.APIToken
		if err := m.tx.CreateToken(ctx, &tok, minted.Hash); err != nil {
			return err
		}
		minted.Issued.APIToken = tok
		out = minted.Issued
		return m.Record(auditTokenCreate, eventTokenCreated, "api_token", tok.ID, tok.ProjectID, nil, tok,
			map[string]any{"name": tok.Name, "actor_id": tok.ActorID, "scopes": scopeStrings(tok.Scopes)})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// checkMintScopes is the ceiling on what a caller may mint, checked before the
// transaction: every scope must be one the caller holds, global:admin needs a
// global admin, and account:read a caller that is not a token lacking it.
func (l *Local) checkMintScopes(ctx context.Context, actor *core.Actor, scopes []core.Scope) error {
	if slices.Contains(scopes, core.ScopeGlobalAdmin) {
		if _, err := l.authorizeGlobal(ctx); err != nil {
			return err
		}
	}
	if slices.Contains(scopes, core.ScopeAccountRead) && actor.TokenID != "" && !actor.HasScope(core.ScopeAccountRead) {
		return core.Forbidden("a token without %s cannot mint one with it", core.ScopeAccountRead).
			WithDetail("scope", string(core.ScopeAccountRead))
	}
	return checkScopeGrant(actor, withoutScope(withoutScope(scopes, core.ScopeGlobalAdmin), core.ScopeAccountRead))
}

// checkMintTarget is the half of the ceiling that depends on whom the token
// acts as, checked inside the transaction that mints it.
func checkMintTarget(ctx context.Context, tx store.Tx, actor, target *core.Actor, scopes []core.Scope) error {
	if slices.Contains(scopes, core.ScopeGlobalAdmin) {
		if err := targetIsGlobalAdmin(ctx, tx, target); err != nil {
			return err
		}
	}
	if slices.Contains(scopes, core.ScopeAccountRead) {
		return targetIsOwnPerson(ctx, tx, actor, target, core.DirectStoreFrom(ctx))
	}
	return nil
}

// checkTokenName refuses a name a live token of this tenant already carries.
//
// The rule lives here rather than in the browser so the command line and the
// HTTP API are held to it too. The check is inside the same transaction as the
// insert, and a unique index backs it, because two creations racing under read
// committed would both see the name free.
func checkTokenName(ctx context.Context, tx store.Tx, name string) error {
	taken, err := tx.TokenNameInUse(ctx, name)
	if err != nil {
		return err
	}
	if !taken {
		return nil
	}
	return core.Conflict("a token named %q already exists; revoke it or choose another name", name).
		WithDetail(core.DetailField, "name")
}

// scopeStrings renders scopes for an event payload.
func scopeStrings(scopes []core.Scope) []string {
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		out = append(out, string(s))
	}
	return out
}

// ListTokens returns an actor's tokens as metadata only. An empty actor means
// the caller's own.
func (l *Local) ListTokens(ctx context.Context, actorID string) ([]core.APIToken, error) {
	actor, err := l.authorize(ctx, authz.ActionTokenAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := []core.APIToken{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		target, err := tokenActor(ctx, tx, actor, actorID)
		if err != nil {
			return err
		}
		found, err := tx.ListTokens(ctx, target.ID)
		if err != nil {
			return err
		}
		out = slices.Clone(found)
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeToken revokes a token of this tenant. A token that is already revoked
// stays revoked, and an identifier of another tenant is reported missing.
func (l *Local) RevokeToken(ctx context.Context, id string) error {
	actor, err := l.authorize(ctx, authz.ActionTokenAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return core.Invalid("token identifier is required")
	}
	return l.write(ctx, actor, func(m *mutation) error {
		existing, err := m.tx.GetToken(ctx, id)
		if err != nil {
			return err
		}
		if err := m.tx.RevokeToken(ctx, id, m.now); err != nil {
			return err
		}
		action := auditTokenRevoke
		if existing.ActorID != actor.ID {
			action = auditTokenRevokeOther
		}
		return m.Record(action, eventTokenRevoked, "api_token", id, "", nil,
			map[string]any{"id": id, "revoked_at": m.now},
			map[string]any{"token_id": id, "token_actor_id": existing.ActorID, "name": existing.Name})
	})
}

// RotateToken replaces a live token with a new one of the same name, scopes
// and project, retiring the old one at once or after a grace window.
func (l *Local) RotateToken(ctx context.Context, ref string, in core.RotateTokenInput) (*core.IssuedToken, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := l.requireFactor(ctx, actor); err != nil {
		return nil, err
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, core.Invalid("token identifier or name is required")
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	var old *core.APIToken
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		old, err = rotationSubject(ctx, tx, ref, in.ActorID)
		return err
	}); err != nil {
		return nil, err
	}
	if err := l.policy.CanRotateToken(actor, old.ActorID, old.ProjectID); err != nil {
		return nil, err
	}
	if err := l.checkMintScopes(ctx, actor, old.Scopes); err != nil {
		return nil, err
	}

	var out *core.IssuedToken
	err = l.write(ctx, actor, func(m *mutation) error {
		cur, err := m.tx.GetToken(ctx, old.ID)
		if err != nil {
			return err
		}
		if err := rotatable(cur, m.now); err != nil {
			return err
		}
		target, err := m.tx.GetActor(ctx, cur.ActorID)
		if err != nil {
			return err
		}
		if err := checkMintTarget(ctx, m.tx, actor, target, cur.Scopes); err != nil {
			return err
		}
		retireAt := m.now.Add(in.Grace.D())
		if cur.ExpiresAt != nil && cur.ExpiresAt.Before(retireAt) {
			retireAt = *cur.ExpiresAt
		}
		if err := m.tx.RevokeToken(ctx, cur.ID, retireAt); err != nil {
			return err
		}
		if err := checkTokenName(ctx, m.tx, cur.Name); err != nil {
			return err
		}
		minted, err := auth.MintAPIToken(l.clock, actor.TenantID, core.CreateTokenInput{
			Name: cur.Name, ActorID: cur.ActorID, Scopes: cur.Scopes, ProjectID: cur.ProjectID,
			ExpiresAt: rotatedExpiry(cur, m.now),
		})
		if err != nil {
			return err
		}
		tok := minted.Issued.APIToken
		if err := m.tx.CreateToken(ctx, &tok, minted.Hash); err != nil {
			return err
		}
		minted.Issued.APIToken = tok
		out = minted.Issued
		action := auditTokenRotate
		if cur.ActorID != actor.ID {
			action = auditTokenRotateOther
		}
		return m.Record(action, eventTokenRotated, "api_token", tok.ID, tok.ProjectID,
			map[string]any{"id": cur.ID, "revoked_at": retireAt}, tok,
			map[string]any{"token_id": tok.ID, "old_token_id": cur.ID, "token_actor_id": cur.ActorID,
				"name": tok.Name, "scopes": scopeStrings(tok.Scopes), "old_revoked_at": retireAt})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// rotationSubject resolves the token a rotation names, by identifier and then
// by live name. A named actor that does not hold it hides it, so a script
// pointed at the wrong agent rotates nothing.
func rotationSubject(ctx context.Context, tx store.Tx, ref, actorRef string) (*core.APIToken, error) {
	tok, err := tx.GetToken(ctx, ref)
	if core.IsKind(err, core.KindNotFound) {
		tok, err = tx.GetLiveTokenByName(ctx, ref)
	}
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("api token %q", ref)
		}
		return nil, err
	}
	if strings.TrimSpace(actorRef) == "" {
		return tok, nil
	}
	holder, err := lookupActor(ctx, tx, actorRef)
	if err != nil {
		return nil, err
	}
	if holder.ID != tok.ActorID {
		return nil, core.NotFound("api token %q of actor %q", ref, actorRef)
	}
	return tok, nil
}

// rotatable refuses a token with nothing live to replace. A retiring token is
// refused too: its name already belongs to its replacement.
func rotatable(tok *core.APIToken, now time.Time) error {
	switch {
	case tok.Retiring(now):
		return core.Conflict("token %q was already rotated and stops at %s; rotate its replacement",
			tok.Name, tok.RevokedAt.UTC().Format(time.RFC3339))
	case !tok.Active(now):
		return core.Conflict("token %q no longer works, so there is nothing to rotate; issue a new one", tok.Name)
	}
	return nil
}

// rotatedExpiry gives the new token the old one's lifetime counted from now,
// and no expiry when the old one had none.
func rotatedExpiry(old *core.APIToken, now time.Time) *time.Time {
	if old.ExpiresAt == nil {
		return nil
	}
	at := now.Add(old.ExpiresAt.Sub(old.CreatedAt))
	return &at
}
