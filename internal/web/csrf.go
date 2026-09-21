package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// CSRFCookieName is the cookie carrying the per-browser CSRF value.
const CSRFCookieName = "tix_csrf"

// CSRFFieldName is the hidden form field every state-changing form carries.
const CSRFFieldName = "csrf_token"

// csrfTokenBytes is the entropy of one CSRF value.
const csrfTokenBytes = 32

// csrfKey carries the value issued for this request.
type csrfKey struct{}

// withCSRF returns a request whose context carries the issued CSRF value.
func withCSRF(r *http.Request, token string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), csrfKey{}, token))
}

// csrfFrom returns the CSRF value issued for this request.
func csrfFrom(r *http.Request) string {
	token, _ := r.Context().Value(csrfKey{}).(string)
	return token
}

// issueCSRF returns the browser's CSRF value, minting and setting one when the
// request carries none.
func (h *handler) issueCSRF(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(CSRFCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	token := newCSRFToken()
	http.SetCookie(w, &http.Cookie{ // #nosec G124 -- readable by no script; Secure tracks TLS like the session cookie
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
	return token
}

// newCSRFToken mints one unguessable value.
func newCSRFToken() string {
	buf := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// checkCSRF refuses a submission whose form value does not match the cookie the
// browser was issued, which is what stops another origin posting on the
// session's behalf.
func (h *handler) checkCSRF(r *http.Request) error {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil || cookie.Value == "" {
		return core.Forbidden("this form has no csrf token; reload the page and try again")
	}
	if err := ensureForm(r); err != nil {
		return err
	}
	submitted := strings.TrimSpace(r.PostFormValue(CSRFFieldName))
	if submitted == "" {
		return core.Forbidden("this form has no csrf token; reload the page and try again")
	}
	if subtle.ConstantTimeCompare([]byte(submitted), []byte(cookie.Value)) != 1 {
		return core.Forbidden("this form's csrf token does not match this browser's session")
	}
	return nil
}
