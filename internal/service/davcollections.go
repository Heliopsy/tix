// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/ical"
	"github.com/heliopsy/tix/internal/store"
)

// davReadScopes are the most a read-only app password acts with, whatever its
// owner's role grants beyond them, and davWriteScopes the most any other
// does. Task delete is never among them: DELETE cancels, it never trashes.
var (
	davReadScopes  = calendarReadScopes
	davWriteScopes = append(slices.Clone(calendarReadScopes), core.ScopeTaskWrite, core.ScopeTaskTransition)
)

// davCeiling is the most a password acts with.
func davCeiling(p core.DAVPassword) []core.Scope {
	if p.ReadOnly {
		return davReadScopes
	}
	return davWriteScopes
}

// davMayWrite reports whether a reader built from the ceiling may change tasks.
func davMayWrite(reader *core.Actor) bool {
	return reader.HasScope(core.ScopeTaskWrite) && reader.HasScope(core.ScopeTaskTransition)
}

// davTaskPagePrefix starts a task page's path, which URL in every object links to.
const davTaskPagePrefix = "/tasks/"

// davSession is one DAV request's app password, its owner and the reader
// built from the owner's current role.
type davSession struct {
	password  core.DAVPassword
	reader    *core.Actor
	tenantKey string
	email     string
}

// errDAVSignedOut is what a DAV request answers when its app password no
// longer signs anybody in: the owner was removed or disabled, or the token is
// not an app password at all. The handler answers it with 401.
func errDAVSignedOut() error {
	return core.Unauthenticated("this app password no longer signs anybody in")
}

// davSessionOf authorizes the app password in ctx and builds its reader.
func (l *Local) davSessionOf(ctx context.Context) (*davSession, error) {
	actor, err := l.authorize(ctx, authz.ActionDAVSync, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if !l.dav.enabled {
		return nil, core.NotFound("CalDAV is turned off on this server")
	}
	out := &davSession{}
	err = l.read(ctx, actor, func(tx store.Tx) error {
		p, err := tx.GetDAVPassword(ctx, actor.TokenID)
		if core.IsKind(err, core.KindNotFound) || err == nil && p.ActorID != actor.ID {
			return errDAVSignedOut()
		}
		if err != nil {
			return err
		}
		out.password = *p
		out.reader, err = ownerReader(ctx, tx, actor.TenantID, p.ActorID, davCeiling(*p))
		if errors.Is(err, errOwnerGone) {
			return errDAVSignedOut()
		}
		if err != nil {
			return err
		}
		tenant, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		out.tenantKey = tenant.Key
		if user, err := accountOfActor(ctx, tx, p.ActorID); err == nil {
			out.email = user.Email
		} else if !core.IsKind(err, core.KindNotFound) {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DAVPrincipal returns the person the app password in ctx signs in as.
func (l *Local) DAVPrincipal(ctx context.Context) (*core.DAVPrincipal, error) {
	s, err := l.davSessionOf(ctx)
	if err != nil {
		return nil, err
	}
	name := s.reader.DisplayName
	if name == "" {
		name = s.reader.Handle
	}
	return &core.DAVPrincipal{TenantKey: s.tenantKey, ActorID: strings.ToLower(s.reader.ID),
		Handle: s.reader.Handle, DisplayName: name, Email: s.email, Write: davMayWrite(s.reader)}, nil
}

// ListDAVCollections lists the collections the app password exposes.
func (l *Local) ListDAVCollections(ctx context.Context) ([]core.DAVCollection, error) {
	s, err := l.davSessionOf(ctx)
	if err != nil {
		return nil, err
	}
	return l.davCollections(ctx, s)
}

// davAssigned is the Assigned to me collection of a session.
func davAssigned(s *davSession) core.DAVCollection {
	return core.DAVCollection{Name: core.DAVAssignedCollection, DisplayName: "Assigned to me",
		Description: "Tasks assigned to " + s.reader.Handle + " in " + s.tenantKey}
}

func (l *Local) davCollections(ctx context.Context, s *davSession) ([]core.DAVCollection, error) {
	var out []core.DAVCollection
	if s.password.Collections.Assigned() {
		out = append(out, davAssigned(s))
	}
	if !s.password.Collections.Projects() {
		return out, nil
	}
	projects, err := l.davProjects(ctx, s, false)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		out = append(out, davProjectCollection(p))
	}
	return out, nil
}

// davProjects lists the projects the reader can read, by key, archived ones
// only when asked: a task assigned to the person may sit in one.
func (l *Local) davProjects(ctx context.Context, s *davSession, archived bool) ([]core.Project, error) {
	readCtx := core.WithActor(ctx, s.reader)
	var out []core.Project
	f := core.ProjectFilter{IncludeArchived: archived, Page: core.Page{Limit: core.MaxPageLimit}}
	for {
		page, next, err := l.ListProjects(readCtx, f)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			break
		}
		f.Page.Cursor = next
	}
	slices.SortFunc(out, func(a, b core.Project) int { return strings.Compare(a.Key, b.Key) })
	return out, nil
}

func davProjectCollection(p core.Project) core.DAVCollection {
	c := core.DAVCollection{Name: core.DAVProjectPrefix + strings.ToLower(p.ID), DisplayName: p.Name,
		Description: "Tasks of " + p.Name + " (" + p.Key + ")", ProjectID: p.ID}
	if rgb, ok := p.Color.RGB(); ok {
		c.Color = rgb.Hex()
	} else if rgb, ok := core.PaletteShade(p.Color); ok {
		c.Color = rgb.Hex()
	}
	return c
}

// davCollection finds one collection the app password exposes, or not found.
func (l *Local) davCollection(ctx context.Context, s *davSession, name string) (core.DAVCollection, error) {
	missing := core.NotFound("collection %q not found", name)
	if name == core.DAVAssignedCollection {
		if !s.password.Collections.Assigned() {
			return core.DAVCollection{}, missing
		}
		return davAssigned(s), nil
	}
	id, ok := strings.CutPrefix(name, core.DAVProjectPrefix)
	if !ok || id == "" || !s.password.Collections.Projects() {
		return core.DAVCollection{}, missing
	}
	project, err := l.GetProject(core.WithActor(ctx, s.reader), strings.ToUpper(id))
	if core.IsKind(err, core.KindNotFound) || core.IsKind(err, core.KindForbidden) ||
		err == nil && (project.ArchivedAt != nil || strings.ToLower(project.ID) != id) {
		return core.DAVCollection{}, missing
	}
	if err != nil {
		return core.DAVCollection{}, err
	}
	return davProjectCollection(*project), nil
}

// davEntry is one member of a collection with what renders it.
type davEntry struct {
	task     core.Task
	category core.StateCategory
	member   core.DAVMember
	project  string
	def      core.WorkflowDefinition
	defs     map[string]core.StateCategory
	start    time.Time
	// uid is the client's UID for a task it created, empty otherwise.
	uid string
	// kept are the lines kept for the reader.
	kept []string
	// name is the client's resource name, beside uid.
	name string
	// parentUID is the RELATED-TO shown, the parent's client UID when it has one.
	parentUID string
}

// named reports whether a resource name a client sent names the entry: its
// client name exactly, or its identifier in any case.
func (e davEntry) named(name string) bool {
	if e.uid != "" {
		return name == e.member.Name
	}
	return strings.ToLower(name) == e.member.Name
}

// davMembers computes a collection: its tasks that are not in the trash, not
// cancelled, and open or completed within the window, open first, then by
// due date, then most recently updated, at most caldav.max_objects. It
// returns how many were left out past the bound.
func (l *Local) davMembers(ctx context.Context, s *davSession, c core.DAVCollection, origin string) ([]davEntry, int, error) {
	readCtx := core.WithActor(ctx, s.reader)
	now := l.clock.Now()
	cutoff := now.Add(-l.dav.window)
	scope := core.TaskFilter{IncludeSnoozed: true}
	if c.ProjectID != "" {
		scope.ProjectIDs = []string{c.ProjectID}
	} else {
		scope.AssigneeIDs = []string{s.reader.ID}
	}
	workflows, err := l.davWorkflows(ctx, s)
	if err != nil {
		return nil, 0, err
	}
	open := scope
	open.Exclude.Statuses = workflows.terminal
	found, err := l.davList(readCtx, open, time.Time{})
	if err != nil {
		return nil, 0, err
	}
	if l.dav.window > 0 && len(workflows.done) > 0 {
		done := scope
		done.Statuses = workflows.done
		recent, err := l.davList(readCtx, done, cutoff)
		if err != nil {
			return nil, 0, err
		}
		found = append(found, recent...)
	}
	seen := map[string]bool{}
	var out []davEntry
	for _, t := range found {
		if seen[t.ID] || t.DeletedAt != nil {
			continue
		}
		seen[t.ID] = true
		w, ok := workflows.byProject[t.ProjectID]
		if !ok {
			continue
		}
		category := w.def.CategoryOf(t.Status)
		if category == core.CategoryCancelled {
			continue
		}
		if category == core.CategoryDone && (l.dav.window == 0 || completedAt(t).Before(cutoff)) {
			continue
		}
		e := davEntry{task: t, category: category, project: w.key, def: w.def, defs: w.categories}
		e.start, _ = ical.TodoStart(t, now)
		out = append(out, e)
	}
	slices.SortFunc(out, davOrder)
	left := 0
	if len(out) > l.dav.maxObjects {
		left = len(out) - l.dav.maxObjects
		out = out[:l.dav.maxObjects]
	}
	if err := l.davObjectsOf(ctx, s, out); err != nil {
		return nil, 0, err
	}
	for i := range out {
		e := &out[i]
		e.member = core.DAVMember{Name: strings.ToLower(e.task.ID), ETag: ical.ETag(ical.ETagInput{
			TaskID: e.task.ID, Version: e.task.Version, UpdatedAt: e.task.UpdatedAt, ProjectKey: e.project,
			Categories: e.defs, Origin: origin, ParentUID: e.parentUID, Start: e.start, UID: e.uid,
			Kept: ical.KeptDigest(e.kept)})}
		if e.uid != "" {
			e.member.Name = e.name
		}
	}
	slices.SortFunc(out, func(a, b davEntry) int { return strings.Compare(a.member.Name, b.member.Name) })
	return out, left, nil
}

// davOrder is the order the bound keeps: open before done, then soonest due
// with undated last, then most recently updated.
func davOrder(a, b davEntry) int {
	ad, bd := a.category == core.CategoryDone, b.category == core.CategoryDone
	if ad != bd {
		if ad {
			return 1
		}
		return -1
	}
	switch {
	case a.task.DueAt != nil && b.task.DueAt == nil:
		return -1
	case a.task.DueAt == nil && b.task.DueAt != nil:
		return 1
	case a.task.DueAt != nil:
		if c := a.task.DueAt.Compare(*b.task.DueAt); c != 0 {
			return c
		}
	}
	if c := b.task.UpdatedAt.Compare(a.task.UpdatedAt); c != 0 {
		return c
	}
	return strings.Compare(a.task.ID, b.task.ID)
}

func completedAt(t core.Task) time.Time {
	if t.CompletedAt != nil {
		return *t.CompletedAt
	}
	return t.UpdatedAt
}

// davList pages through a task listing as the reader, most recently updated
// first. A non-zero floor stops at the first task updated before it, which
// bounds the finished work read to the window.
func (l *Local) davList(ctx context.Context, f core.TaskFilter, floor time.Time) ([]core.Task, error) {
	f.Page = core.Page{Sort: core.SortUpdatedAt, Direction: core.Descending, Limit: core.MaxPageLimit}
	var out []core.Task
	for {
		page, err := l.ListTasks(ctx, f)
		if err != nil {
			return nil, err
		}
		for _, t := range page.Tasks {
			if !floor.IsZero() && t.UpdatedAt.Before(floor) {
				return out, nil
			}
			out = append(out, t)
		}
		if page.NextCursor == "" {
			return out, nil
		}
		f.Page.Cursor = page.NextCursor
	}
}

// davWorkflow is what a project's workflow contributes to its objects.
type davWorkflow struct {
	key        string
	def        core.WorkflowDefinition
	categories map[string]core.StateCategory
}

// davWorkflowSet is every project's workflow, and the state keys the two
// listings select by.
type davWorkflowSet struct {
	byProject map[string]davWorkflow
	// terminal names the states terminal in every workflow defining them;
	// done names the states of the done category in any.
	terminal, done []string
}

func (l *Local) davWorkflows(ctx context.Context, s *davSession) (davWorkflowSet, error) {
	out := davWorkflowSet{byProject: map[string]davWorkflow{}}
	projects, err := l.davProjects(ctx, s, true)
	if err != nil {
		return out, err
	}
	err = l.read(ctx, s.reader, func(tx store.Tx) error {
		workflows, err := tx.ListWorkflows(ctx)
		if err != nil {
			return err
		}
		byID := map[string]core.WorkflowDefinition{}
		done := map[string]bool{}
		for _, w := range workflows {
			byID[w.ID] = w.Definition
			for _, st := range w.Definition.States {
				if w.Definition.CategoryOf(st.Key) == core.CategoryDone {
					done[st.Key] = true
				}
			}
		}
		if out.terminal, err = everywhereTerminal(ctx, tx); err != nil {
			return err
		}
		for k := range done {
			out.done = append(out.done, k)
		}
		slices.Sort(out.done)
		for _, p := range projects {
			def := byID[p.WorkflowID]
			cats := make(map[string]core.StateCategory, len(def.States))
			for _, st := range def.States {
				cats[st.Key] = def.CategoryOf(st.Key)
			}
			out.byProject[p.ID] = davWorkflow{key: p.Key, def: def, categories: cats}
		}
		return nil
	})
	return out, err
}

// davSyncKeep is how many snapshots per app password and collection outlive
// davSyncHorizon; every younger one is kept too.
const (
	davSyncKeep    = 3
	davSyncHorizon = 24 * time.Hour
	// davEventSpan is how far below its latest event sequence a snapshot
	// counts the tenant's events. A write that took its sequence number
	// before another committed and itself commits after the snapshot lands
	// inside the span and moves the count, so the short cut is not taken.
	davEventSpan = 1024
	// DAVSyncTokenPrefix starts every sync token, a URI as RFC 6578 asks.
	DAVSyncTokenPrefix = "https://tix.red/ns/sync/"
)

// DAVSnapshot answers a collection's members and what changed since a token.
func (l *Local) DAVSnapshot(ctx context.Context, in core.DAVSnapshotInput) (*core.DAVSnapshot, error) {
	s, err := l.davSessionOf(ctx)
	if err != nil {
		return nil, err
	}
	c, err := l.davCollection(ctx, s, in.Collection)
	if err != nil {
		return nil, err
	}
	since := ""
	if in.Since != "" {
		var ok bool
		if since, ok = strings.CutPrefix(in.Since, DAVSyncTokenPrefix); !ok || since == "" {
			return nil, core.ErrDAVSyncToken()
		}
	}
	scope := core.TenantScope{TenantID: s.reader.TenantID}
	config := l.davConfig(s, in.Origin)
	now := l.clock.Now()

	var prior *store.DAVSyncState
	var seq, mark int64
	err = l.store.View(ctx, scope, func(tx store.Tx) error {
		if seq, mark, err = davEventMark(ctx, tx); err != nil {
			return err
		}
		if since == "" {
			return nil
		}
		prior, err = tx.GetDAVSyncState(ctx, s.password.ID, c.Name, since)
		if core.IsKind(err, core.KindNotFound) {
			return core.ErrDAVSyncToken()
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if prior != nil && prior.EventSeq == seq && prior.EventMark == mark && prior.Config == config && now.Before(prior.ValidUntil) {
		return &core.DAVSnapshot{Collection: c, Members: prior.Members, Token: in.Since}, nil
	}

	entries, left, err := l.davMembers(ctx, s, c, in.Origin)
	if err != nil {
		return nil, err
	}
	validUntil := l.davValidUntil(entries).Truncate(time.Second)
	members := make([]core.DAVMember, len(entries))
	for i, e := range entries {
		members[i] = e.member
	}
	if left > 0 {
		c.Description += fmt.Sprintf("; %d more left out past the limit of %d", left, l.dav.maxObjects)
	}
	out := &core.DAVSnapshot{Collection: c, Members: members, LeftOut: left}
	var before []core.DAVMember
	if prior != nil {
		before = prior.Members
	}
	out.Changed, out.Removed = davDiff(before, members)
	answered := members
	if in.Limit > 0 && len(out.Changed)+len(out.Removed) > in.Limit {
		out.Changed, out.Removed, answered = davTruncate(before, out.Changed, out.Removed, in.Limit)
		out.Truncated = true
	}
	hash := davSetHash(answered)
	out.Token = DAVSyncTokenPrefix + hash
	state := store.DAVSyncState{TokenID: s.password.ID, ActorID: s.password.ActorID, Collection: c.Name,
		SetHash: hash, Members: answered, EventSeq: seq, EventMark: mark, Config: config, ValidUntil: validUntil,
		SeenAt: now}
	if out.Truncated {
		state.EventSeq, state.EventMark = -1, -1
	}
	existing := prior
	if existing == nil || existing.SetHash != hash {
		existing = nil
		err = l.store.View(ctx, scope, func(tx store.Tx) error {
			found, err := tx.GetDAVSyncState(ctx, s.password.ID, c.Name, hash)
			if err == nil {
				existing = found
			} else if !core.IsKind(err, core.KindNotFound) {
				return err
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if existing != nil && existing.EventSeq == state.EventSeq && existing.EventMark == state.EventMark &&
		existing.Config == config && existing.ValidUntil.Equal(validUntil) {
		return out, nil
	}
	err = l.store.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.PutDAVSyncState(ctx, &state); err != nil {
			return err
		}
		_, err := tx.PruneDAVSyncStates(ctx, s.password.ID, c.Name, davSyncKeep, now.Add(-davSyncHorizon))
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// davValidUntil is the first moment after now that the members could change
// with no event: a finished task leaving the window, or a snooze passing so
// its object loses DTSTART. Without either it is davNoExpiry, a fixed instant
// so that two polls of an unchanged collection agree on it.
func (l *Local) davValidUntil(entries []davEntry) time.Time {
	until := davNoExpiry
	for _, e := range entries {
		if e.category == core.CategoryDone {
			if at := completedAt(e.task).Add(l.dav.window); at.Before(until) {
				until = at
			}
		}
		if !e.start.IsZero() && e.task.SnoozedUntil != nil && e.task.SnoozedUntil.Before(until) {
			until = *e.task.SnoozedUntil
		}
	}
	return until
}

// davNoExpiry is the validity of a snapshot nothing but an event can change.
var davNoExpiry = time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)

// davEventMark reads the tenant's latest event sequence and the count of
// events in the span below it.
func davEventMark(ctx context.Context, tx store.Tx) (seq, mark int64, err error) {
	if seq, err = tx.LatestEventSeq(ctx); err != nil {
		return 0, 0, err
	}
	if mark, err = tx.CountEventsAfter(ctx, seq-davEventSpan); err != nil {
		return 0, 0, err
	}
	return seq, mark, nil
}

// davConfig names everything besides the tenant's events that a
// collection's members depend on: the deployment's window and bound, the
// address objects are built under, the password's choice and the reader's
// authority.
func (l *Local) davConfig(s *davSession, origin string) string {
	scopes := make([]string, len(s.reader.Scopes))
	for i, sc := range s.reader.Scopes {
		scopes[i] = string(sc)
	}
	return strings.Join([]string{l.dav.window.String(), strconv.Itoa(l.dav.maxObjects), origin,
		string(s.password.Collections), strings.Join(scopes, ",")}, "|")
}

// davDiff is what changed from one set of members to another, in name order.
func davDiff(before, after []core.DAVMember) (changed []core.DAVMember, removed []string) {
	was := make(map[string]string, len(before))
	for _, m := range before {
		was[m.Name] = m.ETag
	}
	now := make(map[string]bool, len(after))
	for _, m := range after {
		now[m.Name] = true
		if tag, ok := was[m.Name]; !ok || tag != m.ETag {
			changed = append(changed, m)
		}
	}
	for _, m := range before {
		if !now[m.Name] {
			removed = append(removed, m.Name)
		}
	}
	slices.Sort(removed)
	return changed, removed
}

// davTruncate answers the first limit changes, removals after changes, and
// the set those lead to from before, which the next token stands for.
func davTruncate(before, changed []core.DAVMember, removed []string, limit int) ([]core.DAVMember, []string, []core.DAVMember) {
	if len(changed) >= limit {
		changed, removed = changed[:limit], nil
	} else {
		removed = removed[:limit-len(changed)]
	}
	set := make(map[string]string, len(before)+len(changed))
	for _, m := range before {
		set[m.Name] = m.ETag
	}
	for _, m := range changed {
		set[m.Name] = m.ETag
	}
	for _, name := range removed {
		delete(set, name)
	}
	out := make([]core.DAVMember, 0, len(set))
	for name, tag := range set {
		out = append(out, core.DAVMember{Name: name, ETag: tag})
	}
	slices.SortFunc(out, func(a, b core.DAVMember) int { return cmp.Compare(a.Name, b.Name) })
	return changed, removed, out
}

// davSetHash names a set of members, sorted by name.
func davSetHash(members []core.DAVMember) string {
	h := sha256.New()
	for _, m := range members {
		h.Write([]byte(m.Name))
		h.Write([]byte{0})
		h.Write([]byte(m.ETag))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// GetDAVObjects renders the named members of a collection.
func (l *Local) GetDAVObjects(ctx context.Context, in core.DAVObjectsInput) ([]core.DAVObject, error) {
	s, err := l.davSessionOf(ctx)
	if err != nil {
		return nil, err
	}
	c, err := l.davCollection(ctx, s, in.Collection)
	if err != nil {
		return nil, err
	}
	entries, _, err := l.davMembers(ctx, s, c, in.Origin)
	if err != nil {
		return nil, err
	}
	now := l.clock.Now()
	origin := strings.TrimRight(in.Origin, "/")
	out := []core.DAVObject{}
	for _, e := range entries {
		if len(in.Names) > 0 && !slices.ContainsFunc(in.Names, e.named) {
			continue
		}
		out = append(out, core.DAVObject{Name: e.member.Name, ETag: e.member.ETag, Data: davRender(e, origin, now)})
	}
	return out, nil
}

// davRender writes an entry's object.
func davRender(e davEntry, origin string, now time.Time) []byte {
	pageURL := ""
	if origin != "" {
		pageURL = origin + davTaskPagePrefix + url.PathEscape(e.task.Ref)
	}
	return ical.RenderTodo(ical.Todo{Task: e.task, Category: e.category, ParentUID: e.parentUID, URL: pageURL,
		Now: now, UID: e.uid, Kept: e.kept})
}

// davObjectsOf fills in the client UIDs and names of the entries' tasks and
// their parents, and what is kept for the reader.
func (l *Local) davObjectsOf(ctx context.Context, s *davSession, entries []davEntry) error {
	if len(entries) == 0 {
		return nil
	}
	ids := make([]string, 0, 2*len(entries))
	for _, e := range entries {
		ids = append(ids, e.task.ID)
		if e.task.ParentID != "" {
			ids = append(ids, e.task.ParentID)
		}
	}
	var (
		objects []store.DAVObjectName
		kept    map[string]string
	)
	err := l.store.View(ctx, core.TenantScope{TenantID: s.reader.TenantID}, func(tx store.Tx) error {
		var err error
		if objects, err = tx.ListDAVObjects(ctx, ids); err != nil {
			return err
		}
		own := make([]string, len(entries))
		for i, e := range entries {
			own[i] = e.task.ID
		}
		kept, err = tx.GetDAVProps(ctx, s.reader.ID, own)
		return err
	})
	if err != nil {
		return err
	}
	byTask := make(map[string]store.DAVObjectName, len(objects))
	for _, o := range objects {
		byTask[o.TaskID] = o
	}
	for i := range entries {
		e := &entries[i]
		if o, ok := byTask[e.task.ID]; ok {
			e.uid, e.name = o.UID, o.Name
		}
		if e.task.ParentID != "" {
			e.parentUID = ical.TodoUID(e.task.ParentID, byTask[e.task.ParentID].UID)
		}
		e.kept = splitKept(kept[e.task.ID])
	}
	return nil
}

// davKeptSep joins kept lines in storage.
const davKeptSep = "\r\n"

func splitKept(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, davKeptSep)
}

var _ core.TaskCollections = (*Local)(nil)
