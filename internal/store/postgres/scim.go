// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"slices"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// GetSCIMConfig returns the tenant's SCIM switch, zero when it was never set.
func (t *tx) GetSCIMConfig(ctx context.Context) (store.SCIMConfig, error) {
	b := t.builder("scim_config").Select("scim_config.enabled", "scim_config.agent_id",
		"scim_config.last_activity_at", "scim_config.updated_at")
	rows, err := t.query(ctx, b, "reading scim settings")
	if err != nil {
		return store.SCIMConfig{}, err
	}
	defer func() { _ = rows.Close() }()
	var out store.SCIMConfig
	for rows.Next() {
		var enabled int64
		var last, updated scimStamp
		if err := rows.Scan(&enabled, &out.AgentID, &last, &updated); err != nil {
			return store.SCIMConfig{}, mapErr(err, "scanning scim settings")
		}
		out.Enabled = sqlb.Bool(enabled)
		if out.LastActivityAt, err = scimNullWhen(last); err != nil {
			return store.SCIMConfig{}, err
		}
		if out.UpdatedAt, err = scimWhen(updated); err != nil {
			return store.SCIMConfig{}, err
		}
	}
	return out, mapRowsErr(rows, "reading scim settings")
}

// PutSCIMConfig writes the tenant's SCIM switch, creating the row when absent.
func (t *tx) PutSCIMConfig(ctx context.Context, c store.SCIMConfig) error {
	up := t.builder("scim_config").
		Set("enabled", sqlb.BoolInt(c.Enabled)).
		Set("agent_id", c.AgentID).
		Set("last_activity_at", scimNullAt(c.LastActivityAt)).
		Set("updated_at", scimAt(c.UpdatedAt))
	n, err := t.execUpdate(ctx, up, "writing scim settings")
	if err != nil || n > 0 {
		return err
	}
	ins := t.insert("scim_config").
		Set("enabled", sqlb.BoolInt(c.Enabled)).
		Set("agent_id", c.AgentID).
		Set("last_activity_at", scimNullAt(c.LastActivityAt)).
		Set("updated_at", scimAt(c.UpdatedAt))
	_, err = t.execInsert(ctx, ins, "writing scim settings")
	return err
}

// TouchSCIM records a provider's change.
func (t *tx) TouchSCIM(ctx context.Context, at time.Time) error {
	c, err := t.GetSCIMConfig(ctx)
	if err != nil {
		return err
	}
	c.LastActivityAt = &at
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = at
	}
	return t.PutSCIMConfig(ctx, c)
}

var scimUserCols = []string{
	"scim_users.actor_id", "scim_users.user_name", "scim_users.external_id", "scim_users.email",
	"scim_users.given_name", "scim_users.family_name", "scim_users.created_at", "scim_users.updated_at",
}

func (t *tx) scimUsers(ctx context.Context, b *sqlb.Builder) ([]store.SCIMUserRow, error) {
	b.Select(scimUserCols...).OrderBy("scim_users.actor_id", core.Ascending)
	rows, err := t.query(ctx, b, "reading scim users")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []store.SCIMUserRow
	for rows.Next() {
		var r store.SCIMUserRow
		var created, updated scimStamp
		if err := rows.Scan(&r.ActorID, &r.UserName, &r.ExternalID, &r.Email, &r.GivenName, &r.FamilyName,
			&created, &updated); err != nil {
			return nil, mapErr(err, "scanning scim user")
		}
		if r.CreatedAt, err = scimWhen(created); err != nil {
			return nil, err
		}
		if r.UpdatedAt, err = scimWhen(updated); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, mapRowsErr(rows, "reading scim users")
}

// GetSCIMUser returns a member's SCIM row.
func (t *tx) GetSCIMUser(ctx context.Context, actorID string) (*store.SCIMUserRow, error) {
	found, err := t.scimUsers(ctx, t.builder("scim_users").Where("scim_users.actor_id = ?", actorID))
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, core.NotFound("scim user %q", actorID)
	}
	return &found[0], nil
}

// SCIMUsersByName finds rows by userName, ignoring case.
func (t *tx) SCIMUsersByName(ctx context.Context, userName string) ([]store.SCIMUserRow, error) {
	return t.scimUsers(ctx, t.builder("scim_users").Where("scim_users.user_name_key = ?", core.SCIMKey(userName)))
}

// SCIMUsersByExternalID finds rows by externalId.
func (t *tx) SCIMUsersByExternalID(ctx context.Context, externalID string) ([]store.SCIMUserRow, error) {
	return t.scimUsers(ctx, t.builder("scim_users").Where("scim_users.external_id = ?", externalID))
}

// SCIMUsersByEmail finds rows by address, ignoring case.
func (t *tx) SCIMUsersByEmail(ctx context.Context, email string) ([]store.SCIMUserRow, error) {
	return t.scimUsers(ctx, t.builder("scim_users").Where("scim_users.email = ?", core.SCIMKey(email)))
}

// PutSCIMUser writes a member's row, creating it when absent.
func (t *tx) PutSCIMUser(ctx context.Context, r store.SCIMUserRow) error {
	up := t.builder("scim_users").Where("scim_users.actor_id = ?", r.ActorID).
		Set("user_name", r.UserName).
		Set("user_name_key", core.SCIMKey(r.UserName)).
		Set("external_id", r.ExternalID).
		Set("email", core.SCIMKey(r.Email)).
		Set("given_name", r.GivenName).
		Set("family_name", r.FamilyName).
		Set("updated_at", scimAt(r.UpdatedAt))
	n, err := t.execUpdate(ctx, up, "writing scim user %q", r.UserName)
	if err != nil || n > 0 {
		return err
	}
	ins := t.insert("scim_users").
		Set("actor_id", r.ActorID).
		Set("user_name", r.UserName).
		Set("user_name_key", core.SCIMKey(r.UserName)).
		Set("external_id", r.ExternalID).
		Set("email", core.SCIMKey(r.Email)).
		Set("given_name", r.GivenName).
		Set("family_name", r.FamilyName).
		Set("created_at", scimAt(r.CreatedAt)).
		Set("updated_at", scimAt(r.UpdatedAt))
	_, err = t.execInsert(ctx, ins, "writing scim user %q", r.UserName)
	return err
}

// DeleteSCIMUser drops a member's row.
func (t *tx) DeleteSCIMUser(ctx context.Context, actorID string) error {
	_, err := t.execDelete(ctx, t.builder("scim_users").Where("scim_users.actor_id = ?", actorID),
		"deleting scim user %q", actorID)
	return err
}

// CountSCIMUsers counts the members a provider pushed.
func (t *tx) CountSCIMUsers(ctx context.Context) (int, error) {
	return t.count(ctx, t.builder("scim_users"), "counting scim users")
}

var scimGroupCols = []string{
	"scim_groups.id", "scim_groups.display_name", "scim_groups.external_id",
	"scim_groups.created_at", "scim_groups.updated_at",
}

func (t *tx) scimGroups(ctx context.Context, b *sqlb.Builder) ([]store.SCIMGroupRow, error) {
	b.Select(scimGroupCols...).OrderBy("scim_groups.id", core.Ascending)
	rows, err := t.query(ctx, b, "reading scim groups")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []store.SCIMGroupRow
	for rows.Next() {
		var g store.SCIMGroupRow
		var created, updated scimStamp
		if err := rows.Scan(&g.ID, &g.DisplayName, &g.ExternalID, &created, &updated); err != nil {
			return nil, mapErr(err, "scanning scim group")
		}
		if g.CreatedAt, err = scimWhen(created); err != nil {
			return nil, err
		}
		if g.UpdatedAt, err = scimWhen(updated); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, mapRowsErr(rows, "reading scim groups")
}

// ListSCIMGroups returns every group, ordered by identifier.
func (t *tx) ListSCIMGroups(ctx context.Context) ([]store.SCIMGroupRow, error) {
	return t.scimGroups(ctx, t.builder("scim_groups"))
}

// GetSCIMGroup returns one group.
func (t *tx) GetSCIMGroup(ctx context.Context, id string) (*store.SCIMGroupRow, error) {
	found, err := t.scimGroups(ctx, t.builder("scim_groups").Where("scim_groups.id = ?", id))
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, core.NotFound("group %q", id)
	}
	return &found[0], nil
}

// SCIMGroupsByName finds groups by display name, ignoring case.
func (t *tx) SCIMGroupsByName(ctx context.Context, name string) ([]store.SCIMGroupRow, error) {
	return t.scimGroups(ctx, t.builder("scim_groups").Where("scim_groups.name_key = ?", core.SCIMKey(name)))
}

// SCIMGroupsByExternalID finds groups by externalId.
func (t *tx) SCIMGroupsByExternalID(ctx context.Context, externalID string) ([]store.SCIMGroupRow, error) {
	return t.scimGroups(ctx, t.builder("scim_groups").Where("scim_groups.external_id = ?", externalID))
}

// CreateSCIMGroup inserts a group.
func (t *tx) CreateSCIMGroup(ctx context.Context, g store.SCIMGroupRow) error {
	ins := t.insert("scim_groups").
		Set("id", g.ID).
		Set("display_name", g.DisplayName).
		Set("name_key", core.SCIMKey(g.DisplayName)).
		Set("external_id", g.ExternalID).
		Set("created_at", scimAt(g.CreatedAt)).
		Set("updated_at", scimAt(g.UpdatedAt))
	_, err := t.execInsert(ctx, ins, "creating group %q", g.DisplayName)
	return err
}

// UpdateSCIMGroup writes a group's name and externalId.
func (t *tx) UpdateSCIMGroup(ctx context.Context, g store.SCIMGroupRow) error {
	up := t.builder("scim_groups").Where("scim_groups.id = ?", g.ID).
		Set("display_name", g.DisplayName).
		Set("name_key", core.SCIMKey(g.DisplayName)).
		Set("external_id", g.ExternalID).
		Set("updated_at", scimAt(g.UpdatedAt))
	n, err := t.execUpdate(ctx, up, "updating group %q", g.DisplayName)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("group %q", g.ID)
	}
	return nil
}

// DeleteSCIMGroup removes a group and its memberships.
func (t *tx) DeleteSCIMGroup(ctx context.Context, id string) error {
	if _, err := t.execDelete(ctx, t.builder("scim_group_members").Where("scim_group_members.group_id = ?", id),
		"emptying group %q", id); err != nil {
		return err
	}
	n, err := t.execDelete(ctx, t.builder("scim_groups").Where("scim_groups.id = ?", id), "deleting group %q", id)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("group %q", id)
	}
	return nil
}

// SCIMGroupMembers lists a group's members by actor.
func (t *tx) SCIMGroupMembers(ctx context.Context, groupID string) ([]string, error) {
	b := t.builder("scim_group_members").Select("scim_group_members.actor_id").
		Where("scim_group_members.group_id = ?", groupID).
		OrderBy("scim_group_members.actor_id", core.Ascending)
	rows, err := t.query(ctx, b, "reading members of group %q", groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, mapErr(err, "scanning group member")
		}
		out = append(out, id)
	}
	return out, mapRowsErr(rows, "reading group members")
}

// SetSCIMGroupMembers replaces a group's members.
func (t *tx) SetSCIMGroupMembers(ctx context.Context, groupID string, actorIDs []string) error {
	current, err := t.SCIMGroupMembers(ctx, groupID)
	if err != nil {
		return err
	}
	for _, id := range current {
		if slices.Contains(actorIDs, id) {
			continue
		}
		b := t.builder("scim_group_members").Where("scim_group_members.group_id = ?", groupID).
			Where("scim_group_members.actor_id = ?", id)
		if _, err := t.execDelete(ctx, b, "removing a member of group %q", groupID); err != nil {
			return err
		}
	}
	for _, id := range actorIDs {
		if slices.Contains(current, id) {
			continue
		}
		current = append(current, id)
		ins := t.insert("scim_group_members").Set("group_id", groupID).Set("actor_id", id)
		if _, err := t.execInsert(ctx, ins, "adding a member to group %q", groupID); err != nil {
			return err
		}
	}
	return nil
}

// SCIMGroupsOf lists the groups an actor belongs to.
func (t *tx) SCIMGroupsOf(ctx context.Context, actorID string) ([]store.SCIMGroupRow, error) {
	b := t.builder("scim_groups").
		Join("JOIN scim_group_members ON scim_group_members.group_id = scim_groups.id "+
			"AND scim_group_members.tenant_id = scim_groups.tenant_id").
		Where("scim_group_members.actor_id = ?", actorID)
	return t.scimGroups(ctx, b)
}

// DropSCIMMember removes an actor from every group.
func (t *tx) DropSCIMMember(ctx context.Context, actorID string) error {
	_, err := t.execDelete(ctx, t.builder("scim_group_members").Where("scim_group_members.actor_id = ?", actorID),
		"removing %q from its groups", actorID)
	return err
}

// SCIMGroupRoles lists the group to role mappings by group name.
func (t *tx) SCIMGroupRoles(ctx context.Context) ([]core.SCIMGroupRole, error) {
	b := t.builder("scim_group_roles").
		Select("scim_group_roles.display_name", "scim_group_roles.role", "scim_group_roles.created_at").
		OrderBy("scim_group_roles.name_key", core.Ascending)
	rows, err := t.query(ctx, b, "reading group mappings")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.SCIMGroupRole{}
	for rows.Next() {
		var r core.SCIMGroupRole
		var created scimStamp
		if err := rows.Scan(&r.Group, &r.Role, &created); err != nil {
			return nil, mapErr(err, "scanning group mapping")
		}
		if r.CreatedAt, err = scimWhen(created); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, mapRowsErr(rows, "reading group mappings")
}

// PutSCIMGroupRole maps a group name to a role, replacing an earlier mapping.
func (t *tx) PutSCIMGroupRole(ctx context.Context, r core.SCIMGroupRole) error {
	up := t.builder("scim_group_roles").Where("scim_group_roles.name_key = ?", core.SCIMKey(r.Group)).
		Set("display_name", r.Group).
		Set("role", string(r.Role))
	n, err := t.execUpdate(ctx, up, "mapping group %q", r.Group)
	if err != nil || n > 0 {
		return err
	}
	ins := t.insert("scim_group_roles").
		Set("name_key", core.SCIMKey(r.Group)).
		Set("display_name", r.Group).
		Set("role", string(r.Role)).
		Set("created_at", scimAt(r.CreatedAt))
	_, err = t.execInsert(ctx, ins, "mapping group %q", r.Group)
	return err
}

// DeleteSCIMGroupRole removes a mapping, reporting whether there was one.
func (t *tx) DeleteSCIMGroupRole(ctx context.Context, group string) (bool, error) {
	n, err := t.execDelete(ctx, t.builder("scim_group_roles").Where("scim_group_roles.name_key = ?", core.SCIMKey(group)),
		"unmapping group %q", group)
	return n > 0, err
}

// scimStamp is how this engine reads a timestamp column.
type scimStamp = sql.NullTime

func scimAt(t time.Time) any                       { return timeArg(t) }
func scimNullAt(t *time.Time) any                  { return nullTimeArg(t) }
func scimWhen(v scimStamp) (time.Time, error)      { return scanTime(v), nil }
func scimNullWhen(v scimStamp) (*time.Time, error) { return scanNullTime(v), nil }
