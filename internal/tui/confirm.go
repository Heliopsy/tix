// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// confirmKind names the destructive action a confirmation stands in front of.
type confirmKind int

// The confirmations.
const (
	confirmNone confirmKind = iota
	confirmDeleteTask
	confirmDeleteComment
	confirmArchiveProject
	confirmDeleteProject
	confirmDeleteField
	confirmRemoveDomain
	confirmRemoveMember
	confirmDeleteTemplate
	confirmDeleteWebhook
	confirmDeleteSyncSource
	confirmPrune
	confirmDeleteUser
	confirmRevokeInvite
	confirmClearTOTP
	confirmEndConnection
	confirmDeleteTenant
	confirmRevokeGlobal
	confirmDeleteTenantUser
	confirmRevokeToken
	confirmRevokeKey
	confirmRevokeSession
	confirmRemoveTOTP
	confirmRecoveryCodes
	confirmRotateToken
	confirmImport
	confirmOverwrite
)

// confirmVerbs are what each confirmation says it will do, in the words the
// question asks it in.
var confirmVerbs = map[confirmKind]string{
	confirmDeleteTask:     "delete task",
	confirmDeleteComment:  "delete",
	confirmArchiveProject: "archive project",
	confirmDeleteProject:  "delete project",
	confirmDeleteField:    "delete custom field",
	confirmRemoveDomain:   "remove domain",
	confirmRemoveMember:   "remove member",
	confirmDeleteTemplate: "delete template",
	confirmDeleteUser:     "delete user",
	confirmRevokeInvite:   "revoke the invite for",
	confirmClearTOTP:      "clear the second factor of",

	confirmDeleteWebhook:    "delete endpoint",
	confirmDeleteSyncSource: "delete source",
	confirmPrune:            "prune",

	confirmEndConnection:    "end connection",
	confirmDeleteTenant:     "delete tenant",
	confirmRevokeGlobal:     "revoke global admin from",
	confirmDeleteTenantUser: "delete user",
	confirmRevokeToken:      "revoke",
	confirmRevokeKey:        "revoke",
	confirmRevokeSession:    "revoke",
	confirmRemoveTOTP:       "remove",
	confirmRecoveryCodes:    "replace",
	confirmRotateToken:      "rotate",
	confirmImport:           "import",
	confirmOverwrite:        "overwrite",
}

// Confirm is one destructive action held back until the reader agrees to it.
//
// Target is what the action will affect, spelled the way the reader sees it on
// screen. It is not optional: a confirmation that asks "are you sure?" names
// nothing, so agreeing to it is agreeing to whatever happened to be selected,
// which is how somebody deletes the wrong row. Question renders nothing without
// it, and the interface refuses to open a confirmation that renders nothing.
type Confirm struct {
	Kind   confirmKind
	Target string
	// Note is what the action will do beyond its subject, such as taking the
	// subtasks with it, which is the part a reader cannot see from the target.
	Note string

	// ref, commentID, hard and cascade address the subject for the service
	// call the agreement releases.
	ref       core.TaskRef
	label     string
	commentID string
	hard      bool
	cascade   bool
	// projectRef and fieldKey address the subjects that are not a task: the
	// project a removal acts on, and the custom field a deletion names.
	projectRef string
	fieldKey   string
	// deletion is what a project deletion does with the project's tasks.
	deletion core.DeleteProjectInput
	// hostname and actorID address the tenant screen's two removals, which act
	// on a domain and on a membership rather than on anything a task has.
	hostname string
	actorID  string
	// template names the task template a deletion removes.
	template string
	// integID addresses the endpoint or the source a deletion removes, and
	// prune is what an agreed prune sends.
	integID string
	prune   core.PruneInput
	// userID and inviteID address the people screen's removals.
	userID   string
	inviteID string
	// subject addresses what an administration view's confirmation acts on: a
	// connection identifier, a tenant key, an email or a user identifier.
	// within is the tenant a user deletion acts in.
	subject string
	within  string
	// credential is the token or key a revocation on the account screen names.
	credential string
	// grace is how long a rotated token's old value keeps working.
	grace time.Duration
}

// Open reports whether a confirmation is waiting for an answer.
func (c Confirm) Open() bool { return c.Kind != confirmNone }

// Question states what the action will do, naming its subject.
func (c Confirm) Question() string {
	verb := confirmVerbs[c.Kind]
	if verb == "" || c.Target == "" {
		return ""
	}
	q := verb + " " + c.Target
	if c.Note != "" {
		q += ", " + c.Note
	}
	return q + "?"
}

// DeleteNote says how far a task deletion will reach, which neither the ref nor
// the title can say. It renders nothing for the ordinary case, so the question
// only grows for the deletions that do more than they look like.
func DeleteNote(hard, cascade bool) string {
	switch {
	case hard && cascade:
		return "permanently, with its subtasks"
	case hard:
		return "permanently"
	case cascade:
		return "with its subtasks"
	default:
		return ""
	}
}

// ConfirmHelp is the instruction the confirmation carries, naming the keys that
// answer it rather than assuming the reader knows them, because the answer keys
// are the two a scheme is most likely to have moved.
func ConfirmHelp(agree, cancel string) string {
	return "(" + agree + " confirms, " + cancel + " cancels)"
}
