// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/query"
	"github.com/heliopsy/tix/internal/store"
)

// Events and audit actions of the calendar feed. The subject is gated like a
// token's, because a feed is a credential.
const (
	eventCalendarCreated core.EventType = "calendar_feed.created"
	eventCalendarRotated core.EventType = "calendar_feed.rotated"
	eventCalendarFilter  core.EventType = "calendar_feed.filtered"
	eventCalendarRevoked core.EventType = "calendar_feed.revoked"

	auditCalendarCreate = "calendar_feed.create"
	auditCalendarRotate = "calendar_feed.rotate"
	auditCalendarFilter = "calendar_feed.filter"
	auditCalendarRevoke = "calendar_feed.revoke"

	subjectCalendarFeed = "calendar_feed"
)

// calendarReadScopes are the most a feed ever reads with, whatever its owner's
// role grants beyond them.
var calendarReadScopes = []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead, core.ScopeWorkflowRead}

// dueFloor is a due-after bound earlier than any deadline, which drops the
// tasks carrying none in the query rather than after it.
var dueFloor = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)

// GetCalendarFeed returns the caller's live feed.
func (l *Local) GetCalendarFeed(ctx context.Context) (*core.CalendarFeed, error) {
	actor, err := l.authorize(ctx, authz.ActionCalendarFeed, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.CalendarFeed
	err = l.read(ctx, actor, func(tx store.Tx) error {
		out, err = tx.LiveCalendarFeed(ctx, actor.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateCalendarFeed issues the caller a feed, refusing when one is live.
func (l *Local) CreateCalendarFeed(ctx context.Context, in core.CalendarFeedInput) (*core.IssuedCalendarFeed, error) {
	actor, err := l.authorize(ctx, authz.ActionCalendarFeed, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if in, err = in.Normalize(); err != nil {
		return nil, err
	}
	var out *core.IssuedCalendarFeed
	err = l.write(ctx, actor, func(m *mutation) error {
		if err := requireMember(ctx, m.tx, actor.ID); err != nil {
			return err
		}
		if _, err := m.tx.LiveCalendarFeed(ctx, actor.ID); err == nil {
			return core.Conflict("you already have a calendar feed; rotate it for a new address or revoke it")
		} else if !core.IsKind(err, core.KindNotFound) {
			return err
		}
		if err := l.checkCalendarFilter(ctx, m.tx, actor.ID, in.Filter); err != nil {
			return err
		}
		out, err = issueCalendarFeed(ctx, m, actor.ID, in.Filter)
		if err != nil {
			return err
		}
		return m.Record(auditCalendarCreate, eventCalendarCreated, subjectCalendarFeed, out.ID, "",
			nil, out.CalendarFeed, map[string]any{"actor_id": actor.ID, "filter": out.Filter})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RotateCalendarFeed revokes the caller's feed and issues a new secret with
// the same filter, in one transaction, so there is never a moment with two
// live addresses or none.
func (l *Local) RotateCalendarFeed(ctx context.Context) (*core.IssuedCalendarFeed, error) {
	actor, err := l.authorize(ctx, authz.ActionCalendarFeed, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.IssuedCalendarFeed
	err = l.write(ctx, actor, func(m *mutation) error {
		old, err := m.tx.LiveCalendarFeed(ctx, actor.ID)
		if err != nil {
			return err
		}
		if err := m.tx.RevokeCalendarFeed(ctx, old.ID, m.now); err != nil {
			return err
		}
		out, err = issueCalendarFeed(ctx, m, actor.ID, old.Filter)
		if err != nil {
			return err
		}
		return m.Record(auditCalendarRotate, eventCalendarRotated, subjectCalendarFeed, out.ID, "",
			old, out.CalendarFeed, map[string]any{"actor_id": actor.ID, "replaced": old.ID})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SetCalendarFeedFilter changes the caller's feed filter, keeping its URL.
func (l *Local) SetCalendarFeedFilter(ctx context.Context, in core.CalendarFeedInput) (*core.CalendarFeed, error) {
	actor, err := l.authorize(ctx, authz.ActionCalendarFeed, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if in, err = in.Normalize(); err != nil {
		return nil, err
	}
	var out *core.CalendarFeed
	err = l.write(ctx, actor, func(m *mutation) error {
		feed, err := m.tx.LiveCalendarFeed(ctx, actor.ID)
		if err != nil {
			return err
		}
		if feed.Filter == in.Filter {
			out = feed
			return nil
		}
		if err := l.checkCalendarFilter(ctx, m.tx, actor.ID, in.Filter); err != nil {
			return err
		}
		if err := m.tx.SetCalendarFeedFilter(ctx, feed.ID, in.Filter); err != nil {
			return err
		}
		before := *feed
		feed.Filter = in.Filter
		out = feed
		return m.Record(auditCalendarFilter, eventCalendarFilter, subjectCalendarFeed, feed.ID, "",
			before, *feed, map[string]any{"actor_id": actor.ID, "filter": feed.Filter})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeCalendarFeed stops the caller's feed.
func (l *Local) RevokeCalendarFeed(ctx context.Context) error {
	actor, err := l.authorize(ctx, authz.ActionCalendarFeed, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		feed, err := m.tx.LiveCalendarFeed(ctx, actor.ID)
		if err != nil {
			return err
		}
		if err := m.tx.RevokeCalendarFeed(ctx, feed.ID, m.now); err != nil {
			return err
		}
		after := *feed
		after.RevokedAt = &m.now
		return m.Record(auditCalendarRevoke, eventCalendarRevoked, subjectCalendarFeed, feed.ID, "",
			*feed, after, map[string]any{"actor_id": actor.ID})
	})
}

// issueCalendarFeed mints a secret and stores a live feed behind its hash.
func issueCalendarFeed(ctx context.Context, m *mutation, actorID, filter string) (*core.IssuedCalendarFeed, error) {
	token, hash, err := auth.MintCalendarToken()
	if err != nil {
		return nil, err
	}
	feed := core.CalendarFeed{ID: id.NewAt(m.now), ActorID: actorID, Filter: filter, CreatedAt: m.now}
	if err := m.tx.CreateCalendarFeed(ctx, &feed, hash); err != nil {
		return nil, err
	}
	return &core.IssuedCalendarFeed{CalendarFeed: feed, Token: token}, nil
}

// requireMember refuses an actor holding no membership in this tenant. A feed
// reads with the owner's membership role, so without one it could read
// nothing, and an address that answers nothing is worse than a refusal now.
func requireMember(ctx context.Context, tx store.Tx, actorID string) error {
	role, err := userRole(ctx, tx, actorID)
	if err != nil {
		return err
	}
	if role == "" {
		return core.Invalid("a calendar feed reads as a member of this tenant, and this actor holds no membership; sign in as a user")
	}
	return nil
}

// checkCalendarFilter parses and resolves a filter, so a typo is refused when
// it is stored rather than answered with an empty calendar forever after.
func (l *Local) checkCalendarFilter(ctx context.Context, tx store.Tx, actorID, filter string) error {
	f, err := calendarFilter(ctx, tx, actorID, filter, l.clock.Now())
	if err != nil {
		return err
	}
	return resolveTaskFilter(ctx, tx, &f)
}

// calendarFilter is the listing a feed runs: its own expression, or the
// owner's unfinished tasks, narrowed to tasks with a deadline and ordered by
// it, latest first.
func calendarFilter(ctx context.Context, tx store.Tx, ownerID, expr string, now time.Time) (core.TaskFilter, error) {
	var f core.TaskFilter
	if expr == "" {
		terminal, err := everywhereTerminal(ctx, tx)
		if err != nil {
			return f, err
		}
		f.AssigneeIDs = []string{ownerID}
		f.Exclude.Statuses = terminal
	} else {
		parsed, err := query.ParseAt(expr, now)
		if err != nil {
			return f, core.Invalid("calendar filter: %v", err).WithDetail(core.DetailField, "filter")
		}
		f = parsed
	}
	if f.DueAfter == nil {
		f.DueAfter = &dueFloor
	}
	f.IncludeDeleted = false
	f.Page = core.Page{Sort: core.SortDueAt, Direction: core.Descending, Limit: core.MaxPageLimit}
	return f, nil
}

// everywhereTerminal names the states terminal in every workflow that defines
// them. Excluding a name terminal in one workflow and open in another would
// hide open work, so those are left to the category check after the listing.
func everywhereTerminal(ctx context.Context, tx store.Tx) ([]string, error) {
	workflows, err := tx.ListWorkflows(ctx)
	if err != nil {
		return nil, err
	}
	terminal := map[string]bool{}
	for _, w := range workflows {
		for _, s := range w.Definition.States {
			if prev, seen := terminal[s.Key]; seen && !prev {
				continue
			}
			terminal[s.Key] = s.Terminal
		}
	}
	var out []string
	for key, t := range terminal {
		if t {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out, nil
}

// ReadCalendarFeed answers the feed a secret addresses.
func (l *Local) ReadCalendarFeed(ctx context.Context, token string) (*core.CalendarEntries, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if err := auth.ParseCalendarToken(token); err != nil {
		return nil, err
	}
	notFound := core.NotFound("calendar feed not found")
	system := core.SystemActor(scope.TenantID)
	var (
		out    core.CalendarEntries
		reader *core.Actor
		filter core.TaskFilter
	)
	err = l.read(ctx, system, func(tx store.Tx) error {
		feed, err := tx.CalendarFeedByHash(ctx, auth.HashToken(token))
		if err != nil {
			return err
		}
		reader, err = feedReader(ctx, tx, scope.TenantID, feed.ActorID)
		if err != nil {
			return err
		}
		out.Feed, out.OwnerHandle = *feed, reader.Handle
		filter, err = calendarFilter(ctx, tx, feed.ActorID, feed.Filter, l.clock.Now())
		return err
	})
	if core.IsKind(err, core.KindNotFound) {
		return nil, notFound
	}
	if err != nil {
		return nil, err
	}
	readCtx := core.WithActor(ctx, reader)
	tasks, err := l.calendarTasks(readCtx, filter)
	if err != nil {
		return nil, err
	}
	out.Entries, err = l.calendarEntries(ctx, reader, tasks, out.Feed.Filter == "")
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// feedReader builds the actor a feed reads as: its owner, with the owner's
// current role cut down to reading, or not found when the owner is no longer
// a member or their account is disabled.
func feedReader(ctx context.Context, tx store.Tx, tenantID, actorID string) (*core.Actor, error) {
	owner, err := tx.GetActor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	role, err := userRole(ctx, tx, actorID)
	if err != nil {
		return nil, err
	}
	if role == "" || !role.Valid() {
		return nil, core.NotFound("calendar feed not found")
	}
	user, err := tx.GetUser(ctx, actorID)
	switch {
	case err == nil && user.DisabledAt != nil:
		return nil, core.NotFound("calendar feed not found")
	case err != nil && !core.IsKind(err, core.KindNotFound):
		return nil, err
	}
	member := &core.Actor{Role: role}
	var scopes []core.Scope
	for _, s := range calendarReadScopes {
		if member.HasScope(s) {
			scopes = append(scopes, s)
		}
	}
	return &core.Actor{
		ID: owner.ID, TenantID: tenantID, Kind: owner.Kind, Handle: owner.Handle,
		DisplayName: owner.DisplayName, Scopes: scopes,
	}, nil
}

// calendarTasks pages through the feed's listing as its reader, up to the
// bound on one document.
func (l *Local) calendarTasks(ctx context.Context, f core.TaskFilter) ([]core.Task, error) {
	var out []core.Task
	for {
		page, err := l.ListTasks(ctx, f)
		if err != nil {
			return nil, err
		}
		for _, t := range page.Tasks {
			if t.DueAt != nil && t.DeletedAt == nil {
				out = append(out, t)
			}
		}
		if len(out) >= core.MaxCalendarEntries {
			return out[:core.MaxCalendarEntries], nil
		}
		if page.NextCursor == "" {
			return out, nil
		}
		f.Page.Cursor = page.NextCursor
	}
}

// calendarEntries pairs each task with its state's category and label, and
// drops finished work from a feed reading the default selection.
func (l *Local) calendarEntries(ctx context.Context, reader *core.Actor, tasks []core.Task, openOnly bool) ([]core.CalendarEntry, error) {
	defs := map[string]core.WorkflowDefinition{}
	err := l.read(ctx, reader, func(tx store.Tx) error {
		for _, t := range tasks {
			if _, ok := defs[t.ProjectID]; ok {
				continue
			}
			p, err := tx.GetProject(ctx, t.ProjectID)
			if err != nil {
				return err
			}
			w, err := tx.GetWorkflowByID(ctx, p.WorkflowID)
			if err != nil {
				return err
			}
			defs[t.ProjectID] = w.Definition
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]core.CalendarEntry, 0, len(tasks))
	for _, t := range tasks {
		def := defs[t.ProjectID]
		if openOnly && def.IsTerminal(t.Status) {
			continue
		}
		label := t.Status
		if s, ok := def.State(t.Status); ok && s.Label != "" {
			label = s.Label
		}
		out = append(out, core.CalendarEntry{Task: t, Category: def.CategoryOf(t.Status), State: label})
	}
	slices.SortStableFunc(out, func(a, b core.CalendarEntry) int {
		if c := b.Task.DueAt.Compare(*a.Task.DueAt); c != 0 {
			return c
		}
		return strings.Compare(a.Task.ID, b.Task.ID)
	})
	return out, nil
}
