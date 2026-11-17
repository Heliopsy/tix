// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestSCIMRowsRoundTrip writes every SCIM table and reads it back, inside one
// tenant and invisible from another.
func TestSCIMRowsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	other := seed(t, s, clk, "other")
	now := clk.Now().UTC().Truncate(time.Second)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		empty, err := tx.GetSCIMConfig(ctx)
		if err != nil || empty.Enabled || empty.AgentID != "" {
			t.Errorf("a tenant never set up reads %+v, %v", empty, err)
		}
		if err := tx.PutSCIMConfig(ctx, store.SCIMConfig{Enabled: true, AgentID: f.actor.ID, UpdatedAt: now}); err != nil {
			return err
		}
		if err := tx.TouchSCIM(ctx, now); err != nil {
			return err
		}
		row := store.SCIMUserRow{ActorID: f.actor.ID, UserName: "Ann@Example.com", ExternalID: "ext", Email: "Ann@Example.com",
			GivenName: "Ann", FamilyName: "Lee", CreatedAt: now, UpdatedAt: now}
		if err := tx.PutSCIMUser(ctx, row); err != nil {
			return err
		}
		row.GivenName = "Anne"
		if err := tx.PutSCIMUser(ctx, row); err != nil {
			return err
		}
		g := store.SCIMGroupRow{ID: "01SCIMGROUPAAAAAAAAAAAAAAA", DisplayName: "Eng", ExternalID: "g1", CreatedAt: now, UpdatedAt: now}
		if err := tx.CreateSCIMGroup(ctx, g); err != nil {
			return err
		}
		g.DisplayName = "Engineering"
		if err := tx.UpdateSCIMGroup(ctx, g); err != nil {
			return err
		}
		if err := tx.SetSCIMGroupMembers(ctx, g.ID, []string{f.actor.ID}); err != nil {
			return err
		}
		return tx.PutSCIMGroupRole(ctx, core.SCIMGroupRole{Group: "Engineering", Role: core.RoleAdmin, CreatedAt: now})
	}); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		c, err := tx.GetSCIMConfig(ctx)
		if err != nil || !c.Enabled || c.AgentID != f.actor.ID || c.LastActivityAt == nil || !c.LastActivityAt.Equal(now) {
			t.Errorf("config %+v, %v", c, err)
		}
		u, err := tx.GetSCIMUser(ctx, f.actor.ID)
		if err != nil || u.GivenName != "Anne" || u.UserName != "Ann@Example.com" || !u.CreatedAt.Equal(now) {
			t.Errorf("user %+v, %v", u, err)
		}
		for name, find := range map[string]func() ([]store.SCIMUserRow, error){
			"by name":     func() ([]store.SCIMUserRow, error) { return tx.SCIMUsersByName(ctx, "ann@EXAMPLE.com") },
			"by external": func() ([]store.SCIMUserRow, error) { return tx.SCIMUsersByExternalID(ctx, "ext") },
			"by email":    func() ([]store.SCIMUserRow, error) { return tx.SCIMUsersByEmail(ctx, "ANN@example.com") },
		} {
			if rows, err := find(); err != nil || len(rows) != 1 {
				t.Errorf("%s: %v, %v", name, rows, err)
			}
		}
		if n, err := tx.CountSCIMUsers(ctx); err != nil || n != 1 {
			t.Errorf("count %d, %v", n, err)
		}
		groups, err := tx.SCIMGroupsByName(ctx, "ENGINEERING")
		if err != nil || len(groups) != 1 || groups[0].ExternalID != "g1" {
			t.Errorf("groups %+v, %v", groups, err)
		}
		if of, err := tx.SCIMGroupsOf(ctx, f.actor.ID); err != nil || len(of) != 1 {
			t.Errorf("groups of the actor %+v, %v", of, err)
		}
		if m, err := tx.SCIMGroupMembers(ctx, "01SCIMGROUPAAAAAAAAAAAAAAA"); err != nil || len(m) != 1 {
			t.Errorf("members %v, %v", m, err)
		}
		roles, err := tx.SCIMGroupRoles(ctx)
		if err != nil || len(roles) != 1 || roles[0].Role != core.RoleAdmin {
			t.Errorf("roles %+v, %v", roles, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading: %v", err)
	}

	if err := s.View(ctx, other.scope, func(tx store.Tx) error {
		if _, err := tx.GetSCIMUser(ctx, f.actor.ID); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("another tenant reads the user: %v", err)
		}
		if g, err := tx.ListSCIMGroups(ctx); err != nil || len(g) != 0 {
			t.Errorf("another tenant lists groups %v, %v", g, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.SetSCIMGroupMembers(ctx, "01SCIMGROUPAAAAAAAAAAAAAAA", nil); err != nil {
			return err
		}
		if err := tx.DropSCIMMember(ctx, f.actor.ID); err != nil {
			return err
		}
		if found, err := tx.DeleteSCIMGroupRole(ctx, "engineering"); err != nil || !found {
			t.Errorf("unmapping: %v, %v", found, err)
		}
		if err := tx.DeleteSCIMGroup(ctx, "01SCIMGROUPAAAAAAAAAAAAAAA"); err != nil {
			return err
		}
		if err := tx.DeleteSCIMGroup(ctx, "01SCIMGROUPAAAAAAAAAAAAAAA"); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("deleting twice: %v", err)
		}
		if err := tx.UpdateSCIMGroup(ctx, store.SCIMGroupRow{ID: "missing", DisplayName: "x"}); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("updating nothing: %v", err)
		}
		if _, err := tx.GetSCIMGroup(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("reading nothing: %v", err)
		}
		return tx.DeleteSCIMUser(ctx, f.actor.ID)
	}); err != nil {
		t.Fatalf("deleting: %v", err)
	}
}
