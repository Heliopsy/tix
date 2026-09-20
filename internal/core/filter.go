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
//
// Listings page by (sort key, id) rather than by OFFSET. OFFSET at depth scans
// every skipped row, which is what makes a tracker feel broken once it holds
// real data; keyset pagination stays constant-time at any page.
type Cursor struct {
	// SortValue is the value of the sort field on the last row of the previous
	// page. Encoded as a string so one cursor type serves every sort field.
	SortValue string `json:"v"`
	// ID breaks ties, so rows sharing a sort value still page deterministically.
	ID string `json:"i"`
	// Sort and Direction pin the cursor to the ordering it was produced for. A
	// cursor replayed against a different ordering would silently skip or repeat
	// rows, so it is rejected instead.
	Sort      string        `json:"s"`
	Direction SortDirection `json:"d"`
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

// DecodeCursor parses a cursor token. An empty token decodes to the zero
// cursor, meaning the first page.
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

// CheckOrdering reports an error when the cursor was produced for a different
// ordering than the one now requested.
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
	// Limit caps the rows returned. Zero means DefaultPageLimit.
	Limit int `json:"limit,omitempty"`
	// Cursor positions the page. Empty means the first page.
	Cursor string `json:"cursor,omitempty"`
	// Sort names the field to order by. Empty means the listing's default.
	Sort string `json:"sort,omitempty"`
	// Direction orders the sort. Empty means Ascending.
	Direction SortDirection `json:"direction,omitempty"`
}

// Page limits.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 500
)

// Normalize clamps the page to usable values, returning an error only for input
// that cannot be interpreted. An over-large limit is clamped rather than
// rejected, so a caller asking for too much gets a usable answer.
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
	Tasks []Task `json:"tasks"`
	// NextCursor is empty when there are no further pages.
	NextCursor string `json:"next_cursor,omitempty"`
}

// TriState expresses an optional boolean filter, distinguishing "not filtered"
// from "filtered to false".
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

// TaskFilter selects tasks. Every field is optional; the zero value selects all
// live tasks in the tenant.
type TaskFilter struct {
	ProjectIDs  []string `json:"project_ids,omitempty"`
	ProjectKeys []string `json:"project_keys,omitempty"`
	Statuses    []string `json:"statuses,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	AssigneeIDs []string `json:"assignee_ids,omitempty"`
	CreatorIDs  []string `json:"creator_ids,omitempty"`

	Priorities []Priority `json:"priorities,omitempty"`

	DueBefore *time.Time `json:"due_before,omitempty"`
	DueAfter  *time.Time `json:"due_after,omitempty"`

	// ParentID selects children of one task. Set ParentIsNull to select only
	// top-level tasks instead.
	ParentID     string `json:"parent_id,omitempty"`
	ParentIsNull bool   `json:"parent_is_null,omitempty"`

	// Claimed and Blocked filter on computed state. Claimed honours lease
	// expiry, so an expired lease reads as unclaimed.
	Claimed TriState `json:"claimed,omitempty"`
	Blocked TriState `json:"blocked,omitempty"`

	// ClaimedBy selects tasks held by specific actors.
	ClaimedBy []string `json:"claimed_by,omitempty"`

	// Query is a free-text search over title and body.
	Query string `json:"query,omitempty"`

	// CustomFields filters on custom field values. Filtering a field not marked
	// indexed is a scan, which is documented behaviour rather than an error.
	CustomFields map[string]any `json:"custom_fields,omitempty"`

	// IncludeDeleted includes soft-deleted tasks, which are excluded by default.
	IncludeDeleted bool `json:"include_deleted,omitempty"`

	Page Page `json:"page,omitempty"`
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

// TaskSortFields lists the fields a task listing may be ordered by. Restricting
// this is what keeps every sort backed by an index.
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
	ProjectIDs []string    `json:"project_ids,omitempty"`
	Types      []EventType `json:"types,omitempty"`
	// SinceSeq resumes from a cursor. Because events are durable, a reconnect
	// replays anything missed rather than leaving a gap.
	SinceSeq int64 `json:"since_seq,omitempty"`
}

// Matches reports whether an event satisfies the filter. Type patterns may end
// in "*" to match a prefix, so "task.*" selects every task event.
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
	SubjectType string     `json:"subject_type,omitempty"`
	SubjectID   string     `json:"subject_id,omitempty"`
	ActorIDs    []string   `json:"actor_ids,omitempty"`
	Actions     []string   `json:"actions,omitempty"`
	Sources     []Source   `json:"sources,omitempty"`
	Since       *time.Time `json:"since,omitempty"`
	Until       *time.Time `json:"until,omitempty"`
	Page        Page       `json:"page,omitempty"`
}

// ProjectFilter selects projects.
type ProjectFilter struct {
	Keys            []string `json:"keys,omitempty"`
	IncludeArchived bool     `json:"include_archived,omitempty"`
	Page            Page     `json:"page,omitempty"`
}

// DeliveryFilter selects webhook deliveries.
type DeliveryFilter struct {
	EndpointID string           `json:"endpoint_id,omitempty"`
	Statuses   []DeliveryStatus `json:"statuses,omitempty"`
	Page       Page             `json:"page,omitempty"`
}
