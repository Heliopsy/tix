// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"net/http"
	"net/url"
	"slices"

	"github.com/heliopsy/tix/internal/core"
)

// DialogParam names the dialog a screen is drawn with open. It is the address
// a dialog's opener links to, so with scripting off the same form is drawn in
// place on the page it belongs to, and a refused submission comes back to it.
const DialogParam = "dialog"

// dialogState is which of a screen's dialogs is open, and, when a submission
// from it was refused, what it was sent with and why it was refused.
type dialogState struct {
	Open   string
	Errors fieldErrors
	Values url.Values
	status int
}

// dialogKey carries a refused submission's dialogState to the screen drawn
// for it.
type dialogKey struct{}

// dialogOf is the dialog a request draws open: a refusal's, or the one its
// address names.
func dialogOf(r *http.Request) dialogState {
	if d, ok := r.Context().Value(dialogKey{}).(dialogState); ok {
		return d
	}
	return dialogState{Open: r.URL.Query().Get(DialogParam)}
}

// Opens reports whether the dialog with this id is drawn open.
func (d dialogState) Opens(id string) bool { return id != "" && d.Open == id }

// For is the state of one dialog: its values and refusal when it is the one
// open, and nothing otherwise, so two dialogs on a screen never share them.
func (d dialogState) For(id string) dialogState {
	if !d.Opens(id) {
		return dialogState{}
	}
	return d
}

// Refused reports whether this dialog comes back from a refused submission.
func (d dialogState) Refused() bool { return d.Values != nil }

// Value is what a refused submission sent in one field.
func (d dialogState) Value(name string) string { return d.Values.Get(name) }

// Chose reports whether a refused submission chose this value of a field.
func (d dialogState) Chose(name, value string) bool {
	return slices.Contains(d.Values[name], value)
}

// Checked is a checkbox as the dialog draws it: as submitted after a
// refusal, otherwise its default.
func (d dialogState) Checked(name string, def bool) bool {
	if !d.Refused() {
		return def
	}
	return checkedValue(d.Values.Get(name))
}

// refuseIn answers a submission the reader can correct by drawing the screen
// it came from again, with the dialog it was made in open, holding what was
// typed and the refusal beside the field it is about. Anything else is not
// about the form and goes to the error screen.
func (h *handler) refuseIn(w http.ResponseWriter, r *http.Request, id, path string, err error,
	show func(http.ResponseWriter, *http.Request) error) error {
	if !refusable(err) {
		return err
	}
	state := dialogState{Open: id, Errors: refusal(err), Values: r.PostForm,
		status: core.KindOf(err).HTTPStatus()}
	if state.Values == nil {
		state.Values = url.Values{}
	}
	if r.URL.Path != path {
		w.Header().Set("HX-Replace-Url", path+"?"+url.Values{DialogParam: {id}}.Encode())
	}
	clone := r.Clone(context.WithValue(r.Context(), dialogKey{}, state))
	clone.Method = http.MethodGet
	clone.URL = &url.URL{Path: path}
	return show(w, clone)
}

// drawnWith is a refused request as the screen at path draws it, with the
// dialog id open, for a handler that carries its own refusal to the screen.
func drawnWith(r *http.Request, id, path string) *http.Request {
	clone := r.Clone(context.WithValue(r.Context(), dialogKey{}, dialogState{Open: id}))
	clone.URL = &url.URL{Path: path}
	return clone
}

// refusedStatus is the status a screen drawn for a refused submission
// answers with, or ok when the screen was asked for.
func refusedStatus(r *http.Request, ok int) int {
	if d, found := r.Context().Value(dialogKey{}).(dialogState); found && d.status != 0 && ok == http.StatusOK {
		return d.status
	}
	return ok
}

// dialogOpener is a button that opens a dialog, and the link it is without
// scripting.
type dialogOpener struct {
	ID    string
	Label string
	Icon  string
	Class string
	Href  string
	// Aria names the button in full where its visible label leans on the
	// row it sits in, such as Manage beside an account's name.
	Aria string
}

// Named is the opener with a full accessible name.
func (o dialogOpener) Named(aria string) dialogOpener {
	o.Aria = aria
	return o
}

// newOpener builds the "dlg-open" partial's argument: the dialog it opens,
// its label, an icon from the sprite, the button's intent class, and the
// screen it is on.
func newOpener(id, label, icon, class, here string) dialogOpener {
	return dialogOpener{ID: id, Label: label, Icon: icon, Class: class, Href: dialogHref(here, id)}
}

// dialogShell is a dialog's frame: its id, title, whether it is drawn open,
// and where Cancel goes without scripting.
type dialogShell struct {
	ID    string
	Label string
	Open  bool
	Back  string
	Class string
}

// newShell builds the "dlg-start" partial's argument.
func newShell(id, label, class string, state dialogState, here string) dialogShell {
	return dialogShell{ID: id, Label: label, Class: class, Open: state.Opens(id), Back: withoutDialog(here)}
}

// dialogHref is a screen's address with one dialog drawn open, scrolled to
// it, since without scripting it is drawn where the template places it.
func dialogHref(here, id string) string {
	u, err := url.Parse(here)
	if err != nil {
		return here
	}
	q := u.Query()
	q.Set(DialogParam, id)
	u.RawQuery = q.Encode()
	u.Fragment = id
	return u.String()
}

// withoutDialog is a screen's address with no dialog open.
func withoutDialog(here string) string {
	u, err := url.Parse(here)
	if err != nil {
		return here
	}
	q := u.Query()
	q.Del(DialogParam)
	u.RawQuery = q.Encode()
	return u.String()
}
