// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// DAVPath is where tix serve answers CalDAV, and DAVWellKnownPath the address
// RFC 6764 has a client try first, which redirects there.
const (
	DAVPath          = "/dav"
	DAVWellKnownPath = "/.well-known/caldav"
)

// DAVRoot is the context root a client is pointed at: DAVPath with its slash.
const DAVRoot = DAVPath + "/"

// MaxDAVPasswordName bounds the name a person gives an app password.
const MaxDAVPasswordName = 64

// DAVCollections is which calendar collections an app password exposes.
type DAVCollections string

// The collection choices. Assigned is the default.
const (
	DAVCollectionsAssigned DAVCollections = "assigned"
	DAVCollectionsProjects DAVCollections = "projects"
	DAVCollectionsAll      DAVCollections = "all"
)

// AllDAVCollections lists every choice, the default first.
var AllDAVCollections = []DAVCollections{DAVCollectionsAssigned, DAVCollectionsProjects, DAVCollectionsAll}

// Valid reports whether c is a known choice.
func (c DAVCollections) Valid() bool {
	return c == DAVCollectionsAssigned || c == DAVCollectionsProjects || c == DAVCollectionsAll
}

// Assigned reports whether the choice includes the Assigned to me collection.
func (c DAVCollections) Assigned() bool { return c == DAVCollectionsAssigned || c == DAVCollectionsAll }

// Projects reports whether the choice includes one collection per project.
func (c DAVCollections) Projects() bool { return c == DAVCollectionsProjects || c == DAVCollectionsAll }

// DAVAssignedCollection names the Assigned to me collection, and
// DAVProjectPrefix starts the name of a project's.
const (
	DAVAssignedCollection = "assigned"
	DAVProjectPrefix      = "p-"
)

// DAVInfo tells a client whether CalDAV is served and what to configure.
type DAVInfo struct {
	// Enabled reports whether the server answers CalDAV at all.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Path is the CalDAV context root, /dav/.
	Path string `json:"path" yaml:"path"`
	// URL is Path made absolute, empty when the surface does not know its own address.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
	// Username is the exact Basic username: the caller's sign-in email, or
	// their handle when their account has no email.
	Username string `json:"username" yaml:"username"`
	// Write reports whether the caller's DAV requests may change tasks: true
	// when their role holds task write and task transition. A read-only app
	// password is read-only whatever this says.
	Write bool `json:"write" yaml:"write"`
	// Collections are the choices an app password may be created with.
	Collections []DAVCollections `json:"collections" yaml:"collections"`
	// Reason says why CalDAV is off, when it is.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// DAVPassword is one of a person's app passwords, without its secret.
type DAVPassword struct {
	// ID identifies the app password; it is the identifier of the API token behind it.
	ID string `json:"id" yaml:"id"`
	// Name is the name its owner gave it, unique among their live app passwords.
	Name string `json:"name" yaml:"name"`
	// TokenName is the name of the API token behind it, as tix token ls shows it.
	TokenName string `json:"token_name" yaml:"token_name"`
	// ActorID is its owner.
	ActorID string `json:"actor_id" yaml:"actor_id"`
	// Scopes are the token's scopes, caldav alone.
	Scopes []Scope `json:"scopes" yaml:"scopes"`
	// Collections is which collections it exposes.
	Collections DAVCollections `json:"collections" yaml:"collections"`
	// ReadOnly keeps it from changing tasks, whatever its owner's role allows.
	ReadOnly bool `json:"read_only" yaml:"read_only"`
	// CreatedAt is when it was issued.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// ExpiresAt is when it stops working, absent when it does not expire.
	ExpiresAt *time.Time `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	// LastUsedAt is when a client last signed in with it.
	LastUsedAt *time.Time `json:"last_used_at,omitempty" yaml:"last_used_at,omitempty"`
	// RevokedAt is when it was revoked, absent while it is live.
	RevokedAt *time.Time `json:"revoked_at,omitempty" yaml:"revoked_at,omitempty"`
}

// Active reports whether the app password still signs in at now.
func (p DAVPassword) Active(now time.Time) bool {
	return (p.RevokedAt == nil || p.RevokedAt.After(now)) && (p.ExpiresAt == nil || p.ExpiresAt.After(now))
}

// IssuedDAVPassword carries an app password with its secret, returned only by
// the call that minted it.
type IssuedDAVPassword struct {
	DAVPassword
	// Password is the secret a client signs in with, shown once.
	Password string `json:"password" yaml:"password"`
	// Username is the Basic username to sign in with.
	Username string `json:"username" yaml:"username"`
	// Path is the CalDAV context root, /dav/.
	Path string `json:"path" yaml:"path"`
	// URL is Path made absolute, empty when the surface does not know its own address.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
}

// CreateDAVPasswordInput names a new app password and what it exposes.
type CreateDAVPasswordInput struct {
	// Name labels the app password, at most 64 characters, unique among the
	// caller's live ones. Required.
	Name string `json:"name" yaml:"name"`
	// Collections is assigned (the default), projects or all.
	Collections DAVCollections `json:"collections,omitempty" yaml:"collections,omitempty"`
	// ReadOnly keeps the password from changing tasks, whatever the role allows.
	ReadOnly bool `json:"read_only,omitempty" yaml:"read_only,omitempty"`
	// ExpiresAt is when it stops working, an RFC 3339 timestamp; omitted means never.
	ExpiresAt *time.Time `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
}

// Normalize trims the name, fills in the default choice and checks both.
func (in CreateDAVPasswordInput) Normalize() (CreateDAVPasswordInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return in, Invalid("an app password needs a name").WithDetail(DetailField, "name")
	}
	if utf8.RuneCountInString(in.Name) > MaxDAVPasswordName || strings.ContainsFunc(in.Name, isControl) {
		return in, Invalid("an app password name is at most %d printable characters", MaxDAVPasswordName).
			WithDetail(DetailField, "name")
	}
	if in.Collections == "" {
		in.Collections = DAVCollectionsAssigned
	}
	if !in.Collections.Valid() {
		return in, Invalid("collections %q must be %q, %q or %q", in.Collections,
			DAVCollectionsAssigned, DAVCollectionsProjects, DAVCollectionsAll).WithDetail(DetailField, "collections")
	}
	return in, nil
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f }

// DAVService manages the caller's own app passwords and says how to reach
// CalDAV.
type DAVService interface {
	// GetDAVInfo reports whether this server answers CalDAV, the address and
	// username a client is configured with, whether the caller's DAV requests
	// may change tasks, and the collection choices a password takes.
	GetDAVInfo(ctx context.Context) (*DAVInfo, error)
	// ListDAVPasswords lists the caller's app passwords, revoked ones
	// included, without their secrets.
	ListDAVPasswords(ctx context.Context) ([]DAVPassword, error)
	// CreateDAVPassword issues the caller an app password: an API token
	// carrying the caldav scope alone, for their own person actor, which signs
	// a calendar or reminders app in over HTTP Basic. The secret is returned
	// once. Any person who reads tasks may create one; an agent may not.
	CreateDAVPassword(ctx context.Context, in CreateDAVPasswordInput) (*IssuedDAVPassword, error)
	// RevokeDAVPassword revokes one of the caller's app passwords. A client
	// using it is refused from its next request.
	RevokeDAVPassword(ctx context.Context, id string) error
}

// DAVPrincipal is the person an app password signs in as, in the terms the
// CalDAV discovery answers with.
type DAVPrincipal struct {
	// TenantKey is the tenant's key, the first segment of every DAV path.
	TenantKey string `json:"tenant_key" yaml:"tenant_key"`
	// ActorID is the owner's identifier, lowercase, as it appears in paths.
	ActorID string `json:"actor_id" yaml:"actor_id"`
	// Handle is the owner's handle in the tenant.
	Handle string `json:"handle" yaml:"handle"`
	// DisplayName is the owner's name, falling back to the handle.
	DisplayName string `json:"display_name" yaml:"display_name"`
	// Email is the owner's sign-in address, empty when the account has none.
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
	// Write reports whether the principal may change tasks.
	Write bool `json:"write" yaml:"write"`
}

// DAVCollection is one calendar collection of a person's calendar home.
type DAVCollection struct {
	// Name is the collection's path segment: assigned, or p- and the
	// project's lowercase identifier.
	Name string `json:"name" yaml:"name"`
	// DisplayName is what a client shows the list as.
	DisplayName string `json:"display_name" yaml:"display_name"`
	// Description is the collection's calendar-description.
	Description string `json:"description" yaml:"description"`
	// Color is the list's colour as #rrggbb, empty for none.
	Color string `json:"color,omitempty" yaml:"color,omitempty"`
	// ProjectID is the project of a project collection.
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
}

// DAVMember is one object of a collection: its resource name, without the
// .ics suffix, and its entity tag.
type DAVMember struct {
	Name string `json:"name" yaml:"name"`
	ETag string `json:"etag" yaml:"etag"`
}

// DAVSnapshotInput asks for a collection's members, and optionally for what
// changed since a sync token.
type DAVSnapshotInput struct {
	// Collection is the collection's name.
	Collection string
	// Since is a sync token the client holds; empty asks for every member.
	Since string
	// Limit bounds the changes answered; zero is no limit.
	Limit int
	// Origin is the scheme and host the request reached the server at, which
	// every object's URL, and so its entity tag, is built from.
	Origin string
}

// DAVSnapshot is a collection as it stands and as it changed since a token.
type DAVSnapshot struct {
	// Collection is the collection answered.
	Collection DAVCollection
	// Members are every current member, in name order.
	Members []DAVMember
	// LeftOut counts the tasks past caldav.max_objects.
	LeftOut int
	// Token is the sync token for the state answered, which is the current
	// state unless Truncated.
	Token string
	// Changed are the members added or changed since the input's token, or
	// every member without one, in name order.
	Changed []DAVMember
	// Removed are the names that left since the input's token.
	Removed []string
	// Truncated reports that the limit cut the changes short, and Token leads
	// to the rest.
	Truncated bool
}

// DAVObjectsInput names objects of one collection to read.
type DAVObjectsInput struct {
	Collection string
	// Names are resource names without the .ics suffix; empty reads every member.
	Names []string
	// Origin is as DAVSnapshotInput's.
	Origin string
}

// DAVObject is one rendered object.
type DAVObject struct {
	Name string
	ETag string
	// Data is the VCALENDAR holding the task's VTODO.
	Data []byte
}

// TaskCollections answers CalDAV for the app password in the context. It is
// beside Service rather than part of it, as Provisioner is: it is reached
// only by internal/caldav, never by a client or a surface binding.
type TaskCollections interface {
	// DAVPrincipal returns the person the app password signs in as.
	DAVPrincipal(ctx context.Context) (*DAVPrincipal, error)
	// ListDAVCollections lists the collections the app password exposes and
	// its owner can read.
	ListDAVCollections(ctx context.Context) ([]DAVCollection, error)
	// DAVSnapshot answers a collection's members and, given a sync token,
	// what changed since. A token the server no longer holds is
	// ErrDAVSyncToken; a collection the reader cannot see is not found.
	DAVSnapshot(ctx context.Context, in DAVSnapshotInput) (*DAVSnapshot, error)
	// GetDAVObjects renders the named members of a collection. A name the
	// collection does not hold is left out of the answer.
	GetDAVObjects(ctx context.Context, in DAVObjectsInput) ([]DAVObject, error)
	// PutDAVObject changes the task behind an object by what differs from
	// what tix would render for it, or creates a task when the collection
	// holds no object of that name. Every change is made by UpdateTask,
	// CreateTask, SnoozeTask and TransitionRoute as the owner.
	PutDAVObject(ctx context.Context, in DAVPutInput) (*DAVPutResult, error)
	// DeleteDAVObject cancels the open task behind an object, and leaves a
	// done one alone. It never moves a task to the trash.
	DeleteDAVObject(ctx context.Context, in DAVDeleteInput) error
}

// DAVPutInput is one PUT of an object.
type DAVPutInput struct {
	Collection string
	// Name is the resource name, without the .ics suffix.
	Name string
	// IfMatch is the If-Match header as sent, empty when absent.
	IfMatch string
	// IfNoneMatch reports If-None-Match: *, which asks for a create only.
	IfNoneMatch bool
	// Data is the VCALENDAR the client sent.
	Data []byte
	// Origin is as DAVSnapshotInput's.
	Origin string
}

// DAVPutResult says what a PUT did.
type DAVPutResult struct {
	// Created reports a new task, answered 201 rather than 204.
	Created bool
	// Task is the task as the write left it.
	Task *Task
	// KeptDropped reports that what tix has no field for was over the
	// bound and was not kept.
	KeptDropped bool
}

// DAVDeleteInput is one DELETE of an object.
type DAVDeleteInput struct {
	Collection string
	Name       string
	IfMatch    string
	Origin     string
}

// Reasons a CalDAV write is refused, carried as the reason detail so the
// protocol layer answers each with its own status and precondition.
const (
	// ReasonDAVIfMatchRequired is an update sent without If-Match: 428.
	ReasonDAVIfMatchRequired = "dav_if_match_required"
	// ReasonDAVStale is an If-Match or If-None-Match the object does not
	// meet, or a task changed while the write ran: 412.
	ReasonDAVStale = "dav_stale"
	// ReasonDAVUIDConflict is a create whose UID or name another task holds:
	// 403 with CALDAV:no-uid-conflict.
	ReasonDAVUIDConflict = "dav_uid_conflict"
	// ReasonDAVInvalidObject is a body that is not one VTODO tix can read:
	// 403 with CALDAV:valid-calendar-object-resource.
	ReasonDAVInvalidObject = "invalid_calendar_object"
	// ReasonDAVReadOnly is a write from a password or an owner that may not
	// write: 403 with DAV:need-privileges.
	ReasonDAVReadOnly = "dav_read_only"
	// ReasonDAVRefused is a change the task's rules refuse, such as a guarded
	// or ambiguous move: 403 with the reason as plain text.
	ReasonDAVRefused = "dav_refused"
)

// DAVRefusal is a CalDAV write refused for a reason, of the kind the reason
// maps to everywhere else.
func DAVRefusal(reason string, format string, args ...any) error {
	kind := KindForbidden
	switch reason {
	case ReasonDAVIfMatchRequired, ReasonDAVStale:
		kind = KindPrecondition
	}
	return newf(kind, format, args...).WithDetail("reason", reason)
}

// DAVReason is the reason detail of a CalDAV write refusal, empty for any
// other error.
func DAVReason(err error) string {
	var e *Error
	if !errors.As(err, &e) {
		return ""
	}
	switch r, _ := e.Details["reason"].(string); r {
	case ReasonDAVIfMatchRequired, ReasonDAVStale, ReasonDAVUIDConflict, ReasonDAVInvalidObject,
		ReasonDAVReadOnly, ReasonDAVRefused:
		return r
	}
	return ""
}

// ReasonDAVSyncToken is the reason detail of ErrDAVSyncToken.
const ReasonDAVSyncToken = "invalid_sync_token"

// ErrDAVSyncToken reports a sync token the server does not hold for the
// collection, which a client answers with a full sync.
func ErrDAVSyncToken() error {
	return Forbidden("the sync token is not known for this collection; sync again without one").
		WithDetail("reason", ReasonDAVSyncToken)
}

// IsDAVSyncToken reports whether err is ErrDAVSyncToken.
func IsDAVSyncToken(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Kind == KindForbidden && e.Details["reason"] == ReasonDAVSyncToken
}

type davRequestKey struct{}

// WithDAVRequest marks a request that arrived under DAVPath. Only the HTTP
// layer sets it, from the path; an app password is refused anywhere it is not
// set.
func WithDAVRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, davRequestKey{}, true)
}

// DAVRequestFrom reports whether ctx was marked by WithDAVRequest.
func DAVRequestFrom(ctx context.Context) bool {
	v, _ := ctx.Value(davRequestKey{}).(bool)
	return v
}

// IsDAVPath reports whether a request path is under the CalDAV root.
func IsDAVPath(path string) bool { return path == DAVPath || strings.HasPrefix(path, DAVRoot) }
