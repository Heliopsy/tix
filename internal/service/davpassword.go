// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/store"
)

// davSettings are the deployment's caldav.* keys.
type davSettings struct {
	enabled    bool
	window     time.Duration
	maxObjects int
}

// defaultDAVSettings match the configuration's defaults, for a service built
// without WithCalDAV.
func defaultDAVSettings() davSettings {
	return davSettings{enabled: true, window: 14 * 24 * time.Hour, maxObjects: core.MaxCalendarEntries}
}

// WithCalDAV sets caldav.enabled, caldav.completed_window and
// caldav.max_objects. A bound below one keeps the default.
func WithCalDAV(enabled bool, window time.Duration, maxObjects int) Option {
	return func(l *Local) {
		l.dav.enabled = enabled
		if window >= 0 {
			l.dav.window = window
		}
		if maxObjects > 0 {
			l.dav.maxObjects = maxObjects
		}
	}
}

// davDisabledReason is what GetDAVInfo says while caldav.enabled is false.
const davDisabledReason = "CalDAV is turned off on this server (caldav.enabled)"

// GetDAVInfo reports whether CalDAV is served and what a client is set up with.
func (l *Local) GetDAVInfo(ctx context.Context) (*core.DAVInfo, error) {
	actor, err := l.authorize(ctx, authz.ActionDAVPassword, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := &core.DAVInfo{Enabled: l.dav.enabled, Path: core.DAVRoot,
		Collections: append([]core.DAVCollections(nil), core.AllDAVCollections...)}
	if !l.dav.enabled {
		out.Reason = davDisabledReason
	}
	err = l.read(ctx, actor, func(tx store.Tx) error {
		owner, err := davPerson(ctx, tx, actor.ID)
		if err != nil {
			return err
		}
		out.Username, err = davUsername(ctx, tx, owner)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListDAVPasswords lists the caller's app passwords, revoked ones included.
func (l *Local) ListDAVPasswords(ctx context.Context) ([]core.DAVPassword, error) {
	actor, err := l.authorize(ctx, authz.ActionDAVPassword, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.DAVPassword
	err = l.read(ctx, actor, func(tx store.Tx) error {
		if _, err := davPerson(ctx, tx, actor.ID); err != nil {
			return err
		}
		out, err = davPasswordsOf(ctx, tx, actor.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// davPasswordsOf pairs an actor's app password settings with their tokens,
// which hold revocation, expiry and last use.
func davPasswordsOf(ctx context.Context, tx store.Tx, actorID string) ([]core.DAVPassword, error) {
	settings, err := tx.ListDAVPasswords(ctx, actorID)
	if err != nil {
		return nil, err
	}
	tokens, err := tx.ListTokens(ctx, actorID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]core.APIToken, len(tokens))
	for _, t := range tokens {
		byID[t.ID] = t
	}
	out := make([]core.DAVPassword, 0, len(settings))
	for _, p := range settings {
		if tok, ok := byID[p.ID]; ok {
			out = append(out, withToken(p, tok))
		}
	}
	return out, nil
}

// withToken fills in what an app password takes from its token.
func withToken(p core.DAVPassword, tok core.APIToken) core.DAVPassword {
	p.TokenName, p.Scopes = tok.Name, tok.Scopes
	p.ExpiresAt, p.LastUsedAt, p.RevokedAt = tok.ExpiresAt, tok.LastUsedAt, tok.RevokedAt
	return p
}

// CreateDAVPassword mints an app password for the caller's own person actor.
func (l *Local) CreateDAVPassword(ctx context.Context, in core.CreateDAVPasswordInput) (*core.IssuedDAVPassword, error) {
	actor, err := l.authorize(ctx, authz.ActionDAVPassword, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if in, err = in.Normalize(); err != nil {
		return nil, err
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(l.clock.Now()) {
		return nil, core.Invalid("an app password cannot expire in the past").WithDetail(core.DetailField, "expires_at")
	}
	var out *core.IssuedDAVPassword
	err = l.write(ctx, actor, func(m *mutation) error {
		owner, err := davPerson(ctx, m.tx, actor.ID)
		if err != nil {
			return err
		}
		if role, err := userRole(ctx, m.tx, owner.ID); err != nil {
			return err
		} else if role == "" {
			return core.Invalid("an app password reads as a member of this tenant, and you hold no membership")
		}
		if err := checkDAVPasswordName(ctx, m.tx, owner.ID, in.Name, m.now); err != nil {
			return err
		}
		tokenName, err := davTokenName(ctx, m.tx, owner.Handle, in.Name, m.now)
		if err != nil {
			return err
		}
		minted, err := auth.MintAPIToken(l.clock, actor.TenantID, core.CreateTokenInput{
			Name: tokenName, ActorID: owner.ID, Scopes: []core.Scope{core.ScopeCaldav}, ExpiresAt: in.ExpiresAt})
		if err != nil {
			return err
		}
		tok := minted.Issued.APIToken
		if err := m.tx.CreateToken(ctx, &tok, minted.Hash); err != nil {
			return err
		}
		p := core.DAVPassword{ID: tok.ID, ActorID: owner.ID, Name: in.Name, Collections: in.Collections,
			ReadOnly: in.ReadOnly, CreatedAt: tok.CreatedAt}
		if err := m.tx.CreateDAVPassword(ctx, &p); err != nil {
			return err
		}
		username, err := davUsername(ctx, m.tx, owner)
		if err != nil {
			return err
		}
		out = &core.IssuedDAVPassword{DAVPassword: withToken(p, tok), Password: minted.Issued.Token,
			Username: username, Path: core.DAVRoot}
		return m.Record(auditTokenCreate, eventTokenCreated, "api_token", tok.ID, "", nil, tok,
			map[string]any{"name": tok.Name, "actor_id": tok.ActorID, "scopes": scopeStrings(tok.Scopes),
				"require_claim": false, "app_password": p.Name, "collections": string(p.Collections),
				"read_only": p.ReadOnly})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeDAVPassword revokes one of the caller's own app passwords. Another
// person's is not found; an administrator revokes it with tix token revoke.
func (l *Local) RevokeDAVPassword(ctx context.Context, id string) error {
	actor, err := l.authorize(ctx, authz.ActionDAVPassword, authz.Resource{})
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return core.Invalid("app password identifier is required").WithDetail(core.DetailField, "id")
	}
	return l.write(ctx, actor, func(m *mutation) error {
		p, err := m.tx.GetDAVPassword(ctx, id)
		if core.IsKind(err, core.KindNotFound) || err == nil && p.ActorID != actor.ID {
			return core.NotFound("app password %q not found", id)
		}
		if err != nil {
			return err
		}
		tok, err := m.tx.GetToken(ctx, id)
		if err != nil {
			return err
		}
		if tok.Revoked(m.now) {
			return nil
		}
		if err := m.tx.RevokeToken(ctx, id, m.now); err != nil {
			return err
		}
		return m.Record(auditTokenRevoke, eventTokenRevoked, "api_token", id, "", nil,
			map[string]any{"id": id, "revoked_at": m.now},
			map[string]any{"token_id": id, "token_actor_id": tok.ActorID, "name": tok.Name,
				"scopes": scopeStrings(tok.Scopes), "app_password": p.Name})
	})
}

// davPerson returns the caller's actor when it is a person. An agent signs in
// with its own token; an app password is for a person's calendar apps.
func davPerson(ctx context.Context, tx store.Tx, actorID string) (*core.Actor, error) {
	owner, err := tx.GetActor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if owner.Kind != core.ActorUser {
		return nil, core.Forbidden("an app password belongs to a person; an agent reads tasks with its own API token")
	}
	return owner, nil
}

// davUsername is the Basic username of a person: their sign-in email, or
// their handle when their account has none.
func davUsername(ctx context.Context, tx store.Tx, owner *core.Actor) (string, error) {
	user, err := accountOfActor(ctx, tx, owner.ID)
	switch {
	case err == nil && user.Email != "":
		return user.Email, nil
	case err != nil && !core.IsKind(err, core.KindNotFound):
		return "", err
	}
	return owner.Handle, nil
}

// checkDAVPasswordName refuses a name one of the owner's live app passwords
// already carries, in any case.
func checkDAVPasswordName(ctx context.Context, tx store.Tx, actorID, name string, now time.Time) error {
	existing, err := davPasswordsOf(ctx, tx, actorID)
	if err != nil {
		return err
	}
	for _, p := range existing {
		if p.Active(now) && strings.EqualFold(p.Name, name) {
			return core.Conflict("you already have an app password named %q; revoke it or choose another name", name).
				WithDetail(core.DetailField, "name")
		}
	}
	return nil
}

// maxDAVTokenSlug bounds the part of a token name taken from the person's own name.
const maxDAVTokenSlug = 40

// davTokenName is the name of the token behind an app password, dav-HANDLE-NAME,
// unique among the tenant's live tokens. Token names are unique per tenant and
// two people may both call theirs iPhone, so the person's name alone would
// collide; a short suffix settles what is left.
func davTokenName(ctx context.Context, tx store.Tx, handle, name string, now time.Time) (string, error) {
	base := "dav-" + davSlug(handle) + "-" + davSlug(name)
	candidate := base
	for i := 0; ; i++ {
		taken, err := tx.TokenNameInUse(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		if i == 8 {
			return "", core.Conflict("no free token name for this app password; choose another name").
				WithDetail(core.DetailField, "name")
		}
		suffix := strings.ToLower(id.NewAt(now.Add(time.Duration(i))))
		candidate = base + "-" + suffix[len(suffix)-6:]
	}
}

// davSlug lowercases a name to letters, digits and single dashes.
func davSlug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimSuffix(b.String(), "-")
	if r := []rune(out); len(r) > maxDAVTokenSlug {
		out = strings.TrimSuffix(string(r[:maxDAVTokenSlug]), "-")
	}
	if out == "" {
		out = "app"
	}
	return out
}
