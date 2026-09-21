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
// was added. It returns empty when the payload has nothing more to say.
func EventDetail(e core.Event) string {
	if e.Type == core.EventTaskTransitioned {
		from, _ := e.Payload["from"].(string)
		to, _ := e.Payload["to"].(string)
		if from != "" || to != "" {
			return from + " → " + to
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
