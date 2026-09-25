// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strings"
	"sync"

	"github.com/heliopsy/tix/internal/core"
)

// RouteActors is the directory screen: the people and the agents of this
// tenant, which is also what the assignee picker draws its handles from.
const RouteActors = "/actors"

// actorRoutes is the directory screen.
func (h *handler) actorRoutes() []route {
	return []route{get(RouteActors, "actors.html", h.showActors, "ListActors")}
}

// actorsView is what the directory screen renders.
type actorsView struct {
	Actors []core.Actor
	Pager  pager
}

// showActors renders this tenant's directory.
func (h *handler) showActors(w http.ResponseWriter, r *http.Request) error {
	actors, next, err := h.svc.ListActors(r.Context(), core.Page{Cursor: r.URL.Query().Get(CursorParam)})
	if err != nil {
		return err
	}
	return h.render(w, r, "actors.html", "Directory", actorsView{
		Actors: actors, Pager: newPager(r, RouteActors, next, len(actors), "actors")})
}

// actorNames labels the actor identifiers one screen shows. A handle is used
// wherever the directory holds one, because a name someone chose beats any
// name a machine can invent. Everything else falls back to a generated name,
// which is stable and pronounceable but means nothing beyond telling two rows
// apart. Either way the identifier stays on the element as its title.
type actorNames struct {
	labels    map[string]string
	directory func() []string
}

// Label returns the words to show for one actor identifier.
func (n actorNames) Label(id string) string {
	if id == "" {
		return ""
	}
	if handle := n.labels[id]; handle != "" {
		return handle
	}
	return core.FriendlyName(id)
}

// Handles lists every handle this tenant holds, in the order the directory
// returns them, for the pickers a form offers. Agents are in it: they are
// actors without a user, and work is assigned to them as often as to people.
func (n actorNames) Handles() []string {
	if n.directory == nil {
		return nil
	}
	return n.directory()
}

// resolveActors looks up the handles of the actors a screen names. An
// identifier the caller may not resolve, or that names nothing, is simply
// absent, so the screen falls back to its generated name rather than failing
// over a label.
func (h *handler) resolveActors(r *http.Request, ids ...string) actorNames {
	out := actorNames{labels: make(map[string]string, len(ids)), directory: h.actorHandles(r)}
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, seen := out.labels[id]; seen {
			continue
		}
		out.labels[id] = ""
		actor, err := h.svc.GetActor(r.Context(), id)
		if err != nil || actor == nil {
			continue
		}
		out.labels[id] = actor.Handle
	}
	return out
}

// actorHandles defers the directory listing to the first screen that asks for
// it, so the screens that only label the actors they name never pay for it. A
// listing that fails leaves the picker empty rather than losing the page: the
// field it decorates takes a typed identifier either way.
func (h *handler) actorHandles(r *http.Request) func() []string {
	var (
		once    sync.Once
		handles []string
	)
	return func() []string {
		once.Do(func() {
			actors, _, err := h.svc.ListActors(r.Context(), core.Page{Limit: core.MaxPageLimit})
			if err != nil {
				return
			}
			for _, a := range actors {
				if a.Handle != "" {
					handles = append(handles, a.Handle)
				}
			}
		})
		return handles
	}
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
	// A lease running out is the one entry nobody performed, and the only one
	// that records work being dropped rather than done. It read as "other",
	// the same mark a history gives an action this build has no opinion on.
	case strings.HasPrefix(verb, "expir"), strings.HasPrefix(verb, "lease_expir"):
		return "expire"
	default:
		return "other"
	}
}
