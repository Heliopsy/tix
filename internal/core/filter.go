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

// Cursor is an opaque position in a keyset-paginated listing.
type Cursor struct {
	SortValue string        `json:"v" yaml:"v"`
	ID        string        `json:"i" yaml:"i"`
	Sort      string        `json:"s" yaml:"s"`
	Direction SortDirection `json:"d" yaml:"d"`
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
	Limit     int           `json:"limit,omitempty" yaml:"limit,omitempty"`
	Cursor    string        `json:"cursor,omitempty" yaml:"cursor,omitempty"`
	Sort      string        `json:"sort,omitempty" yaml:"sort,omitempty"`
	Direction SortDirection `json:"direction,omitempty" yaml:"direction,omitempty"`
}

// Page limits.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 500
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

	ClaimedBy []string `json:"claimed_by,omitempty" yaml:"claimed_by,omitempty"`

	Query string `json:"query,omitempty" yaml:"query,omitempty"`

	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`

	IncludeDeleted bool `json:"include_deleted,omitempty" yaml:"include_deleted,omitempty"`

	Page Page `json:"page,omitempty" yaml:"page,omitempty"`
}

// Task sort fields.
const (
	SortCreatedAt = "created_at"
	SortUpdatedAt = "updated_at"
	SortPriority  = "priority"
	SortDueAt     = "due_at"
	SortSeq       = "seq"
	SortTitle     = "title"
)

// TaskSortFields lists the fields a task listing may be ordered by.
var TaskSortFields = []string{
	SortCreatedAt, SortUpdatedAt, SortPriority, SortDueAt, SortSeq, SortTitle,
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
	if f.DueBefore != nil && f.DueAfter != nil && f.DueBefore.Before(*f.DueAfter) {
		return f, Invalid("due_before must not precede due_after")
	}

	if f.Page.Sort == "" {
		f.Page.Sort = SortCreatedAt
	}
	if !validSortField(f.Page.Sort) {
		return f, Invalid("cannot sort tasks by %q", f.Page.Sort)
	}

	page, err := f.Page.Normalize()
	if err != nil {
		return f, err
	}
	f.Page = page

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
	Since       *time.Time `json:"since,omitempty" yaml:"since,omitempty"`
	Until       *time.Time `json:"until,omitempty" yaml:"until,omitempty"`
	Page        Page       `json:"page,omitempty" yaml:"page,omitempty"`
}

// ProjectFilter selects projects.
type ProjectFilter struct {
	Keys            []string `json:"keys,omitempty" yaml:"keys,omitempty"`
	IncludeArchived bool     `json:"include_archived,omitempty" yaml:"include_archived,omitempty"`
	Page            Page     `json:"page,omitempty" yaml:"page,omitempty"`
}

// DeliveryFilter selects webhook deliveries.
type DeliveryFilter struct {
	EndpointID string           `json:"endpoint_id,omitempty" yaml:"endpoint_id,omitempty"`
	Statuses   []DeliveryStatus `json:"statuses,omitempty" yaml:"statuses,omitempty"`
	Page       Page             `json:"page,omitempty" yaml:"page,omitempty"`
}
