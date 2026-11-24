// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/ical"
	"github.com/heliopsy/tix/internal/store"
)

// MaxDAVKeptBytes bounds what is kept for one person and task: the content
// lines a client sent that tix has no field for. Past it the object is still
// accepted and the extras are dropped.
const MaxDAVKeptBytes = 8 << 10

// maxDAVName bounds a resource name or a UID a client chooses.
const maxDAVName = 255

// davStatusOf is the STATUS tix writes for a category, and davCategoryOf the
// category a STATUS a client sends moves a task to.
var (
	davStatusOf = map[core.StateCategory]string{
		core.CategoryDone: ical.StatusCompleted, core.CategoryInProgress: ical.StatusInProcess,
	}
	davCategoryOf = map[string]core.StateCategory{
		ical.StatusCompleted: core.CategoryDone, ical.StatusInProcess: core.CategoryInProgress,
		ical.StatusNeedsAction: core.CategoryTodo, ical.StatusCancelled: core.CategoryCancelled,
	}
)

// davWriter is one write's session, collection and what the client sent.
type davWriter struct {
	l   *Local
	s   *davSession
	c   core.DAVCollection
	v   *ical.TodoValues
	now time.Time
	// as is the context every task operation runs in: the owner, cut to the
	// ceiling, on the request's access channel.
	as context.Context
}

// davWriterOf authorizes a write and reads the body.
func (l *Local) davWriterOf(ctx context.Context, collection string, data []byte) (*davWriter, error) {
	s, err := l.davSessionOf(ctx)
	if err != nil {
		return nil, err
	}
	c, err := l.davCollection(ctx, s, collection)
	if err != nil {
		return nil, err
	}
	if !davMayWrite(s.reader) {
		return nil, core.DAVRefusal(core.ReasonDAVReadOnly, "this app password or its owner's role does not change tasks")
	}
	w := &davWriter{l: l, s: s, c: c, now: l.clock.Now(), as: core.WithActor(ctx, s.reader)}
	if data != nil {
		zone, err := l.davZone(ctx, s)
		if err != nil {
			return nil, err
		}
		if w.v, err = ical.ReadTodo(data, zone); err != nil {
			return nil, core.DAVRefusal(core.ReasonDAVInvalidObject, "%s", err.Error())
		}
	}
	return w, nil
}

// davZone is the zone a floating time is read in: the person's own, then the
// tenant's, then the deployment's, then UTC.
func (l *Local) davZone(ctx context.Context, s *davSession) (*time.Location, error) {
	var name string
	err := l.read(ctx, s.reader, func(tx store.Tx) error {
		settings, err := l.timeSettings(ctx, tx, s.reader)
		name = settings.Effective.Timezone
		return err
	})
	if err != nil {
		return nil, err
	}
	if loc, err := time.LoadLocation(name); name != "" && err == nil {
		return loc, nil
	}
	return time.UTC, nil
}

// entry finds the member a name names, nil when the collection holds none.
func (w *davWriter) entry(ctx context.Context, name, origin string) (*davEntry, error) {
	entries, _, err := w.l.davMembers(ctx, w.s, w.c, origin)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if entries[i].named(name) {
			return &entries[i], nil
		}
	}
	return nil, nil
}

// PutDAVObject applies a client's object to its task, or creates one.
func (l *Local) PutDAVObject(ctx context.Context, in core.DAVPutInput) (*core.DAVPutResult, error) {
	if in.Data == nil {
		in.Data = []byte{}
	}
	w, err := l.davWriterOf(ctx, in.Collection, in.Data)
	if err != nil {
		return nil, err
	}
	e, err := w.entry(ctx, in.Name, in.Origin)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return w.create(ctx, in)
	}
	switch {
	case in.IfNoneMatch:
		return nil, core.DAVRefusal(core.ReasonDAVStale, "the object already exists")
	case in.IfMatch == "":
		return nil, core.DAVRefusal(core.ReasonDAVIfMatchRequired, "an update needs If-Match with the object's ETag")
	case !davETagMatches(in.IfMatch, e.member.ETag):
		return nil, core.DAVRefusal(core.ReasonDAVStale, "the object changed since the client read it")
	case w.v.UID != ical.TodoUID(e.task.ID, e.uid):
		return nil, core.DAVRefusal(core.ReasonDAVUIDConflict, "an object keeps its UID")
	}
	task, err := w.apply(e)
	if err != nil {
		return nil, err
	}
	if err := w.keep(ctx, e.task.ID, e.kept); err != nil {
		return nil, err
	}
	return &core.DAVPutResult{Task: task, KeptDropped: w.dropped()}, nil
}

// davETagMatches reports whether an If-Match value names the entity tag, by
// the strong comparison RFC 9110 asks of If-Match.
func davETagMatches(header, etag string) bool {
	for _, v := range strings.Split(header, ",") {
		if v = strings.TrimSpace(v); v == "*" || v == etag {
			return true
		}
	}
	return false
}

// apply makes the differences between what the client sent and what tix
// would render for the task: a dry run of any move first, so a refused move
// changes nothing, then the update, the snooze and the move, each through
// its ordinary operation with the version the one before left.
func (w *davWriter) apply(e *davEntry) (*core.Task, error) {
	t := e.task
	ref := core.TaskRef{ID: t.ID}
	target, move := davTarget(e.category, w.v)
	var route []string
	if move {
		var err error
		if route, err = davRoute(e.def, t.Status, target); err != nil {
			return nil, err
		}
		dry, err := w.l.TransitionRoute(w.as, ref, core.RouteInput{Route: route, Version: t.Version, DryRun: true})
		if err != nil {
			return nil, davWriteError(err)
		}
		if dry.Partial() {
			return nil, core.DAVRefusal(core.ReasonDAVRefused, "%s", dry.Sentence())
		}
	}
	out := &t
	if upd, changed := davUpdate(t, w.v); changed {
		upd.Version = out.Version
		got, err := w.l.UpdateTask(w.as, ref, upd)
		if err != nil {
			return nil, davWriteError(err)
		}
		out = got
	}
	if until, ok := davSnooze(t, w.v, w.now); ok && (!move || target != core.CategoryDone && target != core.CategoryCancelled) {
		got, err := w.l.SnoozeTask(w.as, ref, core.SnoozeInput{Until: until})
		if err != nil {
			return nil, davWriteError(err)
		}
		out = got
	}
	if move {
		r, err := w.l.TransitionRoute(w.as, ref, core.RouteInput{Route: route, Version: out.Version})
		if err != nil {
			return nil, davWriteError(err)
		}
		if r.Partial() {
			return nil, core.DAVRefusal(core.ReasonDAVRefused, "%s", r.Sentence())
		}
		out = r.Task
	}
	return out, nil
}

// davTarget is the category a client's status moves a task to, if any. An
// absent STATUS reads COMPLETED or PERCENT-COMPLETE:100 as done, and nothing
// at all, from a done task, as reopened: that is how a client unticks one.
// Any other absence leaves the state alone.
func davTarget(current core.StateCategory, v *ical.TodoValues) (core.StateCategory, bool) {
	status := v.Status
	switch {
	case status != "":
	case v.Completed || v.PercentComplete == 100:
		status = ical.StatusCompleted
	case current == core.CategoryDone:
		status = ical.StatusNeedsAction
	default:
		return "", false
	}
	rendered, ok := davStatusOf[current]
	if !ok {
		rendered = ical.StatusNeedsAction
	}
	if status == rendered {
		return "", false
	}
	return davCategoryOf[status], true
}

// davRoute is the one shortest route from a state to a state of a category.
// None, or two of the same length to one state or to two, is refused rather
// than guessed: every state passed through is written to the task's history.
func davRoute(def core.WorkflowDefinition, from string, target core.StateCategory) ([]string, error) {
	var found []core.Route
	for _, r := range core.Routes(def, from) {
		if def.CategoryOf(r.To.Key) != target {
			continue
		}
		switch {
		case len(found) == 0 || r.Hops() < found[0].Hops():
			found = []core.Route{r}
		case r.Hops() == found[0].Hops():
			found = append(found, r)
		}
	}
	switch len(found) {
	case 0:
		return nil, core.DAVRefusal(core.ReasonDAVRefused, "no %s state is reachable from %q in this workflow", target, from)
	case 1:
		return found[0].Keys(), nil
	}
	return nil, core.DAVRefusal(core.ReasonDAVRefused,
		"two routes of %d steps lead from %q to a %s state, %s and %s; move the task in tix instead",
		found[0].Hops(), from, target, found[0].Value(), found[1].Value())
}

// davUpdate is the update the client's object asks of a task: each field
// whose value differs from the task's. An absent SUMMARY, DESCRIPTION,
// PRIORITY or CATEGORIES leaves its field alone, because Apple Reminders
// rewrites whole objects without the properties it does not show; an absent
// DUE clears the date, because every client removes the property to remove
// one.
func davUpdate(t core.Task, v *ical.TodoValues) (core.UpdateTaskInput, bool) {
	var in core.UpdateTaskInput
	changed := false
	if v.Summary != nil {
		if title := strings.TrimSpace(*v.Summary); title != "" && title != t.Title {
			in.Title, changed = &title, true
		}
	}
	if v.Description != nil && *v.Description != t.Body {
		body := *v.Description
		in.Body, changed = &body, true
	}
	if v.Priority != nil {
		if p := ical.PriorityFromTodo(*v.Priority); p != t.Priority {
			in.Priority, changed = &p, true
		}
	}
	if v.HasCategories && !davSameTags(v.Categories, t.Tags) {
		tags := slices.Clone(v.Categories)
		if tags == nil {
			tags = []string{}
		}
		in.Tags, changed = &tags, true
	}
	switch {
	case v.Due == nil && t.DueAt != nil:
		in.DueAt, changed = new(*time.Time), true
	case v.Due != nil && (t.DueAt == nil || !t.DueAt.Equal(v.Due.At)):
		due := v.Due.At.UTC()
		dp := &due
		in.DueAt, changed = &dp, true
	}
	return in, changed
}

// davSameTags compares two tag sets without regard to order or case.
func davSameTags(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		if !containsFold(b, x) {
			return false
		}
	}
	return true
}

func containsFold(list []string, s string) bool {
	return slices.ContainsFunc(list, func(x string) bool { return strings.EqualFold(x, s) })
}

// davSnooze is the snooze a DTSTART asks for: one in the future that is not
// what tix already shows. A past or absent one changes nothing, because
// Thunderbird writes the creation time there.
func davSnooze(t core.Task, v *ical.TodoValues, now time.Time) (time.Time, bool) {
	if v.Start == nil || !v.Start.At.After(now) {
		return time.Time{}, false
	}
	if shown, ok := ical.TodoStart(t, now); ok && shown.Equal(v.Start.At) {
		return time.Time{}, false
	}
	return v.Start.At, true
}

// davWriteError reads an ordinary operation's refusal in CalDAV's terms: a
// stale version is 412, anything the task's rules refuse is 403 naming why.
func davWriteError(err error) error {
	switch {
	case core.DAVReason(err) != "":
		return err
	case core.IsKind(err, core.KindConflict):
		return core.DAVRefusal(core.ReasonDAVStale, "the task changed while the write ran: %s", err.Error())
	case core.IsKind(err, core.KindForbidden), core.IsKind(err, core.KindPrecondition),
		core.IsKind(err, core.KindInvalid), core.IsKind(err, core.KindLeaseExpired):
		return core.DAVRefusal(core.ReasonDAVRefused, "%s", err.Error())
	}
	return err
}

// create makes a task from a client's object at a name the collection does
// not hold: in the collection's project, or for Assigned to me in the
// tenant's default project, assigned to the owner. The client's UID and name
// are kept so the task is found where the client put it.
func (w *davWriter) create(ctx context.Context, in core.DAVPutInput) (*core.DAVPutResult, error) {
	if in.IfMatch != "" {
		return nil, core.DAVRefusal(core.ReasonDAVStale, "the object does not exist")
	}
	v := w.v
	if utf8.RuneCountInString(in.Name) > maxDAVName || len(v.UID) > maxDAVName ||
		strings.ContainsFunc(in.Name+v.UID, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return nil, core.DAVRefusal(core.ReasonDAVInvalidObject, "a resource name and a UID are at most %d printable characters", maxDAVName)
	}
	if v.Summary == nil || strings.TrimSpace(*v.Summary) == "" {
		return nil, core.DAVRefusal(core.ReasonDAVInvalidObject, "a new task needs a SUMMARY")
	}
	if err := w.l.davFree(ctx, w.s, in.Name, v.UID); err != nil {
		return nil, err
	}
	create := core.CreateTaskInput{Title: strings.TrimSpace(*v.Summary), ProjectRef: w.c.ProjectID}
	if w.c.ProjectID == "" {
		create.AssigneeActorID = w.s.reader.ID
	}
	if v.Description != nil {
		create.Body = *v.Description
	}
	if v.Priority != nil {
		create.Priority = ical.PriorityFromTodo(*v.Priority)
	}
	create.Tags = v.Categories
	if v.Due != nil {
		due := v.Due.At.UTC()
		create.DueAt = &due
	}
	task, err := w.l.CreateTask(w.as, create)
	if err != nil && w.c.ProjectID == "" && davOnField(err, "project_ref") {
		return nil, core.DAVRefusal(core.ReasonDAVRefused, "a task added to Assigned to me goes to the tenant's "+
			"%q project, and this tenant has several projects and none of that key; add it to a project's list instead",
			DefaultProjectKey)
	}
	if err != nil {
		return nil, davWriteError(err)
	}
	err = w.l.store.Update(ctx, core.TenantScope{TenantID: w.s.reader.TenantID}, func(tx store.Tx) error {
		return tx.CreateDAVObject(ctx, &store.DAVObjectName{TaskID: task.ID, UID: v.UID, Name: in.Name, CreatedAt: w.now})
	})
	if core.IsKind(err, core.KindConflict) {
		return nil, core.DAVRefusal(core.ReasonDAVUIDConflict, "another task took this UID or name meanwhile")
	}
	if err != nil {
		return nil, err
	}
	out := &core.DAVPutResult{Created: true, Task: task}
	if e, err := w.entry(ctx, in.Name, in.Origin); err == nil && e != nil {
		if moved, err := w.apply(e); err == nil {
			out.Task = moved
		}
	}
	if err := w.keep(ctx, task.ID, nil); err != nil {
		return nil, err
	}
	out.KeptDropped = w.dropped()
	return out, nil
}

// davOnField reports whether err names a field.
func davOnField(err error, field string) bool {
	var e *core.Error
	return errors.As(err, &e) && e.Details[core.DetailField] == field
}

// davFree refuses a UID or a name already in use in the tenant: a client's
// for another task, or a task's own.
func (l *Local) davFree(ctx context.Context, s *davSession, name, uid string) error {
	taken := core.DAVRefusal(core.ReasonDAVUIDConflict, "another task already has this UID or resource name")
	return l.store.View(ctx, core.TenantScope{TenantID: s.reader.TenantID}, func(tx store.Tx) error {
		for _, find := range []func() (*store.DAVObjectName, error){
			func() (*store.DAVObjectName, error) { return tx.GetDAVObjectByUID(ctx, uid) },
			func() (*store.DAVObjectName, error) { return tx.GetDAVObjectByName(ctx, name) },
		} {
			if _, err := find(); err == nil {
				return taken
			} else if !core.IsKind(err, core.KindNotFound) {
				return err
			}
		}
		ids := []string{strings.ToUpper(name)}
		if id, ok := strings.CutSuffix(strings.TrimPrefix(uid, "task-"), "@tix"); ok && strings.HasPrefix(uid, "task-") {
			ids = append(ids, strings.ToUpper(id))
		}
		for _, id := range ids {
			if _, err := tx.GetTask(ctx, core.TaskRef{ID: id}); err == nil {
				return taken
			} else if !core.IsKind(err, core.KindNotFound) && !core.IsKind(err, core.KindInvalid) {
				return err
			}
		}
		return nil
	})
}

// keep stores what the client sent that tix has no field for, for this
// person and task, when it differs from what is kept. Over MaxDAVKeptBytes
// nothing is kept. It is the client's own copy, never read by tix and never
// shown to anybody else, so like a sync state it records no audit entry and
// no event; it spends the person's sync short cut instead, so the changed
// entity tag reaches their next sync.
func (w *davWriter) keep(ctx context.Context, taskID string, before []string) error {
	lines := w.v.Kept
	if len(strings.Join(lines, davKeptSep)) > MaxDAVKeptBytes {
		lines = nil
	}
	if slices.Equal(lines, before) {
		return nil
	}
	return w.l.store.Update(ctx, core.TenantScope{TenantID: w.s.reader.TenantID}, func(tx store.Tx) error {
		if err := tx.PutDAVProps(ctx, w.s.reader.ID, taskID, strings.Join(lines, davKeptSep), w.now); err != nil {
			return err
		}
		return tx.InvalidateDAVSyncStates(ctx, w.s.reader.ID)
	})
}

// dropped reports that the extras were over the bound and not kept.
func (w *davWriter) dropped() bool {
	return len(strings.Join(w.v.Kept, davKeptSep)) > MaxDAVKeptBytes
}

// DeleteDAVObject cancels an open task and leaves a done one alone.
func (l *Local) DeleteDAVObject(ctx context.Context, in core.DAVDeleteInput) error {
	w, err := l.davWriterOf(ctx, in.Collection, nil)
	if err != nil {
		return err
	}
	e, err := w.entry(ctx, in.Name, in.Origin)
	if err != nil {
		return err
	}
	if e == nil {
		return core.NotFound("no such object")
	}
	if in.IfMatch != "" && !davETagMatches(in.IfMatch, e.member.ETag) {
		return core.DAVRefusal(core.ReasonDAVStale, "the object changed since the client read it")
	}
	if e.category == core.CategoryDone {
		return nil
	}
	route, err := davRoute(e.def, e.task.Status, core.CategoryCancelled)
	if err != nil {
		return err
	}
	r, err := l.TransitionRoute(w.as, core.TaskRef{ID: e.task.ID}, core.RouteInput{Route: route, Version: e.task.Version,
		DryRun: true})
	if err != nil {
		return davWriteError(err)
	}
	if r.Partial() {
		return core.DAVRefusal(core.ReasonDAVRefused, "%s", r.Sentence())
	}
	r, err = l.TransitionRoute(w.as, core.TaskRef{ID: e.task.ID}, core.RouteInput{Route: route, Version: e.task.Version})
	if err != nil {
		return davWriteError(err)
	}
	if r.Partial() {
		return core.DAVRefusal(core.ReasonDAVRefused, "%s", r.Sentence())
	}
	return nil
}
