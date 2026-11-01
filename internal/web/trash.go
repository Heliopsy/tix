// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"sort"

	"github.com/heliopsy/tix/internal/core"
)

// confirmParam is the field a permanent-delete confirmation posts back. Only a
// submission carrying it deletes on a tenant whose trash is off.
const confirmParam = "confirm"

// trashOff reports whether the reader's tenant deletes tasks permanently. A
// failed read answers false, which keeps the trash drawn: the service refuses
// is:trashed and restore by itself, so the cost is a link that explains, not
// a delete that surprises.
func (h *handler) trashOff(r *http.Request) bool {
	if _, ok := core.ActorFrom(r.Context()); !ok {
		return false
	}
	status, err := h.svc.TrashStatus(r.Context())
	return err == nil && !status.Enabled
}

// hiddenField is one posted value the confirmation carries back unchanged.
type hiddenField struct {
	Name  string
	Value string
}

// confirmDeleteView is the screen that stands in for an undo when nothing
// could be undone.
type confirmDeleteView struct {
	// Refs are the tasks the confirmed post would delete.
	Refs []string
	// Action is where the confirmation posts: the route the delete was
	// first posted to, with the place to return to.
	Action string
	Cancel string
	Fields []hiddenField
	// Cascade says the subtasks go too.
	Cascade bool
}

// askBeforePermanentDelete answers a delete on a tenant whose trash is off and
// that nobody has confirmed yet with a confirmation, and reports whether it
// did. It runs inside the delete handlers rather than on the buttons, so every
// place that draws a Delete asks, including ones drawn after this was written.
func (h *handler) askBeforePermanentDelete(w http.ResponseWriter, r *http.Request, refs []string, back string) (bool, error) {
	if checked(r, confirmParam) || !h.trashOff(r) {
		return false, nil
	}
	if err := r.ParseForm(); err != nil {
		return true, core.Invalid("reading the form: %v", err)
	}
	var fields []hiddenField
	for name, values := range r.PostForm {
		if name == "csrf_token" || name == confirmParam || name == "next" {
			continue
		}
		for _, v := range values {
			fields = append(fields, hiddenField{Name: name, Value: v})
		}
	}
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	q := r.URL.Query()
	q.Set("next", back)
	action := url.URL{Path: r.URL.Path, RawQuery: q.Encode()}
	// A request from a control that swaps one card or row would put the
	// confirmation inside that card. Swapping the page's main region instead
	// draws it as a screen.
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		w.Header().Set("HX-Retarget", "main")
		w.Header().Set("HX-Reselect", "main")
		w.Header().Set("HX-Reswap", "outerHTML")
	}
	return true, h.render(w, r, "taskdelete.html", "Delete permanently", confirmDeleteView{
		Refs: refs, Action: action.String(), Cancel: back, Fields: fields,
		Cascade: checked(r, "cascade"),
	})
}
