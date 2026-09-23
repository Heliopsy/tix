// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// noisyFields change on every mutation and name nothing a reader would act
// on, so they never appear in a history sentence or its field summary. They
// still appear in a row's raw disclosure, because the full record still has
// to be inspectable.
var noisyFields = map[string]bool{
	"updated_at":   true,
	"version":      true,
	"completed_at": true, // implied by a move into a terminal state
}

// fieldLabels names the task fields a history sentence may mention. A field
// missing here falls back to its raw key with underscores turned to spaces,
// which stays legible without needing an entry for every custom field.
var fieldLabels = map[string]string{
	"title":             "the title",
	"body":              "the description",
	"priority":          "the priority",
	"assignee_actor_id": "the assignee",
	"tags":              "the tags",
	"due_at":            "the due date",
	"custom_fields":     "a custom field",
	"depends_on":        "a dependency",
	"blocked":           "the blocked state",
}

// fieldLabel names one changed field for a sentence.
func fieldLabel(key string) string {
	if label, ok := fieldLabels[key]; ok {
		return label
	}
	return strings.ReplaceAll(key, "_", " ")
}

// meaningfulFields drops the noisy bookkeeping fields from a changed-field
// list, preserving order.
func meaningfulFields(fields []string) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if !noisyFields[f] {
			out = append(out, f)
		}
	}
	return out
}

// historyGroup is what one line of the history view renders: a single user
// action, which may have produced more than one audit entry.
//
// completeTask (tasks.go) walks a workflow path one hop at a time, and the
// service correctly writes one audit entry per hop -- each hop is a real,
// individually authorized transition, so it earns its own row in the ledger.
// But a reader who clicked "complete" once did one thing, not three, so a
// chain of task.transition entries where each hop's start matches the
// previous hop's end is folded into a single group here rather than shown as
// unrelated rows.
type historyGroup struct {
	Hops       []historyRow // chronological order, oldest first
	Sentence   string
	Verb       string
	Source     core.Source
	OccurredAt time.Time
}

// Latest is the entry a group's row is keyed and time-stamped by: the most
// recent hop, since that is when the action, as the reader experienced it,
// finished.
func (g historyGroup) Latest() historyRow { return g.Hops[len(g.Hops)-1] }

// groupHistory turns a task's audit trail, oldest first, into the rows a
// reader sees. A run of task.transition entries by the same actor chains
// into one group exactly when each hop's starting status is the previous
// hop's ending status: that is what "one click walked a multi-hop path"
// looks like in the data, and it is the only condition under which entries
// are merged. Anything else, including two genuinely separate transitions
// that happen to share a minute, stays on its own row.
func groupHistory(rows []historyRow) []historyGroup {
	var groups []historyGroup
	for _, row := range rows {
		if continuesChain(groups, row) {
			groups[len(groups)-1].Hops = append(groups[len(groups)-1].Hops, row)
			groups[len(groups)-1].OccurredAt = row.Entry.OccurredAt
			groups[len(groups)-1].Source = row.Entry.Source
			continue
		}
		groups = append(groups, historyGroup{
			Hops: []historyRow{row}, Source: row.Entry.Source, OccurredAt: row.Entry.OccurredAt,
		})
	}
	for i := range groups {
		groups[i].Verb = actionVerb(groups[i].Latest().Entry.Action)
	}
	return groups
}

// maxHopGap bounds how far apart two hops of one chain may occur.
// completeTask (tasks.go) issues every hop of a path from the same request,
// in a tight loop with no user think-time between them, so genuine hops of
// one click land milliseconds apart. Without this bound, an actor completing
// a task and later reopening it -- two distinct clicks -- would chain into
// one another whenever the reopen's path happened to retrace the complete's,
// since status continuity alone cannot tell "one request" from "the same
// path walked twice".
const maxHopGap = 5 * time.Second

// continuesChain reports whether row is the next hop of the open group at
// the end of groups: the same subject, the same actor, the same transition
// action, its starting status equal to the previous hop's ending status, and
// close enough in time to plausibly be the same request.
//
// The subject check is not needed by attachHistory's own caller, since a
// single task's audit trail already shares one subject throughout, but it is
// required once the same grouping runs over the tenant-wide activity feed
// (activity.go), whose rows interleave many different subjects: without it,
// two unrelated tasks whose statuses and timing happened to line up could
// chain into one row.
func continuesChain(groups []historyGroup, row historyRow) bool {
	if len(groups) == 0 || row.Entry.Action != "task.transition" {
		return false
	}
	last := groups[len(groups)-1].Latest()
	if last.Entry.Action != "task.transition" || last.Entry.ActorID != row.Entry.ActorID {
		return false
	}
	if last.Entry.SubjectType != row.Entry.SubjectType || last.Entry.SubjectID != row.Entry.SubjectID {
		return false
	}
	if gap := row.Entry.OccurredAt.Sub(last.Entry.OccurredAt); gap < 0 || gap > maxHopGap {
		return false
	}
	prevAfter, _ := stringField(last.Entry.After, "status")
	nextBefore, ok := stringField(row.Entry.Before, "status")
	if !ok || prevAfter == "" || prevAfter != nextBefore {
		return false
	}
	return true
}

// stringField reads one string field out of an audit snapshot.
func stringField(raw []byte, key string) (string, bool) {
	m := decodeSnapshot(raw)
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// sentenceFor renders a group as the sentence the task detail page shows:
// who, and what changed, in words rather than field names. The subject is
// always "this", since the whole page it appears on is already about one
// task and naming it again would be noise.
func sentenceFor(actorLabel string, g historyGroup) string {
	return string(sentenceForSubject(actorLabel, "", "this", "", g))
}

// sentenceForSubject is sentenceFor generalised with an explicit name for
// the record acted on, and an optional link for it. The task detail page
// calls it (via sentenceFor) with the subject fixed at "this"; the
// tenant-wide activity feed (activity.go) calls it directly, naming and
// linking the task a row is about, since a feed spanning every task has no
// single implied subject the way one task's own history does. Reusing this
// one switch statement for both, rather than writing a second sentence
// builder for the feed, is what keeps the two views from ever disagreeing
// about how the same kind of event reads.
//
// It returns template.HTML because the subject, and the actor, may each
// carry a real anchor tag: on the tenant-wide feed the actor's name links to
// the same feed narrowed to that actor, which is how a reader follows one
// person's trail. The task detail page passes no actor link, since every row
// there is already about the one task.
//
// It returns template.HTML because subject may carry a real anchor tag.
// Every other dynamic fragment -- the actor label, state names, field
// labels -- is escaped by hand, since returning template.HTML bypasses the
// automatic escaping html/template would otherwise apply to the whole
// string.
func sentenceForSubject(actorLabel, actorHref, subjectText, subjectHref string, g historyGroup) template.HTML {
	esc := template.HTMLEscapeString
	subject := esc(subjectText)
	if subjectHref != "" {
		subject = `<a href="` + esc(subjectHref) + `">` + subject + `</a>`
	}
	if actorLabel == "" {
		actorLabel = "The system"
		actorHref = ""
	}
	actor := esc(actorLabel)
	if actorHref != "" {
		actor = `<a class="who-link" href="` + esc(actorHref) + `">` + actor + `</a>`
	}
	first, last := g.Hops[0], g.Latest()
	switch {
	case first.Entry.Action == "task.transition":
		from, _ := stringField(first.Entry.Before, "status")
		to, _ := stringField(last.Entry.After, "status")
		sentence := actor + " moved " + subject + " from " + esc(from) + " to " + esc(to)
		if len(g.Hops) > 1 {
			sentence += " (" + strconv.Itoa(len(g.Hops)) + " steps)"
		}
		return template.HTML(sentence) // #nosec G203 -- every dynamic piece above is escaped by hand
	case last.Entry.Action == "task.create":
		return template.HTML(actor + " created " + subject) // #nosec G203
	case last.Entry.Action == "task.delete":
		return template.HTML(actor + " deleted " + subject) // #nosec G203
	case last.Entry.Action == "task.restore":
		return template.HTML(actor + " restored " + subject) // #nosec G203
	case last.Entry.Action == "task.claim":
		return template.HTML(actor + " claimed " + subject) // #nosec G203
	case last.Entry.Action == "task.release":
		return template.HTML(actor + " released the claim on " + subject) // #nosec G203
	case last.Entry.Action == "task.lease_renew":
		return template.HTML(actor + " renewed the claim on " + subject) // #nosec G203
	case last.Entry.Action == "task.lease_expire":
		return template.HTML("The claim on " + subject + " expired") // #nosec G203
	case last.Entry.Action == "user.login":
		return template.HTML(actor + " signed in") // #nosec G203
	case last.Entry.Action == "user.logout":
		return template.HTML(actor + " signed out") // #nosec G203
	case last.Entry.Action == "member.add":
		return template.HTML(actor + " added " + subject + " to the tenant") // #nosec G203
	case last.Entry.Action == "member.remove":
		return template.HTML(actor + " removed " + subject + " from the tenant") // #nosec G203
	case last.Entry.Action == "task.update":
		fields := meaningfulFields(last.Changed)
		if len(fields) == 0 {
			return template.HTML(actor + " updated " + subject) // #nosec G203
		}
		labels := make([]string, 0, len(fields))
		for _, f := range fields {
			labels = append(labels, esc(fieldLabel(f)))
		}
		sentence := actor + " updated " + strings.Join(labels, ", ")
		// The task page's own sentence never names "this" a second time here,
		// so an explicit subject is appended only when there is one to show.
		if subjectText != "this" {
			sentence += " on " + subject
		}
		return template.HTML(sentence) // #nosec G203
	default:
		// Every task action above is spelled out because each reads as its
		// own sentence shape ("moved", "claimed", "released the claim on").
		// A subject that is not a task -- a project, a workflow, a tenant,
		// and so on (activity.go's subjectFor names them) -- writes far
		// fewer distinct actions, and none of them need a shape of their
		// own: the three-way split the row's own badge already uses
		// (actionVerb) says everything a reader needs -- created, deleted,
		// or some other change -- without a switch arm per action.
		switch actionVerb(last.Entry.Action) {
		case "create":
			return template.HTML(actor + " created " + subject) // #nosec G203
		case "delete":
			return template.HTML(actor + " deleted " + subject) // #nosec G203
		default:
			return template.HTML(actor + " changed " + subject) // #nosec G203
		}
	}
}

// The history view used to carry its own relative-time renderer here. It now
// goes through the handler's TimeStyle (bound into the "relativeAt" template
// function in render.go), which is output.TimeStyle.Relative, so there is a
// single implementation of what "a while ago" means across the CLI and the
// web.
