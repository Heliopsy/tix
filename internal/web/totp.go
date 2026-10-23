// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// The second factor in the browser: the code step of a sign-in, enrolment and
// recovery codes on the settings screen, and the administrative clear on the
// user list.
//
// Every screen here is a plain form. The sign-in path has to work with
// scripting off, as every other form in this interface does, and the code
// step is the worst possible place to break that: a reader who cannot
// complete it cannot reach any screen at all. Nothing on these screens is
// drawn by a script, the otpauth code included, which is drawn as SVG by the
// encoder in qr.go.

// Route patterns for the second factor. Enrolment hangs off the settings
// screen because that is where a reader manages their own credentials; the
// administrative clear hangs off the user list, where somebody else's are.
const (
	RouteTOTPSetup     = "/settings/totp"
	RouteTOTPConfirm   = "/settings/totp/confirm"
	RouteTOTPCodes     = "/settings/totp/recovery-codes"
	RouteTOTPRemove    = "/settings/totp/remove"
	RouteUserTOTPClear = "/admin/users/totp/clear"
)

// The two screens the second factor draws for itself. Neither is reached by a
// GET: one is answered to a form submission carrying a ticket or a session,
// and the other shows recovery codes that exist for exactly one response.
const (
	totpTemplate      = "totp.html"
	totpCodesTemplate = "totpcodes.html"
)

// totpRoutes are enrolment, recovery codes and removal, plus the
// administrative clear.
func (h *handler) totpRoutes() []route {
	return []route{
		post(RouteTOTPSetup, h.beginTOTP, "BeginTOTPEnrolment"),
		post(RouteTOTPConfirm, h.confirmTOTP, "ConfirmTOTPEnrolment"),
		post(RouteTOTPCodes, h.regenerateRecoveryCodes, "RegenerateRecoveryCodes"),
		post(RouteTOTPRemove, h.removeTOTP, "RemoveTOTP"),
		post(RouteUserTOTPClear, h.clearUserTOTP, "ClearUserTOTP"),
	}
}

// totpView is the code form. One view draws three things that differ only in
// where they post and whether an enrolment is in progress: the code step of a
// sign-in, the enrolment step of a sign-in that found no enrolment, and
// enrolment started from the settings screen.
type totpView struct {
	Action  string
	Heading string
	Lede    string
	// Ticket is the signed first step of a sign-in, carried forward in a
	// hidden field. It is empty on the settings screen, where the session
	// itself says who the reader is.
	Ticket string
	// Next is the destination the sign-in was interrupted on its way to. It
	// is carried through both steps so a deep link still lands.
	Next string
	// Cancel is where a reader who changed their mind goes.
	Cancel string
	// Recovery says whether a recovery code is accepted in the code field. It
	// is not during enrolment: there are no codes yet.
	Recovery bool
	Enrol    bool
	URI      string
	Key      string
	QR       template.HTML
	Errors   fieldErrors
}

// totpCodesView is the one response a set of recovery codes appears in. They
// are generated, shown and forgotten: nothing can produce them again, which
// is why this screen is rendered rather than redirected to.
type totpCodesView struct {
	Codes []string
	// Continue is where the reader goes once they have kept the codes.
	Continue string
	Lede     string
}

// Joined is every code on its own line, for the copy control and for anybody
// selecting the block by hand with scripting off.
func (v totpCodesView) Joined() string { return strings.Join(v.Codes, "\n") }

// signInChallenge builds the code step of a sign-in. An enrolment URI means
// the tenant requires a factor the reader does not hold yet, so the step
// takes them through enrolment instead of refusing them.
func signInChallenge(ticket, next, uri string, errs fieldErrors) totpView {
	v := totpView{
		Action:   RouteLogin,
		Heading:  "Second step",
		Lede:     "Enter the six digit code from your authenticator app.",
		Ticket:   ticket,
		Next:     next,
		Cancel:   RouteLogin,
		Recovery: true,
		Errors:   errs,
	}
	if uri == "" {
		return v
	}
	v.Lede = "This tenant asks for a second factor and this account holds none yet. " +
		"Add the code below to an authenticator app, then enter the code it shows."
	v.Recovery = false
	return v.enrolling(uri)
}

// enrolment builds the enrolment screen the settings screen starts.
func enrolment(uri string, errs fieldErrors) totpView {
	v := totpView{
		Action:  RouteTOTPConfirm,
		Heading: "Add a second factor",
		Lede: "Scan this with an authenticator app, or type the key into it, " +
			"then enter the code it shows to prove it works.",
		Cancel: RouteSettings,
		Errors: errs,
	}
	return v.enrolling(uri)
}

// enrolling fills in what an enrolment draws from the otpauth URI: the key to
// type by hand and the code to scan.
//
// The URI is carried in the form and read back from it rather than asked for
// again, because the service hands the secret over exactly once, when the
// enrolment begins. A URI that is not one is drawn as neither a key nor a
// code: it reached this screen through a submitted field, so it is treated as
// one.
func (v totpView) enrolling(uri string) totpView {
	v.Enrol = true
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "otpauth" || parsed.Host != "totp" {
		return v
	}
	v.URI = uri
	v.Key = groupKey(parsed.Query().Get("secret"))
	if qr, err := qrSVG(uri, "The enrolment code for this account, as a qr code"); err == nil {
		v.QR = qr
	}
	return v
}

// groupKey breaks the shared secret into groups of four, which is how an
// authenticator asks for it when it is typed rather than scanned.
func groupKey(secret string) string {
	var out strings.Builder
	for i, r := range secret {
		if i > 0 && i%4 == 0 {
			out.WriteString(" ")
		}
		out.WriteRune(r)
	}
	return out.String()
}

// totpRefusal says why a code was not accepted, beside the field it was typed
// in.
//
// A locked verifier is rate limiting, not a wrong code, and has to read as
// one: a reader told their code is wrong tries again immediately, which is
// the thing the lock exists to stop.
func totpRefusal(err error) fieldErrors {
	message := safeMessage(err, core.KindOf(err))
	if core.IsTOTPLocked(err) {
		return fieldErrors{"code": message + ". Wait, then try again."}
	}
	return fieldErrors{"code": message}
}

// renderTOTP draws the code form, with the status of whatever refused it.
func (h *handler) renderTOTP(w http.ResponseWriter, r *http.Request, status int, v totpView) error {
	return h.renderStatus(w, r, status, totpTemplate, v.Heading, v)
}

// renderRecoveryCodes shows a set of codes once.
func (h *handler) renderRecoveryCodes(w http.ResponseWriter, r *http.Request, codes []string,
	next, lede string,
) error {
	return h.render(w, r, totpCodesTemplate, "Recovery codes",
		totpCodesView{Codes: codes, Continue: next, Lede: lede})
}

// beginTOTP starts enrolment and draws the code to scan.
//
// It answers with the screen rather than a redirect because the secret is in
// it: a redirect would have to carry the secret in a URL, where it reaches
// the access log, the browser history and any referrer.
func (h *handler) beginTOTP(w http.ResponseWriter, r *http.Request) error {
	started, err := held(h.svc.BeginTOTPEnrolment(r.Context()))
	if err != nil {
		return err
	}
	return h.renderTOTP(w, r, http.StatusOK, enrolment(started.URI, nil))
}

// confirmTOTP proves possession and shows the recovery codes once.
func (h *handler) confirmTOTP(w http.ResponseWriter, r *http.Request) error {
	codes, err := h.svc.ConfirmTOTPEnrolment(r.Context(), field(r, "code"))
	if err != nil {
		return h.renderTOTP(w, r, core.KindOf(err).HTTPStatus(),
			enrolment(field(r, "uri"), totpRefusal(err)))
	}
	return h.renderRecoveryCodes(w, r, codes.Codes, RouteSettings,
		"Your second factor is on. Keep these codes somewhere safe: "+
			"each one signs you in once if you lose the authenticator, and they are not shown again.")
}

// regenerateRecoveryCodes replaces the reader's codes and shows the new set.
func (h *handler) regenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	codes, err := held(h.svc.RegenerateRecoveryCodes(r.Context()))
	if err != nil {
		return err
	}
	return h.renderRecoveryCodes(w, r, codes.Codes, RouteSettings,
		"These replace every code issued before, including any already used. "+
			"They are not shown again.")
}

// removeTOTP drops the reader's own enrolment.
func (h *handler) removeTOTP(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RemoveTOTP(r.Context()); err != nil {
		return err
	}
	redirect(w, r, RouteSettings, "second factor removed")
	return nil
}

// clearUserTOTP returns another user to an unenrolled state. The scope it
// needs is checked by the service; the control is drawn only for a reader who
// holds it.
func (h *handler) clearUserTOTP(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.ClearUserTOTP(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteUsers, "second factor cleared")
	return nil
}

// totpSection is what the settings screen says about the reader's own second
// factor. Status is nil when the caller is not a person, which is the one
// case TOTPStatus refuses: an agent holding a token has no enrolment to
// manage and the section says so rather than failing the screen.
type totpSection struct {
	Status *core.TOTPStatus
}

// Available reports whether this caller has a second factor to manage.
func (s totpSection) Available() bool { return s.Status != nil }

// Removable reports whether the reader may drop their enrolment. A tenant
// that requires a factor in the browser is not offered the control: removing
// it there only means the next sign-in walks them through enrolment again.
func (s totpSection) Removable() bool {
	return s.Status != nil && s.Status.Enrolled && !s.Status.RequiredWeb
}

// totpFor reads the caller's enrolment for the settings screen. A caller with
// no user record behind it is not an error here, only a section with nothing
// to show.
func (h *handler) totpFor(r *http.Request) (totpSection, error) {
	status, err := h.svc.TOTPStatus(r.Context())
	if core.IsKind(err, core.KindInvalid) {
		return totpSection{}, nil
	}
	if err != nil {
		return totpSection{}, err
	}
	return totpSection{Status: status}, nil
}
