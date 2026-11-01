// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// keyChange is the confirmation a changed project key waits on. It carries the
// whole submitted form, so confirming saves every field the reader changed and
// not only the key.
type keyChange struct {
	From, To    string
	Examples    []refChange
	Name        string
	Description string
	Workflow    string
	Color       string
	Icon        string

	// IconKind and IconName repeat the icon picker's answer when it sent
	// one. A browser cannot carry an uploaded file to a second screen, so
	// ReuploadImage asks for the image again instead of losing it quietly.
	IconKind      string
	IconKindSent  bool
	IconName      string
	ReuploadImage bool
}

// refChange is one task reference before and after the rename.
type refChange struct{ From, To string }

// exampleRefs shows what the rename does to the project's first references, or
// to a reference it would have, when it holds no task yet.
func (h *handler) exampleRefs(r *http.Request, p *core.Project, to string) ([]refChange, error) {
	page, err := h.svc.ListTasks(r.Context(), core.TaskFilter{
		ProjectKeys:    []string{p.Key},
		IncludeSnoozed: true,
		Page:           core.Page{Limit: 2, Sort: core.SortSeq},
	})
	if err != nil {
		return nil, err
	}
	seqs := []int64{1}
	if len(page.Tasks) > 0 {
		seqs = seqs[:0]
		for _, t := range page.Tasks {
			seqs = append(seqs, t.Seq)
		}
	}
	out := make([]refChange, 0, len(seqs))
	for _, seq := range seqs {
		out = append(out, refChange{
			From: core.TaskRef{ProjectKey: p.Key, Seq: seq}.String(),
			To:   core.TaskRef{ProjectKey: to, Seq: seq}.String(),
		})
	}
	return out, nil
}

// confirmKeyChange renders the project screen as a question when the settings
// form changes the key, and reports whether it did. Nothing is written until
// the reader confirms, because every link to the project's tasks changes.
func (h *handler) confirmKeyChange(w http.ResponseWriter, r *http.Request, p *core.Project, to string) (bool, error) {
	if to == "" || strings.EqualFold(to, p.Key) || checked(r, "confirm_key") {
		return false, nil
	}
	if err := core.ValidateProjectKey(to); err != nil {
		return false, err
	}
	examples, err := h.exampleRefs(r, p, strings.ToLower(to))
	if err != nil {
		return false, err
	}
	change := &keyChange{
		From: p.Key, To: strings.ToLower(to), Examples: examples,
		Name: field(r, "name"), Description: prose(r, "description"),
		Workflow: field(r, "workflow_key"), Color: colourField(r, "color"), Icon: field(r, "icon"),
		IconName: field(r, "icon_name"),
	}
	change.IconKind, change.IconKindSent = sent(r, "icon_kind")
	if change.IconKind == iconKindImage {
		file, err := uploadedIcon(r)
		if err != nil {
			return false, err
		}
		change.ReuploadImage = file != nil
	}
	return true, h.render(w, r, "board.html", p.Name, boardView{Project: *p, KeyChange: change})
}

// movedTo is the request's own location with the one path segment that named
// a record by an old key replaced by the current one, query kept.
func movedTo(r *http.Request, from, to string) string {
	segs := strings.Split(r.URL.Path, "/")
	for i, s := range segs {
		if s == from {
			segs[i] = to
			break
		}
	}
	target := strings.Join(segs, "/")
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	return target
}

// redirectOldKey answers a screen that named a project by a key it was renamed
// from with a permanent redirect to the same screen under the current key, and
// reports whether it did. Only the screens call it: a form post is served where
// it was sent, so a form left open across a rename still saves.
func redirectOldKey(w http.ResponseWriter, r *http.Request, asked string, p *core.Project) bool {
	if asked == p.ID || strings.EqualFold(asked, p.Key) {
		return false
	}
	// #nosec G710 -- the target is this request's own path, which the router
	// matched and cleaned, with one segment swapped for a stored project key;
	// a key holds letters, digits, hyphens and underscores only.
	http.Redirect(w, r, movedTo(r, asked, p.Key), http.StatusMovedPermanently)
	return true
}

// redirectOldRef is redirectOldKey for a task reference.
func redirectOldRef(w http.ResponseWriter, r *http.Request, asked string, t *core.Task) bool {
	ref, err := core.ParseTaskRef(asked)
	if err != nil || ref.IsID() || strings.EqualFold(asked, t.Ref) {
		return false
	}
	// #nosec G710 -- as in redirectOldKey: this request's own cleaned path
	// with one segment swapped for a reference built from a stored key.
	http.Redirect(w, r, movedTo(r, asked, t.Ref), http.StatusMovedPermanently)
	return true
}
