package web

import (
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// actorNames labels the actor identifiers one screen shows. A handle is used
// wherever the directory holds one, because a name someone chose beats any
// name a machine can invent. Everything else falls back to a generated name,
// which is stable and pronounceable but means nothing beyond telling two rows
// apart. Either way the identifier stays on the element as its title.
type actorNames map[string]string

// Label returns the words to show for one actor identifier.
func (n actorNames) Label(id string) string {
	if id == "" {
		return ""
	}
	if handle := n[id]; handle != "" {
		return handle
	}
	return core.FriendlyName(id)
}

// resolveActors looks up the handles of the actors a screen names. An
// identifier the caller may not resolve, or that names nothing, is simply
// absent, so the screen falls back to its generated name rather than failing
// over a label.
func (h *handler) resolveActors(r *http.Request, ids ...string) actorNames {
	out := make(actorNames, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, seen := out[id]; seen {
			continue
		}
		out[id] = ""
		actor, err := h.svc.GetActor(r.Context(), id)
		if err != nil || actor == nil {
			continue
		}
		out[id] = actor.Handle
	}
	return out
}

// shortID abbreviates an identifier that names a record rather than a person,
// so a row stays readable while the full value remains on its title.
func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:4] + "…" + id[len(id)-4:]
}

// actionVerb reduces an audit action or event type to the kind of change it
// records, so the feed can mark a creation apart from a deletion without
// enumerating every action the product will ever record.
func actionVerb(action string) string {
	_, verb, found := strings.Cut(action, ".")
	if !found {
		verb = action
	}
	switch {
	case strings.HasPrefix(verb, "creat"), strings.HasPrefix(verb, "add"),
		strings.HasPrefix(verb, "restor"), strings.HasPrefix(verb, "import"):
		return "create"
	case strings.HasPrefix(verb, "delet"), strings.HasPrefix(verb, "remov"),
		strings.HasPrefix(verb, "revok"), strings.HasPrefix(verb, "prun"),
		strings.HasPrefix(verb, "archiv"):
		return "delete"
	case strings.HasPrefix(verb, "updat"), strings.HasPrefix(verb, "edit"),
		strings.HasPrefix(verb, "transition"), strings.HasPrefix(verb, "put"),
		strings.HasPrefix(verb, "mov"):
		return "update"
	default:
		return "other"
	}
}
