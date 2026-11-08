// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// actorFilterParam names the actor the token and SSH key screens list.
const actorFilterParam = "actor"

// actorFilter is the Actor control above the token and SSH key tables. The
// command line and the API list another actor's credentials on token:admin,
// and so does the browser through this: the value goes to the service as
// typed, which resolves a handle or an identifier and decides whether the
// reader may.
type actorFilter struct {
	Action string
	Value  string
	// Label names the actor listed, by handle where the directory has one.
	Label string
	// Missing is a value that names no actor of this tenant.
	Missing bool
	handles func() []string
}

// Handles are the directory's handles, offered as suggestions.
func (f actorFilter) Handles() []string {
	if f.handles == nil {
		return nil
	}
	return f.handles()
}

// Active reports whether the screen lists a chosen actor's credentials.
func (f actorFilter) Active() bool { return f.Value != "" }

// actorFilterOf reads the filter out of a screen request. A refused form
// draws its screen from a POST, which names no actor, so it lists the
// reader's own as the screen does by default.
func (h *handler) actorFilterOf(r *http.Request, action string) actorFilter {
	out := actorFilter{Action: action, handles: h.actorHandles(r)}
	if r.Method == http.MethodGet {
		out.Value = strings.TrimSpace(r.URL.Query().Get(actorFilterParam))
	}
	return out
}

// listed records the outcome of the listing the filter asked for: the
// actor's name when it worked, and the reason when the reader can correct it.
// Any other failure is the screen's.
func (f *actorFilter) listed(owner string, err error) error {
	if err == nil {
		f.Label = owner
		return nil
	}
	if core.IsKind(err, core.KindNotFound) || core.IsKind(err, core.KindInvalid) {
		f.Missing = true
		return nil
	}
	return err
}

// filterTarget resolves the actor the filter names, or nil when it names none,
// either because it is empty or because no actor of this tenant answers to
// it.
func (h *handler) filterTarget(r *http.Request, f *actorFilter) (*core.Actor, error) {
	if !f.Active() {
		return nil, nil
	}
	found, err := h.svc.GetActor(r.Context(), f.Value)
	label := ""
	if err == nil {
		label = found.Handle
		if label == "" {
			label = core.FriendlyName(found.ID)
		}
	}
	if err := f.listed(label, err); err != nil {
		return nil, err
	}
	if f.Missing {
		return nil, nil
	}
	return found, nil
}
