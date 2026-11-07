// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"hash/fnv"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// placeholderPrefix starts the handle of every actor an import creates for a
// person it could not match. Real handles come from email local parts or are
// typed by hand, so the prefix keeps the two apart.
const placeholderPrefix = "ext-"

// maxPlaceholderHandle bounds a placeholder handle, prefix included.
const maxPlaceholderHandle = 64

// maxUnmatchedNamed is how many unmatched people the summary warning names
// before it only counts the rest; the result field lists every one.
const maxUnmatchedNamed = 20

// auditSyncActorCreate records a placeholder actor an import created.
const auditSyncActorCreate = "sync.actor.create"

// syncUsers resolves source assignees to tix actors for one run. The tenant's
// directory is read once, on the first assignee, and every answer is kept, so
// a large import costs one directory read and not a query per record.
type syncUsers struct {
	loaded       bool
	handles      map[string]string
	matchHandles map[string]string
	emails       map[string]string
	names        map[string][]string
	answers      map[string]string
	placeholders map[string]string
	unmatched    map[string]bool
}

// load reads the tenant's actors and the accounts behind them. Every read goes
// through the tenant-scoped transaction, so no other tenant's people can match.
func (u *syncUsers) load(ctx context.Context, tx store.Tx) error {
	if u.loaded {
		return nil
	}
	u.handles, u.matchHandles = map[string]string{}, map[string]string{}
	u.emails, u.names = map[string]string{}, map[string][]string{}
	u.answers, u.placeholders = map[string]string{}, map[string]string{}
	page := core.Page{Limit: core.MaxPageLimit, Sort: actorSort}
	for {
		actors, err := tx.ListActors(ctx, page)
		if err != nil {
			return err
		}
		for _, a := range actors {
			if err := u.add(ctx, tx, a); err != nil {
				return err
			}
		}
		if len(actors) < page.Limit {
			break
		}
		page.Cursor = actorCursor(page, actors[len(actors)-1]).Encode()
	}
	u.loaded = true
	return nil
}

func (u *syncUsers) add(ctx context.Context, tx store.Tx, a core.Actor) error {
	handle := strings.ToLower(a.Handle)
	u.handles[handle] = a.ID
	var account *core.User
	if a.Kind == core.ActorUser {
		found, err := accountOf(ctx, tx, &a)
		switch {
		case err == nil:
			account = found
		case !core.IsKind(err, core.KindNotFound):
			return err
		}
	}
	if account == nil && strings.HasPrefix(handle, placeholderPrefix) {
		u.placeholders[handle] = a.ID
		return nil
	}
	u.matchHandles[handle] = a.ID
	names := []string{a.DisplayName}
	if account != nil {
		u.emails[strings.ToLower(account.Email)] = a.ID
		names = append(names, account.DisplayName)
	}
	seen := map[string]bool{}
	for _, n := range names {
		key := strings.ToLower(strings.TrimSpace(n))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		u.names[key] = append(u.names[key], a.ID)
	}
	return nil
}

// match applies the explicit map and then the automatic strategies, returning
// the actor identifier or "" when nothing matched.
func (u *syncUsers) match(rules extsync.Users, value, email string, warn func(string)) string {
	key := value + "\x00" + strings.ToLower(email)
	if id, ok := u.answers[key]; ok {
		return id
	}
	id := u.explicit(rules, value, warn)
	for _, strategy := range rules.Strategies() {
		if id != "" {
			break
		}
		id = u.byStrategy(strategy, value, email)
	}
	u.answers[key] = id
	return id
}

func (u *syncUsers) explicit(rules extsync.Users, value string, warn func(string)) string {
	target, ok := rules.Map[value]
	if !ok {
		return ""
	}
	target = strings.ToLower(strings.TrimSpace(target))
	if id, ok := u.emails[target]; ok && strings.Contains(target, "@") {
		return id
	}
	if id, ok := u.handles[target]; ok {
		return id
	}
	warn("users.map." + value + " names " + strconv.Quote(target) +
		", which is no tix actor; matching it automatically instead")
	return ""
}

func (u *syncUsers) byStrategy(strategy, value, email string) string {
	needle := strings.ToLower(strings.TrimSpace(value))
	switch strategy {
	case extsync.MatchEmail:
		if id, ok := u.emails[strings.ToLower(strings.TrimSpace(email))]; ok && email != "" {
			return id
		}
		return u.emails[needle]
	case extsync.MatchHandle:
		return u.matchHandles[needle]
	case extsync.MatchName:
		if ids := u.names[needle]; len(ids) == 1 {
			return ids[0]
		}
	}
	return ""
}

// noteUnmatched remembers a value no rule matched, for the run's report.
func (u *syncUsers) noteUnmatched(value string) {
	if u.unmatched == nil {
		u.unmatched = map[string]bool{}
	}
	u.unmatched[value] = true
}

// matchAssignee resolves a record's assignee without writing anything, and
// records it as unmatched when nothing claims it.
func (r *syncRun) matchAssignee(ctx context.Context, tx store.Tx, mapped extsync.Mapped) (string, error) {
	if err := r.users.load(ctx, tx); err != nil {
		return "", err
	}
	id := r.users.match(r.mapping.Users, mapped.Assignee, mapped.AssigneeEmail, r.warn)
	if id == "" {
		r.users.noteUnmatched(mapped.Assignee)
	}
	return id, nil
}

// placeholderFor returns the placeholder actor standing in for an unmatched
// person, creating it on first sight. An actor holding the handle with an
// account is a real person and is never taken over.
func (r *syncRun) placeholderFor(ctx context.Context, m *mutation, value string) (string, error) {
	handle := placeholderHandle(value)
	if id, ok := r.users.placeholders[handle]; ok {
		return id, nil
	}
	if _, taken := r.users.handles[handle]; taken {
		r.warn("assignee " + value + " needs placeholder handle " + handle +
			", which a tix account holds; left unassigned")
		return "", nil
	}
	actor := core.Actor{Kind: core.ActorUser, Handle: handle, DisplayName: strings.TrimSpace(value)}
	if err := m.tx.CreateActor(ctx, &actor); err != nil {
		return "", err
	}
	if err := m.Audit(auditSyncActorCreate, "actor", actor.ID, nil, actor); err != nil {
		return "", err
	}
	r.users.placeholders[handle] = actor.ID
	r.users.handles[handle] = actor.ID
	return actor.ID, nil
}

// placeholderHandle derives a stable handle from an external value, so every
// run and every source names the same person the same way.
func placeholderHandle(value string) string {
	var b strings.Builder
	dash := false
	for _, c := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(c) || unicode.IsDigit(c) {
			b.WriteRune(c)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if slug == "" {
		h := fnv.New32a()
		_, _ = h.Write([]byte(value))
		slug = strconv.FormatUint(uint64(h.Sum32()), 16)
	}
	handle := placeholderPrefix + slug
	if len(handle) > maxPlaceholderHandle {
		handle = strings.TrimRight(strings.ToValidUTF8(handle[:maxPlaceholderHandle], ""), "-")
	}
	return handle
}

// reportUnmatched lists every person no rule matched, in the result and in one
// warning, which is added past the warning cap because it is the summary.
func (r *syncRun) reportUnmatched() {
	if len(r.users.unmatched) == 0 {
		return
	}
	names := make([]string, 0, len(r.users.unmatched))
	for n := range r.users.unmatched {
		names = append(names, n)
	}
	sort.Strings(names)
	r.result.UnmatchedUsers = names
	r.result.Warnings = append(r.result.Warnings, unmatchedWarning(names, r.dryRun, r.mapping.Users.Placeholders()))
}

func unmatchedWarning(names []string, dryRun, placeholders bool) string {
	subject, have, was := "assignees", "have", "were"
	if len(names) == 1 {
		subject, have, was = "assignee", "has", "was"
	}
	outcome := was + " left unassigned"
	if placeholders {
		outcome = "got placeholder actors"
	}
	if dryRun {
		outcome = "would be left unassigned"
		if placeholders {
			outcome = "would get placeholder actors"
		}
	}
	listed := names
	more := ""
	if len(listed) > maxUnmatchedNamed {
		listed = listed[:maxUnmatchedNamed]
		more = " and " + strconv.Itoa(len(names)-maxUnmatchedNamed) + " more"
	}
	return strconv.Itoa(len(names)) + " " + subject + " " + have + " no tix actor and " + outcome + ": " +
		strings.Join(listed, ", ") + more
}

// hrefTail returns the last path segment of a link, or "" when the value is
// not an absolute path or an http(s) url.
func hrefTail(value string) string {
	p := value
	if !strings.HasPrefix(value, "/") {
		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return ""
		}
		p = u.Path
	}
	tail := path.Base(strings.TrimRight(p, "/"))
	if tail == "/" || tail == "." || tail == value {
		return ""
	}
	return tail
}
