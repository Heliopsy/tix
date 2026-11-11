// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// SortDirection orders a listing.
type SortDirection string

// Sort directions.
const (
	Ascending  SortDirection = "asc"
	Descending SortDirection = "desc"
)

// Valid reports whether d is a known direction.
func (d SortDirection) Valid() bool { return d == Ascending || d == Descending }

// Cursor is an opaque position in a keyset-paginated listing. SortValue2 is
// additive: it carries the second key of a compound ordering (currently only
// SortUrgency) and is empty for every single-key ordering, so an older
// encoded cursor decodes unchanged. Starred is the leading group of a task
// listing read for an actor, and is equally additive: a cursor without it
// resumes among the unstarred tasks, which is where every cursor issued
// before stars existed already was.
type Cursor struct {
	SortValue  string        `json:"v" yaml:"v"`
	SortValue2 string        `json:"v2,omitempty" yaml:"v2,omitempty"`
	Starred    bool          `json:"st,omitempty" yaml:"st,omitempty"`
	ID         string        `json:"i" yaml:"i"`
	Sort       string        `json:"s" yaml:"s"`
	Direction  SortDirection `json:"d" yaml:"d"`
}

// Zero reports whether the cursor addresses nothing, meaning the first page.
func (c Cursor) Zero() bool { return c.ID == "" && c.SortValue == "" }

// Encode renders the cursor as an opaque token.
func (c Cursor) Encode() string {
	if c.Zero() {
		return ""
	}
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor parses a cursor token.
func DecodeCursor(s string) (Cursor, error) {
	if strings.TrimSpace(s) == "" {
		return Cursor{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, Invalid("cursor is not a valid token")
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return Cursor{}, Invalid("cursor is not a valid token")
	}
	if c.Zero() {
		return Cursor{}, Invalid("cursor is empty")
	}
	return c, nil
}

// CheckOrdering rejects a cursor produced for a different ordering.
func (c Cursor) CheckOrdering(sort string, dir SortDirection) error {
	if c.Zero() {
		return nil
	}
	if c.Sort != sort || c.Direction != dir {
		return Invalid("cursor was produced for a different ordering; restart the listing")
	}
	return nil
}

// Page bounds one page of a listing.
type Page struct {
	// Limit is the most items one page holds: default 50, at most 500.
	Limit int `json:"limit,omitempty" yaml:"limit,omitempty"`
	// Cursor is the next_cursor of the previous page; empty reads the first.
	Cursor string `json:"cursor,omitempty" yaml:"cursor,omitempty"`
	// Sort names the field the listing is ordered by; each listing has its
	// own default and refuses a field it cannot sort by.
	Sort string `json:"sort,omitempty" yaml:"sort,omitempty"`
	// Direction is asc or desc, default asc.
	Direction SortDirection `json:"direction,omitempty" yaml:"direction,omitempty"`
}

// Page limits.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 500

	// DefaultDisplayLimit is how many rows a surface shows a person when
	// nothing configures its page size. It is smaller than DefaultPageLimit,
	// which stays the fallback for a caller that names no limit at all: a
	// reader looks at a screen, an API client follows a cursor.
	DefaultDisplayLimit = 25
)

// Normalize clamps the page to usable values.
func (p Page) Normalize() (Page, error) {
	if p.Limit < 0 {
		return Page{}, Invalid("limit must not be negative")
	}
	if p.Limit == 0 {
		p.Limit = DefaultPageLimit
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}
	if p.Direction == "" {
		p.Direction = Ascending
	}
	if !p.Direction.Valid() {
		return Page{}, Invalid("sort direction %q must be %q or %q", p.Direction, Ascending, Descending)
	}
	return p, nil
}

// TaskPage is one page of tasks plus the cursor for the next.
type TaskPage struct {
	Tasks      []Task `json:"tasks" yaml:"tasks"`
	NextCursor string `json:"next_cursor,omitempty" yaml:"next_cursor,omitempty"`
	// Total is how many tasks the filter selects, counted up to TaskCountCap.
	Total int `json:"total" yaml:"total"`
	// TotalCapped is true when the count stopped at TaskCountCap, so Total is
	// a floor rather than the whole.
	TotalCapped bool `json:"total_capped,omitempty" yaml:"total_capped,omitempty"`
	// Number is the page answered, from one, when the filter asked by
	// PageNumber; a number past the end is answered with the last page.
	Number int `json:"page,omitempty" yaml:"page,omitempty"`
	// Tally counts the whole selection, every page of it, when the filter
	// asked with Tally. It is for in-process callers and does not travel.
	Tally *TaskTally `json:"-" yaml:"-"`
}

// TaskTally counts every task a filter selects, not one page of them.
type TaskTally struct {
	// States counts the selection per project and status.
	States []StateCount
	// Blocked counts the tasks waiting on an unfinished dependency.
	Blocked int
	// Held counts the tasks under a live claim.
	Held int
}

// StateCount is how many selected tasks of one project sit in one status.
type StateCount struct {
	ProjectID string
	Status    string
	Count     int
}

// TaskCountCap is where a task listing's count stops. Past it a total is
// reported as a floor, and a page number cannot reach further than it.
const TaskCountCap = 10000

// PagesOf is how many pages of size a listing of total rows fills, never
// fewer than one.
func PagesOf(total, size int) int {
	if size < 1 || total <= size {
		return 1
	}
	return (total + size - 1) / size
}

// TriState expresses an optional boolean filter, distinguishing "not filtered"
type TriState uint8

// TriState values.
const (
	Either TriState = iota
	Yes
	No
)

// Match reports whether v satisfies the tri-state.
func (t TriState) Match(v bool) bool {
	switch t {
	case Yes:
		return v
	case No:
		return !v
	default:
		return true
	}
}

// TaskFilter selects tasks.
type TaskFilter struct {
	ProjectIDs  []string `json:"project_ids,omitempty" yaml:"project_ids,omitempty"`
	ProjectKeys []string `json:"project_keys,omitempty" yaml:"project_keys,omitempty"`
	Statuses    []string `json:"statuses,omitempty" yaml:"statuses,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	AssigneeIDs []string `json:"assignee_ids,omitempty" yaml:"assignee_ids,omitempty"`
	CreatorIDs  []string `json:"creator_ids,omitempty" yaml:"creator_ids,omitempty"`

	Priorities []Priority `json:"priorities,omitempty" yaml:"priorities,omitempty"`

	DueBefore *time.Time `json:"due_before,omitempty" yaml:"due_before,omitempty"`
	DueAfter  *time.Time `json:"due_after,omitempty" yaml:"due_after,omitempty"`

	ParentID     string `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	ParentIsNull bool   `json:"parent_is_null,omitempty" yaml:"parent_is_null,omitempty"`

	Claimed TriState `json:"claimed,omitempty" yaml:"claimed,omitempty"`
	Blocked TriState `json:"blocked,omitempty" yaml:"blocked,omitempty"`
	// Recurring selects tasks by whether they carry a repeat rule.
	Recurring TriState `json:"recurring,omitempty" yaml:"recurring,omitempty"`

	ClaimedBy []string `json:"claimed_by,omitempty" yaml:"claimed_by,omitempty"`

	// Starred selects by the reading actor's own stars.
	Starred TriState `json:"starred,omitempty" yaml:"starred,omitempty"`

	// Checklist selects tasks by whether the description carries a checklist
	// marker, and ChecklistOpen by whether it carries an unticked one. Both
	// read the text for ChecklistMarkers, not the parsed Markdown.
	Checklist     TriState `json:"checklist,omitempty" yaml:"checklist,omitempty"`
	ChecklistOpen TriState `json:"checklist_open,omitempty" yaml:"checklist_open,omitempty"`

	// StarredBy is the reading actor, set by the service from the caller and
	// never by the caller, which is why it does not travel on the wire. A
	// listing with no reader neither groups nor filters by stars.
	StarredBy string `json:"-" yaml:"-"`

	// OmitBody leaves each task's Body and Checklist empty, for an in-process
	// caller that only counts the tasks: a description can be a megabyte. It
	// does not travel on the wire, so a remote listing always carries bodies.
	OmitBody bool `json:"-" yaml:"-"`

	// Tally asks for TaskPage.Tally, the counts of the whole selection. Like
	// OmitBody it is for an in-process caller and does not travel.
	Tally bool `json:"-" yaml:"-"`

	Query string `json:"query,omitempty" yaml:"query,omitempty"`

	// Text holds the explicit title and body predicates: exact equality, or
	// the weak substring match. Unlike Query, which each engine answers with
	// its own native search, every engine answers these the same way.
	Text []TextTerm `json:"text,omitempty" yaml:"text,omitempty"`

	// Exclude removes tasks the inclusion terms would otherwise have kept.
	Exclude TaskExclude `json:"exclude,omitempty" yaml:"exclude,omitempty"`

	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`

	IncludeDeleted bool `json:"include_deleted,omitempty" yaml:"include_deleted,omitempty"`

	// Trashed selects by soft deletion: Yes is the trash alone, No the live
	// tasks alone. When set it decides, whatever IncludeDeleted says.
	Trashed TriState `json:"trashed,omitempty" yaml:"trashed,omitempty"`
	// Snoozed selects by whether a task is snoozed now. IncludeSnoozed widens
	// a listing that names no snooze to hold snoozed tasks too; without
	// either, Service.ListTasks leaves snoozed tasks out.
	Snoozed        TriState `json:"snoozed,omitempty" yaml:"snoozed,omitempty"`
	IncludeSnoozed bool     `json:"include_snoozed,omitempty" yaml:"include_snoozed,omitempty"`

	Page Page `json:"page,omitempty" yaml:"page,omitempty"`

	// PageNumber reads the listing from page N, counting from one, instead of
	// from Page.Cursor. The service seeks the cursor that page starts at, so
	// the page is still keyset-addressed. Zero reads from the cursor.
	PageNumber int `json:"page_number,omitempty" yaml:"page_number,omitempty"`
}

// ListsDeleted reports whether the filter admits soft-deleted tasks.
func (f TaskFilter) ListsDeleted() bool {
	switch f.Trashed {
	case Yes:
		return true
	case No:
		return false
	default:
		return f.IncludeDeleted
	}
}

// ListsLive reports whether the filter admits tasks that are not deleted.
func (f TaskFilter) ListsLive() bool { return f.Trashed != Yes }

// Task sort fields.
const (
	SortCreatedAt = "created_at"
	SortUpdatedAt = "updated_at"
	SortPriority  = "priority"
	SortDueAt     = "due_at"
	SortSeq       = "seq"
	SortTitle     = "title"

	// SortUrgency is the compound ordering: priority first (PriorityHighest
	// leads), then due date within a priority (soonest leads, and a task with
	// no due date sorts as if its deadline were the distant future, since a
	// dated task at the same priority is more urgent than an undated one).
	// It is the default ordering for a task listing.
	SortUrgency = "urgency"
)

// TaskSortFields lists the fields a task listing may be ordered by.
var TaskSortFields = []string{
	SortUrgency, SortCreatedAt, SortUpdatedAt, SortPriority, SortDueAt, SortSeq, SortTitle,
}

// Validate checks the filter and normalizes its page.
func (f TaskFilter) Validate() (TaskFilter, error) {
	if f.ParentID != "" && f.ParentIsNull {
		return f, Invalid("parent_id and parent_is_null are mutually exclusive")
	}
	for _, p := range f.Priorities {
		if !p.Valid() {
			return f, Invalid("priority %d is out of range", p)
		}
	}
	if err := f.Exclude.Validate(); err != nil {
		return f, err
	}
	for _, t := range f.Text {
		if err := t.Validate(); err != nil {
			return f, err
		}
	}
	if f.DueBefore != nil && f.DueAfter != nil && f.DueBefore.Before(*f.DueAfter) {
		return f, Invalid("due_before must not precede due_after")
	}

	if f.Page.Sort == "" {
		f.Page.Sort = SortUrgency
	}
	if !validSortField(f.Page.Sort) {
		return f, Invalid("cannot sort tasks by %q", f.Page.Sort)
	}

	page, err := f.Page.Normalize()
	if err != nil {
		return f, err
	}
	f.Page = page
	if f.PageNumber < 0 {
		return f, Invalid("page number must not be negative")
	}
	if f.PageNumber > 0 && f.Page.Cursor != "" {
		return f, Invalid("a page number and a cursor are mutually exclusive")
	}

	if f.Page.Cursor != "" {
		c, err := DecodeCursor(f.Page.Cursor)
		if err != nil {
			return f, err
		}
		if err := c.CheckOrdering(f.Page.Sort, f.Page.Direction); err != nil {
			return f, err
		}
	}
	return f, nil
}

func validSortField(s string) bool {
	for _, f := range TaskSortFields {
		if f == s {
			return true
		}
	}
	return false
}

// EventFilter selects events for a subscription.
type EventFilter struct {
	ProjectIDs []string    `json:"project_ids,omitempty" yaml:"project_ids,omitempty"`
	Types      []EventType `json:"types,omitempty" yaml:"types,omitempty"`
	SinceSeq   int64       `json:"since_seq,omitempty" yaml:"since_seq,omitempty"`
}

// EventPollInput asks for the events after a cursor, waiting for one when none is there yet.
type EventPollInput struct {
	SinceSeq   int64       `json:"since_seq,omitempty" yaml:"since_seq,omitempty"`
	Types      []EventType `json:"types,omitempty" yaml:"types,omitempty"`
	ProjectIDs []string    `json:"project_ids,omitempty" yaml:"project_ids,omitempty"`
	Limit      int         `json:"limit,omitempty" yaml:"limit,omitempty"`
	Wait       Duration    `json:"wait,omitempty" yaml:"wait,omitempty"`
}

// EventPage is one poll's events in sequence order and the cursor that resumes after them.
type EventPage struct {
	Events  []Event `json:"events" yaml:"events"`
	NextSeq int64   `json:"next_seq" yaml:"next_seq"`
}

// Matches reports whether an event satisfies the filter.
func (f EventFilter) Matches(e Event) bool {
	if len(f.ProjectIDs) > 0 && !containsString(f.ProjectIDs, e.ProjectID) {
		return false
	}
	if len(f.Types) == 0 {
		return true
	}
	for _, pattern := range f.Types {
		if matchEventType(string(pattern), string(e.Type)) {
			return true
		}
	}
	return false
}

func matchEventType(pattern, typ string) bool {
	if pattern == "*" || pattern == typ {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(typ, strings.TrimSuffix(pattern, "*"))
	}
	return false
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// AuditFilter selects audit entries.
type AuditFilter struct {
	SubjectType string     `json:"subject_type,omitempty" yaml:"subject_type,omitempty"`
	SubjectID   string     `json:"subject_id,omitempty" yaml:"subject_id,omitempty"`
	ActorIDs    []string   `json:"actor_ids,omitempty" yaml:"actor_ids,omitempty"`
	Actions     []string   `json:"actions,omitempty" yaml:"actions,omitempty"`
	Sources     []Source   `json:"sources,omitempty" yaml:"sources,omitempty"`
	Vias        []ViaKind  `json:"vias,omitempty" yaml:"vias,omitempty"`
	Tokens      []string   `json:"tokens,omitempty" yaml:"tokens,omitempty"`
	Since       *time.Time `json:"since,omitempty" yaml:"since,omitempty"`
	Until       *time.Time `json:"until,omitempty" yaml:"until,omitempty"`

	// Text is a weak, case-insensitive substring match over what an entry
	// records: its action, the kind of record it touched, the surface it
	// arrived from, and the before and after snapshots. It belongs to the
	// filter rather than to a caller's own loop because a page answered in
	// part by the store and in part by the caller cannot report a cursor: the
	// cursor the store returns names the end of the store page, not the last
	// row the caller kept, so every match the caller cut off was skipped
	// rather than deferred.
	Text string `json:"text,omitempty" yaml:"text,omitempty"`

	Page Page `json:"page,omitempty" yaml:"page,omitempty"`
}

// ProjectFilter selects projects.
type ProjectFilter struct {
	Keys            []string `json:"keys,omitempty" yaml:"keys,omitempty"`
	IncludeArchived bool     `json:"include_archived,omitempty" yaml:"include_archived,omitempty"`
	Page            Page     `json:"page,omitempty" yaml:"page,omitempty"`
}

// UserFilter selects the users a listing returns. People removed from the
// tenant are left out unless IncludeRemoved asks for them.
type UserFilter struct {
	IncludeRemoved bool `json:"include_removed,omitempty" yaml:"include_removed,omitempty"`
	// WithoutTOTP lists only people who hold no confirmed second factor.
	WithoutTOTP bool `json:"without_totp,omitempty" yaml:"without_totp,omitempty"`
	Page        Page `json:"page,omitempty" yaml:"page,omitempty"`
}

// DeliveryFilter selects webhook deliveries.
type DeliveryFilter struct {
	EndpointID string           `json:"endpoint_id,omitempty" yaml:"endpoint_id,omitempty"`
	Statuses   []DeliveryStatus `json:"statuses,omitempty" yaml:"statuses,omitempty"`
	Page       Page             `json:"page,omitempty" yaml:"page,omitempty"`
}
