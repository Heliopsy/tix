// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// eventSCIMUpdated records a change to a tenant's SCIM settings.
const eventSCIMUpdated core.EventType = "scim.updated"

// Subject and audit actions for administering SCIM.
const (
	subjectSCIM = "scim"

	auditSCIMEnable  = "scim.enable"
	auditSCIMDisable = "scim.disable"
	auditSCIMToken   = "scim.token"
	auditSCIMMap     = "scim.map"
	auditSCIMUnmap   = "scim.unmap"
)

// scimSettings reads a tenant's SCIM settings inside a transaction.
func (l *Local) scimSettings(ctx context.Context, tx store.Tx) (*core.SCIMSettings, error) {
	c, err := tx.GetSCIMConfig(ctx)
	if err != nil {
		return nil, err
	}
	out := &core.SCIMSettings{Enabled: c.Enabled, Path: core.SCIMPath, AgentID: c.AgentID,
		LastActivityAt: c.LastActivityAt, Tokens: []core.APIToken{}}
	if c.AgentID != "" {
		found, err := tx.ListTokens(ctx, c.AgentID)
		if err != nil {
			return nil, err
		}
		at := l.clock.Now()
		for _, t := range found {
			if t.Active(at) {
				out.Tokens = append(out.Tokens, t)
			}
		}
	}
	if out.Users, err = tx.CountSCIMUsers(ctx); err != nil {
		return nil, err
	}
	groups, err := tx.ListSCIMGroups(ctx)
	if err != nil {
		return nil, err
	}
	out.Groups = len(groups)
	if out.GroupRoles, err = tx.SCIMGroupRoles(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// linkSCIM sets the absolute endpoint when a public address is configured.
func (l *Local) linkSCIM(ctx context.Context, s *core.SCIMSettings) *core.SCIMSettings {
	if s != nil && l.publicURL != "" {
		s.URL = strings.TrimRight(l.webBase(ctx), "/") + s.Path
	}
	return s
}

// GetSCIM reports the tenant's SCIM provisioning settings.
func (l *Local) GetSCIM(ctx context.Context) (*core.SCIMSettings, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.SCIMSettings
	err = l.read(ctx, actor, func(tx store.Tx) error {
		out, err = l.scimSettings(ctx, tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return l.linkSCIM(ctx, out), nil
}

// scimAgent returns the tenant's provisioning agent, creating it when the
// settings name none.
func scimAgent(ctx context.Context, m *mutation, c *store.SCIMConfig) (*core.Actor, error) {
	if c.AgentID != "" {
		a, err := m.tx.GetActor(ctx, c.AgentID)
		if err == nil {
			return a, nil
		}
		if !core.IsKind(err, core.KindNotFound) {
			return nil, err
		}
	}
	a, err := m.tx.GetActorByHandle(ctx, core.SCIMAgentHandle)
	switch {
	case err == nil && a.Kind == core.ActorAgent:
	case err == nil:
		return nil, core.Conflict("the handle %q belongs to a person; rename them before enabling SCIM",
			core.SCIMAgentHandle)
	case core.IsKind(err, core.KindNotFound):
		a = &core.Actor{Kind: core.ActorAgent, Handle: core.SCIMAgentHandle, DisplayName: "SCIM"}
		if err := m.tx.CreateActor(ctx, a); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	c.AgentID = a.ID
	return a, nil
}

// mintSCIMToken mints a provisioning token for the agent and records it.
func (l *Local) mintSCIMToken(ctx context.Context, m *mutation, agent *core.Actor) (string, error) {
	if err := checkTokenName(ctx, m.tx, core.SCIMTokenName); err != nil {
		return "", err
	}
	minted, err := auth.MintAPIToken(l.clock, m.actor.TenantID, core.CreateTokenInput{
		Name: core.SCIMTokenName, ActorID: agent.ID, Scopes: []core.Scope{core.ScopeSCIMProvision},
	})
	if err != nil {
		return "", err
	}
	tok := minted.Issued.APIToken
	if err := m.tx.CreateToken(ctx, &tok, minted.Hash); err != nil {
		return "", err
	}
	if err := m.Record(auditTokenCreate, eventTokenCreated, "api_token", tok.ID, "", nil, tok,
		map[string]any{"name": tok.Name, "actor_id": tok.ActorID, "scopes": scopeStrings(tok.Scopes)}); err != nil {
		return "", err
	}
	return minted.Issued.Token, nil
}

// liveTokens counts an actor's tokens that still work.
func liveTokens(ctx context.Context, m *mutation, actorID string) (int, error) {
	found, err := m.tx.ListTokens(ctx, actorID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range found {
		if t.Active(m.now) {
			n++
		}
	}
	return n, nil
}

// changeSCIM runs one administrative change to the tenant's SCIM settings.
// fn changes the settings and returns a token it minted, if any.
func (l *Local) changeSCIM(ctx context.Context, action string,
	fn func(m *mutation, c *store.SCIMConfig) (string, error),
) (*core.SCIMSettings, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.SCIMSettings
	err = l.write(ctx, actor, func(m *mutation) error {
		c, err := m.tx.GetSCIMConfig(ctx)
		if err != nil {
			return err
		}
		was := c.Enabled
		token, err := fn(m, &c)
		if err != nil {
			return err
		}
		c.UpdatedAt = m.now
		if err := m.tx.PutSCIMConfig(ctx, c); err != nil {
			return err
		}
		if out, err = l.scimSettings(ctx, m.tx); err != nil {
			return err
		}
		out.Token = token
		return m.Record(action, eventSCIMUpdated, subjectSCIM, actor.TenantID, "",
			map[string]any{"enabled": was}, map[string]any{"enabled": c.Enabled, "agent_id": c.AgentID},
			map[string]any{"enabled": c.Enabled, "token_minted": token != ""})
	})
	if err != nil {
		return nil, err
	}
	return l.linkSCIM(ctx, out), nil
}

// EnableSCIM turns SCIM provisioning on, creating the provisioning agent and
// minting its token when it holds no live one.
func (l *Local) EnableSCIM(ctx context.Context) (*core.SCIMSettings, error) {
	return l.changeSCIM(ctx, auditSCIMEnable, func(m *mutation, c *store.SCIMConfig) (string, error) {
		agent, err := scimAgent(ctx, m, c)
		if err != nil {
			return "", err
		}
		c.Enabled = true
		live, err := liveTokens(ctx, m, agent.ID)
		if err != nil || live > 0 {
			return "", err
		}
		return l.mintSCIMToken(ctx, m, agent)
	})
}

// DisableSCIM turns SCIM provisioning off and keeps the token.
func (l *Local) DisableSCIM(ctx context.Context) (*core.SCIMSettings, error) {
	return l.changeSCIM(ctx, auditSCIMDisable, func(_ *mutation, c *store.SCIMConfig) (string, error) {
		c.Enabled = false
		return "", nil
	})
}

// RotateSCIMToken revokes the provisioning agent's live tokens and mints one.
func (l *Local) RotateSCIMToken(ctx context.Context) (*core.SCIMSettings, error) {
	return l.changeSCIM(ctx, auditSCIMToken, func(m *mutation, c *store.SCIMConfig) (string, error) {
		agent, err := scimAgent(ctx, m, c)
		if err != nil {
			return "", err
		}
		if _, err := m.tx.RevokeActorTokens(ctx, agent.ID, m.now); err != nil {
			return "", err
		}
		return l.mintSCIMToken(ctx, m, agent)
	})
}

// groupMembersNamed lists the members of every group with a display name.
func groupMembersNamed(ctx context.Context, tx store.Tx, name string) ([]string, error) {
	groups, err := tx.SCIMGroupsByName(ctx, name)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, g := range groups {
		ids, err := tx.SCIMGroupMembers(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, ids...)
	}
	return out, nil
}

// MapSCIMGroup maps a SCIM group name to a role and settles its members.
func (l *Local) MapSCIMGroup(ctx context.Context, in core.SCIMGroupRoleInput) (*core.SCIMGroupRole, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	out := &core.SCIMGroupRole{Group: strings.TrimSpace(in.Group), Role: in.Role}
	err = l.write(ctx, actor, func(m *mutation) error {
		ids, err := groupMembersNamed(ctx, m.tx, out.Group)
		if err != nil {
			return err
		}
		mapped, err := mappedBefore(ctx, m.tx, ids)
		if err != nil {
			return err
		}
		existing, err := m.tx.SCIMGroupRoles(ctx)
		if err != nil {
			return err
		}
		out.CreatedAt = m.now
		for _, r := range existing {
			if core.SCIMKey(r.Group) == core.SCIMKey(out.Group) {
				out.CreatedAt = r.CreatedAt
			}
		}
		if err := m.tx.PutSCIMGroupRole(ctx, *out); err != nil {
			return err
		}
		if err := applyMappedRoles(ctx, m, ids, mapped); err != nil {
			return err
		}
		return m.Record(auditSCIMMap, eventSCIMUpdated, subjectSCIM, actor.TenantID, "", nil, out,
			map[string]any{"group": out.Group, "role": string(out.Role)})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UnmapSCIMGroup removes a group's role mapping and settles its members.
func (l *Local) UnmapSCIMGroup(ctx context.Context, group string) error {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	group = strings.TrimSpace(group)
	if group == "" {
		return core.Invalid("a group name is required").WithDetail(core.DetailField, "group")
	}
	return l.write(ctx, actor, func(m *mutation) error {
		ids, err := groupMembersNamed(ctx, m.tx, group)
		if err != nil {
			return err
		}
		mapped, err := mappedBefore(ctx, m.tx, ids)
		if err != nil {
			return err
		}
		found, err := m.tx.DeleteSCIMGroupRole(ctx, group)
		if err != nil {
			return err
		}
		if !found {
			return core.NotFound("no mapping for group %q", group)
		}
		if err := applyMappedRoles(ctx, m, ids, mapped); err != nil {
			return err
		}
		return m.Record(auditSCIMUnmap, eventSCIMUpdated, subjectSCIM, actor.TenantID, "",
			map[string]any{"group": group}, nil, map[string]any{"group": group})
	})
}
