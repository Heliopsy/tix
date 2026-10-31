// SPDX-License-Identifier: AGPL-3.0-or-later

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

// csrfFrom returns the CSRF value issued for this request, masked afresh, so
// it is what a form on this response carries.
//
// Screens are compressed, and a compressed page that both reflects input (the
// filter box echoes q) and carries a constant secret leaks that secret a byte
// at a time to anyone who can watch response sizes and steer requests: BREACH.
// A one-time pad drawn per call makes the bytes of the token different on
// every page, so there is nothing constant for the compression to give away.
func csrfFrom(r *http.Request) string {
	token, _ := r.Context().Value(csrfKey{}).(string)
	return maskCSRF(token)
}

// maskCSRF returns pad || pad XOR token, URL-safe base64, for a token in the
// form newCSRFToken mints. Anything else is returned as it is.
func maskCSRF(token string) string {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) == 0 {
		return token
	}
	out := make([]byte, 2*len(raw))
	if _, err := rand.Read(out[:len(raw)]); err != nil {
		return token
	}
	for i, b := range raw {
		out[len(raw)+i] = out[i] ^ b
	}
	return base64.RawURLEncoding.EncodeToString(out)
}

// unmaskCSRF recovers the token maskCSRF hid, or "" when submitted is not a
// masked value.
func unmaskCSRF(submitted string) string {
	raw, err := base64.RawURLEncoding.DecodeString(submitted)
	if err != nil || len(raw) == 0 || len(raw)%2 != 0 {
		return ""
	}
	n := len(raw) / 2
	out := make([]byte, n)
	for i := range out {
		out[i] = raw[i] ^ raw[n+i]
	}
	return base64.RawURLEncoding.EncodeToString(out)
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
	plain := subtle.ConstantTimeCompare([]byte(submitted), []byte(cookie.Value)) == 1
	masked := subtle.ConstantTimeCompare([]byte(unmaskCSRF(submitted)), []byte(cookie.Value)) == 1
	if !plain && !masked {
		return core.Forbidden("this form's csrf token does not match this browser's session")
	}
	return nil
}
