// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "net/http"

// oneTimeSecret is a value the server will never disclose again, shown once to
// whoever caused it to exist.
//
// One type and one "secret" partial, not one per screen. The token screen grew
// the pattern first -- a region of its own rather than a flash, a .copyline
// that wraps, and the generic data-copy-target delegate -- and the webhook
// screen's field-info had been promising the same thing without anything
// rendering it. A second copy of the markup would have been a second thing to
// keep correct.
type oneTimeSecret struct {
	// ID is the element identifier the copy button points at, and the stem of
	// the heading's identifier.
	ID string
	// Heading is the instruction, which is to copy the value now.
	Heading string
	// Note says why there is no second chance and what to do if it is missed.
	Note string
	// Value is the secret itself.
	Value string
}

// issuedTokenSecret presents a freshly minted API token, or nothing when no
// token was just issued.
func issuedTokenSecret(value string) *oneTimeSecret {
	if value == "" {
		return nil
	}
	return &oneTimeSecret{
		ID:      "issued-token",
		Heading: "Copy this token now",
		Note: "It is shown once and cannot be retrieved again. " +
			"If you lose it, revoke the token and issue another.",
		Value: value,
	}
}

// generatedWebhookSecret presents a signing secret tix generated for an
// endpoint, or nothing when the operator supplied their own.
func generatedWebhookSecret(value string) *oneTimeSecret {
	if value == "" {
		return nil
	}
	return &oneTimeSecret{
		ID:      "generated-secret",
		Heading: "Copy this signing secret now",
		Note: "tix generated it because you left the field blank, and it is shown once. " +
			"Configure it on the receiving end; if you lose it, save the endpoint again with a secret of your own.",
		Value: value,
	}
}

// oneTimeCookie carries a one-time secret to the screen that shows it, so the
// value never appears in a URL or in the audit trail. Each screen uses its own
// name and its own path, so one screen's value cannot be read by another.
func oneTimeCookie(name, path, value string, secure bool) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: path,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: 60}
}

// readOneTimeCookie reads and clears a one-time secret cookie.
func readOneTimeCookie(w http.ResponseWriter, r *http.Request, name, path string) string {
	cookie, err := r.Cookie(name)
	if err != nil || cookie.Value == "" {
		return ""
	}
	// #nosec G124 -- this clears the cookie (MaxAge -1). HttpOnly and SameSite
	// are set; Secure follows the deployment and is applied where it is issued.
	http.SetCookie(w, &http.Cookie{Name: name, Path: path,
		MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	return cookie.Value
}
