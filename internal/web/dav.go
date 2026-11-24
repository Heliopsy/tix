// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// The Tasks in other apps section of the settings screen: the CalDAV address
// and the reader's app passwords. A new password is answered as a screen
// rather than a redirect, as a feed address is, so the secret is in no URL,
// no access log and no browser history.
const (
	RouteDAVPasswordCreate = "/settings/dav/passwords"             // #nosec G101 -- a route path, not a credential
	RouteDAVPasswordRevoke = "/settings/dav/passwords/{id}/revoke" // #nosec G101 -- a route path, not a credential

	davIssuedTemplate = "davpassword.html"
	davSection        = "dav"
	davCreateDialog   = "dav-new"
)

// davRoutes are the section's two writes.
func (h *handler) davRoutes() []route {
	return []route{
		post(RouteDAVPasswordCreate, h.createDAVPassword, "CreateDAVPassword"),
		post(RouteDAVPasswordRevoke, h.revokeDAVPassword, "RevokeDAVPassword"),
	}
}

// davChoice is one collections choice of the create dialog.
type davChoice struct {
	Value core.DAVCollections
	Label string
	Help  string
}

// davChoiceLabels name each collections choice and say what it lists.
var davChoiceLabels = map[core.DAVCollections][2]string{
	core.DAVCollectionsAssigned: {"Assigned to me", "One list of the tasks assigned to you, in every project."},
	core.DAVCollectionsProjects: {"One list per project", "A list for each project you can read, with everybody's tasks."},
	core.DAVCollectionsAll:      {"Both", "Assigned to me and every project, so your own tasks appear twice."},
}

// davChoices are the collections choices in the order the service offers them.
func davChoices(offered []core.DAVCollections) []davChoice {
	if len(offered) == 0 {
		offered = core.AllDAVCollections
	}
	out := make([]davChoice, 0, len(offered))
	for _, c := range offered {
		labels, ok := davChoiceLabels[c]
		if !ok {
			labels = [2]string{string(c), ""}
		}
		out = append(out, davChoice{Value: c, Label: labels[0], Help: labels[1]})
	}
	return out
}

// davCollectionsLabel names a password's collections choice.
func davCollectionsLabel(c core.DAVCollections) string {
	if labels, ok := davChoiceLabels[c]; ok {
		return labels[0]
	}
	return string(c)
}

// davRow is one app password as the section lists it.
type davRow struct {
	core.DAVPassword
	// Live is false once it is revoked or expired, when it has no Revoke.
	Live    bool
	Expired bool
	// Revoke is where the confirm dialog posts.
	Revoke string
}

// CollectionsLabel names the password's collections choice.
func (r davRow) CollectionsLabel() string { return davCollectionsLabel(r.Collections) }

// DialogID is the id of the password's revoke dialog.
func (r davRow) DialogID() string { return "dav-revoke-" + r.ID }

// davView is the settings section. Available is false when the reader may not
// hold an app password at all, such as an agent or a session that reads no
// tasks; Info.Enabled is false when this server does not answer CalDAV.
type davView struct {
	Available bool
	Info      core.DAVInfo
	URL       string
	Passwords []davRow
	Choices   []davChoice
}

// Offered reports whether the section offers a new password.
func (v davView) Offered() bool { return v.Available && v.Info.Enabled }

// davAddress is the CalDAV context root under the origin the request reached.
func davAddress(r *http.Request, path string) string {
	if path == "" {
		path = core.DAVRoot
	}
	return baseURL(r) + path
}

// davFor reads the section. Lacking the authority is a section that says so,
// never a failed screen.
func (h *handler) davFor(r *http.Request) (davView, error) {
	info, err := h.svc.GetDAVInfo(r.Context())
	if davUnavailable(err) {
		return davView{}, nil
	}
	if err != nil {
		return davView{}, err
	}
	passwords, err := h.svc.ListDAVPasswords(r.Context())
	if davUnavailable(err) {
		return davView{}, nil
	}
	if err != nil {
		return davView{}, err
	}
	now := time.Now()
	rows := make([]davRow, 0, len(passwords))
	for _, p := range passwords {
		expired := p.RevokedAt == nil && p.ExpiresAt != nil && !p.ExpiresAt.After(now)
		rows = append(rows, davRow{DAVPassword: p, Live: p.RevokedAt == nil && !expired, Expired: expired,
			Revoke: davRevokePath(p.ID)})
	}
	return davView{Available: true, Info: *info, URL: davAddress(r, info.Path), Passwords: rows,
		Choices: davChoices(info.Collections)}, nil
}

// davUnavailable reports a refusal that means the reader holds no app
// passwords here, rather than a failure.
func davUnavailable(err error) bool {
	switch core.KindOf(err) {
	case core.KindForbidden, core.KindInvalid, core.KindUnauthenticated:
		return err != nil
	default:
		return false
	}
}

// davRevokePath is where one password's revoke dialog posts.
func davRevokePath(id string) string {
	return strings.Replace(RouteDAVPasswordRevoke, "{id}", id, 1)
}

// davIssuedView is the one screen an app password appears on.
type davIssuedView struct {
	Name        string
	Password    string
	Username    string
	URL         string
	Collections string
	ReadOnly    bool
}

// createDAVPassword issues the reader an app password and shows it once. A
// refusal the reader can correct comes back with the dialog open.
func (h *handler) createDAVPassword(w http.ResponseWriter, r *http.Request) error {
	issued, err := held(h.svc.CreateDAVPassword(r.Context(), core.CreateDAVPasswordInput{
		Name:        field(r, "name"),
		Collections: core.DAVCollections(field(r, "collections")),
		ReadOnly:    checkedValue(field(r, "read_only")),
	}))
	if err != nil {
		return h.refuseIn(w, r, davCreateDialog, RouteSettings, err, h.showSettings)
	}
	w.Header().Set("Referrer-Policy", "no-referrer")
	return h.render(w, r, davIssuedTemplate, "App password", davIssuedView{
		Name: issued.Name, Password: issued.Password, Username: issued.Username,
		URL: davAddress(r, issued.Path), Collections: davCollectionsLabel(issued.Collections),
		ReadOnly: issued.ReadOnly,
	})
}

// revokeDAVPassword revokes one of the reader's app passwords and returns to
// the section, where it is listed as revoked.
func (h *handler) revokeDAVPassword(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RevokeDAVPassword(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	redirect(w, r, RouteSettings+"#"+davSection, "app password revoked")
	return nil
}
