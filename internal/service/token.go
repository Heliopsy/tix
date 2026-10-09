// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Event types for API token lifecycle.
const (
	eventTokenCreated core.EventType = "token.created"
	eventTokenRevoked core.EventType = "token.revoked"
)

// Audit actions recorded for API tokens.
const (
	auditTokenCreate = "token.create"
	auditTokenRevoke = "token.revoke"
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
	if err := checkScopeGrant(actor, in.Scopes); err != nil {
		return nil, err
	}

	var out *core.IssuedToken
	err = l.write(ctx, actor, func(m *mutation) error {
		target, err := tokenActor(ctx, m.tx, actor, in.ActorID)
		if err != nil {
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
		if err := m.tx.RevokeToken(ctx, id, m.now); err != nil {
			return err
		}
		return m.Record(auditTokenRevoke, eventTokenRevoked, "api_token", id, "", nil,
			map[string]any{"id": id, "revoked_at": m.now}, map[string]any{"token_id": id})
	})
}
