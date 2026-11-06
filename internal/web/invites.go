// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
)

// createModeInvite is the create dialog's choice of sending an invite link
// rather than setting a password.
const createModeInvite = "invite"

// issuedInviteCookie carries a freshly issued invite link to the Users screen,
// which shows it once.
const issuedInviteCookie = "tix_issued_invite"

// inviteLinkDialog is the dialog the Users screen shows an issued link in.
const inviteLinkDialog = "invite-link"

// inviteExpiry is one lifetime the invite form offers.
type inviteExpiry struct {
	Key   string
	Label string
	Days  int
}

// inviteExpiryChoices are the lifetimes the form offers, and the only values
// it accepts.
var inviteExpiryChoices = []inviteExpiry{
	{Key: "1d", Label: "1 day", Days: 1},
	{Key: "3d", Label: "3 days", Days: 3},
	{Key: "7d", Label: "7 days", Days: 7},
	{Key: "14d", Label: "14 days", Days: 14},
	{Key: "30d", Label: "30 days", Days: 30},
}

// defaultInviteExpiry is what the form proposes, the service's own default.
const defaultInviteExpiry = "7d"

// inviteLifetime reads the chosen lifetime, refusing a value the form does
// not offer rather than reading it as the default.
func inviteLifetime(r *http.Request) (core.Duration, error) {
	key := field(r, "expires")
	if key == "" {
		key = defaultInviteExpiry
	}
	for _, c := range inviteExpiryChoices {
		if c.Key == key {
			return core.Duration(time.Duration(c.Days) * 24 * time.Hour), nil
		}
	}
	return 0, core.Invalid("%q is not one of the expiry choices", key).WithDetail(core.DetailField, "expires")
}

// inviteRow is one pending invite as the Users screen lists it.
type inviteRow struct {
	core.Invite
	Expired bool
}

// inviteRows lists the pending invites, or none when the reader may not.
func (h *handler) inviteRows(r *http.Request) []inviteRow {
	invites, err := h.svc.ListInvites(r.Context())
	if err != nil {
		return nil
	}
	now := time.Now()
	out := make([]inviteRow, 0, len(invites))
	for _, inv := range invites {
		out = append(out, inviteRow{Invite: inv, Expired: !now.Before(inv.ExpiresAt)})
	}
	return out
}

// issuedInviteLink is a link shown once, right after it was issued.
type issuedInviteLink struct {
	URL       string
	ExpiresAt time.Time
}

// issuedInvite reads and clears the one-time invite cookie. Its value is the
// token and the expiry in Unix seconds, joined by a dot, which no token holds.
func issuedInvite(w http.ResponseWriter, r *http.Request) *issuedInviteLink {
	value := readOneTimeCookie(w, r, issuedInviteCookie, RouteUsers)
	token, unix, ok := strings.Cut(value, ".")
	if !ok || auth.ParseInviteToken(token) != nil {
		return nil
	}
	seconds, err := strconv.ParseInt(unix, 10, 64)
	if err != nil {
		return nil
	}
	w.Header().Set("Referrer-Policy", "no-referrer")
	return &issuedInviteLink{URL: httpapi.InviteURL(r, token), ExpiresAt: time.Unix(seconds, 0).UTC()}
}

// showIssuedInvite hands an issued link to the Users screen, which draws it
// in its dialog once.
func (h *handler) showIssuedInvite(w http.ResponseWriter, r *http.Request, issued *core.IssuedInvite, flash string) {
	value := issued.Token + "." + strconv.FormatInt(issued.ExpiresAt.Unix(), 10)
	// #nosec G124 -- one-time value, HttpOnly, cleared by the screen that shows it
	http.SetCookie(w, oneTimeCookie(issuedInviteCookie, RouteUsers, value, h.secureCookie(r)))
	redirect(w, r, dialogHref(RouteUsers, inviteLinkDialog), flash)
}

// createInvite invites an address and shows its link once.
func (h *handler) createInvite(w http.ResponseWriter, r *http.Request) error {
	lifetime, err := inviteLifetime(r)
	if err != nil {
		return h.refuseIn(w, r, "user-new", RouteUsers, err, h.showUsers)
	}
	issued, err := h.svc.CreateInvite(r.Context(), core.CreateInviteInput{
		Email:       field(r, "email"),
		DisplayName: field(r, "display_name"),
		Role:        core.Role(field(r, "role")),
		ExpiresIn:   lifetime,
	})
	if err != nil {
		return h.refuseIn(w, r, "user-new", RouteUsers, err, h.showUsers)
	}
	h.showIssuedInvite(w, r, issued, "invite created")
	return nil
}

// revokeInvite stops a pending invite's link.
func (h *handler) revokeInvite(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RevokeInvite(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteUsers, "invite revoked")
	return nil
}

// reissueInvite replaces an invite's link and shows the new one once.
func (h *handler) reissueInvite(w http.ResponseWriter, r *http.Request) error {
	lifetime, err := inviteLifetime(r)
	if err != nil {
		return err
	}
	issued, err := h.svc.ReissueInvite(r.Context(), field(r, "id"), core.ReissueInviteInput{ExpiresIn: lifetime})
	if err != nil {
		return err
	}
	h.showIssuedInvite(w, r, issued, "invite re-issued")
	return nil
}

// inviteView is what the public invite page renders.
type inviteView struct {
	Token   string
	Preview *core.InvitePreview
	// Dead draws the one message every unusable link answers with.
	Dead   bool
	Errors fieldErrors
	// DisplayName is what the form holds: the account's, or what a refused
	// submission sent.
	DisplayName string
}

// showInvite renders the page an invite link opens.
func (h *handler) showInvite(w http.ResponseWriter, r *http.Request) error {
	token := r.PathValue("token")
	preview, err := h.svc.PreviewInvite(r.Context(), token)
	if err != nil {
		return h.deadInvite(w, r, err)
	}
	return h.renderInvite(w, r, http.StatusOK, inviteView{Token: token, Preview: preview, DisplayName: preview.DisplayName})
}

// acceptInvite sets the password, uses up the link and signs the person in.
func (h *handler) acceptInvite(w http.ResponseWriter, r *http.Request) error {
	token := r.PathValue("token")
	preview, err := h.svc.PreviewInvite(r.Context(), token)
	if err != nil {
		return h.deadInvite(w, r, err)
	}
	view := inviteView{Token: token, Preview: preview, DisplayName: field(r, "display_name")}
	password := r.PostFormValue("password")
	if !preview.ExistingAccount && password != r.PostFormValue("confirm") {
		view.Errors = fieldErrors{"confirm": "the two passwords do not match"}
		return h.renderInvite(w, r, http.StatusUnprocessableEntity, view)
	}
	result, err := h.svc.AcceptInvite(r.Context(), token, core.AcceptInviteInput{
		Password: password, DisplayName: view.DisplayName, Surface: core.SurfaceWeb,
	})
	switch {
	case core.IsKind(err, core.KindNotFound):
		return h.deadInvite(w, r, err)
	case err != nil && (correctable(err) || core.IsKind(err, core.KindUnauthenticated)):
		view.Errors = fieldErrors{"password": safeMessage(err, core.KindOf(err))}
		return h.renderInvite(w, r, core.KindOf(err).HTTPStatus(), view)
	case err != nil:
		return err
	}
	return h.finishSignIn(w, r, result, RouteProjects)
}

// deadInvite answers a link that cannot be used. Every such link, whatever
// made it unusable, gets this one page, so it says nothing about any other.
func (h *handler) deadInvite(w http.ResponseWriter, r *http.Request, err error) error {
	if !core.IsKind(err, core.KindNotFound) {
		return err
	}
	return h.renderInvite(w, r, http.StatusNotFound, inviteView{Dead: true})
}

// renderInvite draws the invite page. The address carries a secret, so it is
// kept out of any referrer.
func (h *handler) renderInvite(w http.ResponseWriter, r *http.Request, status int, v inviteView) error {
	w.Header().Set("Referrer-Policy", "no-referrer")
	return h.renderStatus(w, r, status, inviteTemplate, "Accept your invite", v)
}

// inviteTemplate is the public invite page.
const inviteTemplate = "invite.html"
