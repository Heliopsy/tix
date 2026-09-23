// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"fmt"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// EventVerb names an event type in plain words, such as task.lease_expired
// becoming "lease expired". It is the CLI's and the TUI's shared answer to
// "what happened", so the two surfaces cannot describe the same event type
// two different ways.
func EventVerb(t core.EventType) string {
	verb := string(t)
	if _, rest, ok := strings.Cut(verb, "."); ok {
		verb = rest
	}
	return strings.ReplaceAll(verb, "_", " ")
}

// EventRef names the subject an event happened to, the way the rest of the
// CLI names a task: its ref when the event recorded one, falling back to the
// raw subject identifier the audit trail uses for the same event when it did
// not.
func EventRef(e core.Event) string {
	if ref, ok := e.Payload["ref"].(string); ok && ref != "" {
		return ref
	}
	return e.SubjectID
}

// EventDetail summarises whatever an event's payload adds beyond its ref: a
// transition's states, a task's title on creation, the tag or dependency that
// was added, which fields an update touched. It returns empty when the
// payload has nothing more to say.
func EventDetail(e core.Event) string {
	if e.Type == core.EventTaskTransitioned {
		from, _ := e.Payload["from"].(string)
		to, _ := e.Payload["to"].(string)
		if from != "" || to != "" {
			return from + " → " + to
		}
	}
	if e.Type == core.EventTaskUpdated {
		if d := taskUpdatedDetail(e.Payload); d != "" {
			return d
		}
	}
	for _, key := range []string{"title", "tag", "depends_on", "name", "final_status", "status"} {
		v, ok := e.Payload[key]
		if !ok {
			continue
		}
		if s := strings.TrimSpace(fmt.Sprint(v)); s != "" {
			return s
		}
	}
	return ""
}

// taskUpdatedDetail is as specific as core.EventTaskUpdated's payload allows
// today. A field edit, a tag removal, a dependency removal, a comment
// deletion, a restore and a lease renewal all share this one event type, so
// the type alone cannot tell them apart; this reads whatever flag or field
// the payload for each of those actually carries.
//
// It cannot name which task fields a plain field edit touched, because
// task.update's own event payload (internal/service/task.go) carries only
// ref and version, not the changed field names the way the web history view's
// audit-snapshot diff does (internal/web/history.go's changedFields). Until
// the service adds that -- a "fields" key naming what changed, alongside ref
// -- a plain field edit falls through to the bare "updated" verb. If the
// service ever adds that key, using it here is a two-line change: read
// e.Payload["fields"], run it through SummariseFields.
func taskUpdatedDetail(payload map[string]any) string {
	if fields, ok := stringSlice(payload["fields"]); ok && len(fields) > 0 {
		return SummariseFields(fields)
	}
	if restored, _ := payload["restored"].(bool); restored {
		return "restored"
	}
	if deleted, _ := payload["deleted"].(bool); deleted {
		return "comment deleted"
	}
	if _, ok := payload["lease_expires_at"]; ok {
		return "lease renewed"
	}
	if removed, _ := payload["removed"].(bool); removed {
		if tag, ok := payload["tag"].(string); ok && tag != "" {
			return "tag removed: " + tag
		}
		if dep, ok := payload["depends_on"].(string); ok && dep != "" {
			return "dependency removed: " + dep
		}
	}
	if system, ok := payload["system"].(string); ok && system != "" {
		return "synced from " + system
	}
	return ""
}

// stringSlice reads a []string out of a decoded payload value, tolerating the
// []any shape JSON decoding produces.
func stringSlice(v any) ([]string, bool) {
	switch t := v.(type) {
	case []string:
		return t, true
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

// updateFieldLabels names the task fields a change summary may mention, bare
// of any article so SummariseFields can prefix the whole list with a single
// "the". A field missing here falls back to its raw key with underscores
// turned to spaces, which stays legible without needing an entry for every
// custom field. This deliberately does not import internal/web/history.go's
// own field-label map (a web page may not depend on a CLI package and vice
// versa); keeping it here, rather than duplicating it again in internal/tui,
// is what stops the CLI line and the terminal interface's status bar and
// activity view from ever naming the same field two different ways.
var updateFieldLabels = map[string]string{
	"title":             "title",
	"body":              "description",
	"priority":          "priority",
	"assignee_actor_id": "assignee",
	"tags":              "tags",
	"due_at":            "due date",
	"custom_fields":     "custom fields",
	"parent_ref":        "parent",
}

// FieldLabel names one changed task field for a change summary, such as
// "priority" for the key "priority_actor_id"'s sibling "priority". Callers
// that need the field named as a sentence subject prepend "the" themselves,
// or use SummariseFields for more than one.
func FieldLabel(key string) string {
	if label, ok := updateFieldLabels[key]; ok {
		return label
	}
	return strings.ReplaceAll(key, "_", " ")
}

// SummariseFields joins one or more changed fields into a sentence fragment:
// "the title", "the title and priority", "the title, priority and due date".
// Naming every field this way, rather than listing what each one changed to,
// is the summary the reporting gap asked for: precise about what changed,
// silent about the values, which keeps a five-field edit one short phrase
// instead of a paragraph.
func SummariseFields(fields []string) string {
	if len(fields) == 0 {
		return ""
	}
	labels := make([]string, len(fields))
	for i, f := range fields {
		labels[i] = FieldLabel(f)
	}
	switch len(labels) {
	case 1:
		return "the " + labels[0]
	default:
		return "the " + strings.Join(labels[:len(labels)-1], ", ") + " and " + labels[len(labels)-1]
	}
}

// EventActor names who caused an event, or a placeholder for the rare event
// with no actor attached.
func EventActor(e core.Event) string {
	// The payload names the actor when the service knew its handle. Falling
	// back to the identifier keeps an older event readable rather than blank.
	if h, ok := e.Payload["actor_handle"].(string); ok && h != "" {
		return h
	}
	if e.ActorID == "" {
		return "-"
	}
	return e.ActorID
}

// FormatEventLine renders one event as a single line: a time of day in the
// painter's configured zone, the actor, what happened, and the task or
// subject it happened to. It is what a live tail draws instead of buffering
// into a table that can never finish sizing its columns while the stream is
// still open.
func FormatEventLine(p Painter, e core.Event) string {
	line := strings.Join([]string{
		p.Muted(p.style.Clock(e.OccurredAt)),
		EventActor(e),
		EventVerb(e.Type),
		p.Ref(EventRef(e)),
	}, "  ")
	if detail := EventDetail(e); detail != "" {
		line += "  " + detail
	}
	return line
}
