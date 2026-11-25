// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "net/http"

// AddAnotherCookie remembers the quick-add row's Add another checkbox. It is
// written by the add itself, from the field the form posts, the way the lanes
// grouping is remembered by the screen that uses it, so it needs no script
// and no row on the settings screen.
const AddAnotherCookie = "tix_add_another"

// QuickFocusCookie is the one-shot mark an Add another redirect leaves, so
// the listing it lands on, and only that one, focuses the quick-add box.
const QuickFocusCookie = "tix_quick_focus"

// addAnother reports whether this browser last added with Add another ticked.
func addAnother(r *http.Request) bool {
	return cookieValue(r, AddAnotherCookie) == "1"
}

// rememberAddAnother stores what a quick add posted for Add another, and
// leaves the cookie alone when it already says that.
func (h *handler) rememberAddAnother(w http.ResponseWriter, r *http.Request, on bool) {
	if addAnother(r) == on {
		return
	}
	value, age := "1", cookieYear
	if !on {
		value, age = "", -1
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: AddAnotherCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
}

// markQuickFocus asks the next listing to focus the quick-add box. It lasts
// a minute, long enough for the redirect it rides on to be followed.
func (h *handler) markQuickFocus(w http.ResponseWriter, r *http.Request) {
	// #nosec G124 -- a one-shot display mark, readable by no script.
	http.SetCookie(w, &http.Cookie{
		Name: QuickFocusCookie, Value: "1", Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: 60,
	})
}

// takeQuickFocus reports whether the listing should focus the quick-add box,
// clearing the mark so a later visit does not.
func (h *handler) takeQuickFocus(w http.ResponseWriter, r *http.Request) bool {
	if cookieValue(r, QuickFocusCookie) == "" {
		return false
	}
	// #nosec G124 -- clears the one-shot mark above.
	http.SetCookie(w, &http.Cookie{
		Name: QuickFocusCookie, Value: "", Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: -1,
	})
	return true
}

func init() {
	jsMessages = append(jsMessages, "Quick add suggestions", "The clipboard holds no text.")
}
