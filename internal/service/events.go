// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"maps"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Poll limits.
const (
	DefaultPollLimit = 200
	MaxPollLimit     = 1000
	MaxPollWait      = 25 * time.Second
)

// pollScanBudget bounds how many events one poll reads, so a narrow filter
// over a busy tenant cannot turn one request into a read of the whole log, and
// pollBatch is how many it reads at a time. They are variables so a test can
// reach the budget without writing thousands of events.
var (
	pollScanBudget = 5000
	pollBatch      = 256
)

// subjectScope names, for each event subject that describes an administrative
// object, the scope that lets a subscriber read that object over REST.
//
// event:subscribe on its own used to be enough for every event in the tenant,
// which is what the spec said and what the code did. The effect was that a
// token holding only event:subscribe received user.created carrying an email
// address, token.created carrying the name and scope list of every credential
// minted, and session.created naming who logged in and when: a directory of
// the tenant's people and of which key can do what, assembled by a credential
// that is refused all three over REST. None of it is a secret in the sense of
// being usable on its own, which is why it survived review for so long, and
// all of it is exactly what you would want before attacking the tenant.
//
// A subject absent from this map is ordinary work -- tasks, comments,
// projects, workflows -- and event:subscribe remains sufficient for it, which
// is the whole point of the scope. TestEveryEventSubjectIsClassified refuses a
// new subject that appears in neither this map nor the work list, so the next
// administrative object cannot be added to the stream by omission.
var subjectScope = map[string]core.Scope{
	"api_token": core.ScopeTokenAdmin,
	// A calendar feed is a credential, and its events say whose feed changed
	// and what it selects.
	"calendar_feed": core.ScopeTokenAdmin,
	"ssh_key":       core.ScopeTokenAdmin,
	"user":          core.ScopeUserAdmin,
	"membership":    core.ScopeUserAdmin,
	// An invite names an address and the role it was offered.
	"user_invite": core.ScopeUserAdmin,
	"session":     core.ScopeUserAdmin,

	// A second-factor event names a user and says whether that account holds
	// one, which is a list of who to attack first. The scope is the one that
	// clears somebody else's enrolment, since that authority already carries
	// knowing who has one.
	"totp": core.ScopeTOTPAdmin,

	"tenant":     core.ScopeTenantAdmin,
	"domain":     core.ScopeTenantAdmin,
	"connection": core.ScopeTenantAdmin,

	// webhook.put carries the endpoint URL, so an ungated subscriber learns
	// the internal hostnames this tenant delivers to.
	"webhook":          core.ScopeWebhookAdmin,
	"webhook_delivery": core.ScopeWebhookAdmin,
	"sync_source":      core.ScopeSyncAdmin,
}

// EventSubjectScopes returns a copy of the extra scope each administrative event subject requires.
func EventSubjectScopes() map[string]core.Scope { return maps.Clone(subjectScope) }

// MayReceiveEvent reports whether a credential may see an event on any event transport.
func MayReceiveEvent(a *core.Actor, e core.Event) bool {
	if a == nil || a.TenantID == "" || a.TenantID != e.TenantID {
		return false
	}
	if !a.HasScope(core.ScopeEventSubscribe) {
		return false
	}
	if need, ok := subjectScope[e.SubjectType]; ok && !a.HasScope(need) {
		return false
	}
	return !a.ScopedToProject() || a.ProjectID == e.ProjectID
}

// knownEventTypes is the closed set a filter may name. It is derived from the
// contract package, never copied: a copy would refuse a filter naming an event
// type the outbox already emits.
var knownEventTypes = core.EventTypes()

// ValidateEventTypes rejects a filter naming a type outside the closed set.
func ValidateEventTypes(types []core.EventType) error {
	for _, t := range types {
		if !knownEventType(t) {
			return core.Invalid("unknown event type %q", t)
		}
	}
	return nil
}

// knownEventType reports whether a literal or wildcard pattern can ever match.
func knownEventType(pattern core.EventType) bool {
	if pattern == "*" {
		return true
	}
	prefix, wildcard := strings.CutSuffix(string(pattern), "*")
	for _, known := range knownEventTypes {
		if wildcard && strings.HasPrefix(string(known), prefix) {
			return true
		}
		if !wildcard && known == pattern {
			return true
		}
	}
	return false
}

// NarrowEventProjects confines a project-scoped credential to its project and
// refuses one that names another.
func NarrowEventProjects(a *core.Actor, projectIDs []string) ([]string, error) {
	if !a.ScopedToProject() {
		return projectIDs, nil
	}
	for _, id := range projectIDs {
		if id == a.ProjectID {
			return []string{a.ProjectID}, nil
		}
	}
	if len(projectIDs) > 0 {
		return nil, core.Forbidden("this credential may only subscribe to project %q", a.ProjectID)
	}
	return []string{a.ProjectID}, nil
}

// CheckCursorRetained refuses a resume cursor older than the oldest retained
// event. Every event transport answers with this one error, so a client
// handles an expired cursor the same way whichever it used.
func CheckCursorRetained(sinceSeq, oldest int64) error {
	if sinceSeq > 0 && oldest > 0 && sinceSeq+1 < oldest {
		return core.Invalid("cursor %d is no longer available; the oldest retained event is %d", sinceSeq, oldest)
	}
	return nil
}

// PollEvents returns the committed events after since_seq that match the
// filter, waiting for one when there are none and wait allows.
func (l *Local) PollEvents(ctx context.Context, in core.EventPollInput) (*core.EventPage, error) {
	actor, err := l.authorize(ctx, authz.ActionEventSubscribe, authz.Resource{})
	if err != nil {
		return nil, err
	}
	filter, limit, err := pollFilter(actor, in)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	wake := l.commitSignal(ctx, actor.TenantID, in.Wait.D())

	reader := &eventReader{local: l, actor: actor}
	if filter.SinceSeq == 0 {
		if filter.SinceSeq, err = reader.Latest(ctx); err != nil {
			return nil, err
		}
	} else if err := reader.checkRetained(ctx, filter.SinceSeq); err != nil {
		return nil, err
	}

	page := &core.EventPage{Events: []core.Event{}, NextSeq: filter.SinceSeq}
	var deadline <-chan time.Time
	for {
		caughtUp, err := l.pollOnce(ctx, reader, actor, filter, limit, page)
		if err != nil {
			return nil, err
		}
		if len(page.Events) > 0 || !caughtUp || wake == nil {
			return page, nil
		}
		if deadline == nil {
			deadline = l.clock.After(in.Wait.D())
		}
		select {
		case <-wake:
		case <-deadline:
			return page, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// pollFilter validates a poll and narrows it to what the credential may see.
func pollFilter(actor *core.Actor, in core.EventPollInput) (core.EventFilter, int, error) {
	if in.SinceSeq < 0 {
		return core.EventFilter{}, 0, core.Invalid("since_seq must not be negative")
	}
	if in.Wait < 0 || in.Wait.D() > MaxPollWait {
		return core.EventFilter{}, 0, core.Invalid("wait must be between 0 and %s", MaxPollWait)
	}
	limit := in.Limit
	switch {
	case limit == 0:
		limit = DefaultPollLimit
	case limit < 0 || limit > MaxPollLimit:
		return core.EventFilter{}, 0, core.Invalid("limit must be between 1 and %d", MaxPollLimit)
	}
	if err := ValidateEventTypes(in.Types); err != nil {
		return core.EventFilter{}, 0, err
	}
	projects, err := NarrowEventProjects(actor, in.ProjectIDs)
	if err != nil {
		return core.EventFilter{}, 0, err
	}
	return core.EventFilter{ProjectIDs: projects, Types: in.Types, SinceSeq: in.SinceSeq}, limit, nil
}

// commitSignal registers for the tenant's commit signal before the first read,
// so a commit landing between that read and the wait still wakes the poll. It
// is nil when the poll does not wait or the store cannot signal.
func (l *Local) commitSignal(ctx context.Context, tenantID string, wait time.Duration) <-chan struct{} {
	if wait <= 0 {
		return nil
	}
	w, ok := l.store.(store.CommitWaker)
	if !ok {
		return nil
	}
	ch, err := w.WakeOnCommit(ctx, tenantID)
	if err != nil {
		return nil
	}
	return ch
}

// pollOnce reads forward from page.NextSeq, appending what matches, and
// reports whether it reached the newest committed event.
func (l *Local) pollOnce(ctx context.Context, reader *eventReader, actor *core.Actor, f core.EventFilter, limit int, page *core.EventPage) (bool, error) {
	scanned := 0
	for scanned < pollScanBudget {
		events, err := reader.ReadSince(ctx, page.NextSeq, pollBatch)
		if err != nil {
			return false, err
		}
		for i := range events {
			scanned++
			page.NextSeq = events[i].Seq
			if !MayReceiveEvent(actor, events[i]) || !f.Matches(events[i]) {
				continue
			}
			page.Events = append(page.Events, events[i])
			if len(page.Events) >= limit {
				return true, nil
			}
		}
		if len(events) < pollBatch {
			return true, nil
		}
	}
	return false, nil
}
