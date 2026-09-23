// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// sessionRoutes are the sign-in, sign-out and landing screens, plus the
// per-browser display preferences.
func (h *handler) sessionRoutes() []route {
	return []route{
		get(RouteRoot, "", h.showRoot, "WhoAmI"),
		public(get(RouteLogin, "login.html", h.showLogin)),
		public(post(RouteLogin, h.doLogin, "Login")),
		post(RouteLogout, h.doLogout, "Logout"),
		get(RouteSettings, "settings.html", h.showSettings, "WhoAmI"),
		post(RouteAdvanced, h.toggleAdvanced, "Logout"),
		post(RouteTheme, h.setTheme, "Logout"),
		post(RouteKeyScheme, h.setKeyScheme, "Logout"),
		post(RouteColumns, h.setColumns, "Logout"),
		post(RouteVisibility, h.setVisibility, "Logout"),
		post(RouteDragMove, h.toggleDragMove, "Logout"),
	}
}

// showRoot sends a signed-in browser to the project list.
func (h *handler) showRoot(w http.ResponseWriter, r *http.Request) error {
	if r.URL.Path != RouteRoot {
		return core.NotFound("no page at %q", r.URL.Path)
	}
	if _, err := h.svc.WhoAmI(r.Context()); err != nil {
		return err
	}
	http.Redirect(w, r, RouteProjects, http.StatusSeeOther)
	return nil
}

// loginView is what the sign-in screen renders. TargetHint is the resolved
// --db or --ctx target, when the process was started with WithTargetHint; it
// is not tenant- or account-specific, so showing it never depends on what
// was typed into the form above it.
type loginView struct {
	TargetHint string
}

// showLogin renders the sign-in screen.
func (h *handler) showLogin(w http.ResponseWriter, r *http.Request) error {
	if _, ok := core.ActorFrom(r.Context()); ok {
		http.Redirect(w, r, RouteProjects, http.StatusSeeOther)
		return nil
	}
	return h.render(w, r, "login.html", "Sign in", loginView{TargetHint: h.targetHint})
}

// doLogin exchanges an email and password for a browser session.
func (h *handler) doLogin(w http.ResponseWriter, r *http.Request) error {
	session, err := h.svc.Login(r.Context(), field(r, "email"), r.PostFormValue("password"))
	if err != nil {
		return err
	}
	http.SetCookie(w, auth.NewSessionCookie(session.Token, session.ExpiresAt, h.secureCookie(r)))
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// settingsView is what the dedicated settings screen renders. TargetDescribe
// is the redacted target description a process started with
// WithTargetDescribe carries; blank when the caller passed none. Shortcuts is
// the scheme comparison table, built once per request from shortcuts.go's own
// action and binding tables so it can never list a key the scripted help
// overlay does not also know about.
type settingsView struct {
	TargetDescribe string
	Shortcuts      shortcutTable
}

// showSettings renders the settings screen: every per-browser display
// preference, grouped and explained, replacing the sidebar disclosure that
// used to hold the same controls. Keeping both would let them drift, so the
// sidebar now only links here.
func (h *handler) showSettings(w http.ResponseWriter, r *http.Request) error {
	return h.render(w, r, "settings.html", "Settings", settingsView{
		TargetDescribe: h.targetDescribe,
		Shortcuts:      buildShortcutTable(),
	})
}

// doLogout ends the session the browser presented.
func (h *handler) doLogout(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.Logout(r.Context()); err != nil && !core.IsKind(err, core.KindUnauthenticated) {
		return err
	}
	http.SetCookie(w, auth.ClearSessionCookie(h.secureCookie(r)))
	http.Redirect(w, r, RouteLogin, http.StatusSeeOther)
	return nil
}

// toggleAdvanced flips the browser between the simple and the full interface.
// It is a per-browser preference, so it lives in a cookie rather than in the
// tenant record: two people sharing a tenant want different amounts of it.
func (h *handler) toggleAdvanced(w http.ResponseWriter, r *http.Request) error {
	value := "1"
	if advancedMode(r) {
		value = ""
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: AdvancedCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: cookieYear,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// toggleDragMove flips whether the board offers dragging a card between
// columns, in addition to its per-card Move disclosure, which never goes
// away: dragging is unreachable from a keyboard and unusable for anyone who
// cannot hold a pointer gesture. A per-browser preference, like Advanced, so
// it lives in a cookie; default on, unlike Advanced, so the polarity here is
// reversed -- an absent or non-"0" cookie means on.
func (h *handler) toggleDragMove(w http.ResponseWriter, r *http.Request) error {
	value := "0"
	if !dragMoveMode(r) {
		value = ""
	}
	// #nosec G124 -- a display preference, readable by no script.
	http.SetCookie(w, &http.Cookie{
		Name: DragMoveCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: cookieYear,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// setTheme records the colour scheme this browser wants.
func (h *handler) setTheme(w http.ResponseWriter, r *http.Request) error {
	want := field(r, "theme")
	value := ""
	for _, t := range Themes {
		if t != "" && t == want {
			value = t
		}
	}
	// #nosec G124 -- a display preference, readable by no script.
	http.SetCookie(w, &http.Cookie{
		Name: ThemeCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: cookieYear,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// setKeyScheme records the keyboard shortcut scheme this browser wants,
// following exactly the pattern setTheme above uses to persist a per-browser
// display preference: a cookie, not a tenant setting, because two people
// sharing a tenant may want different muscle memory.
func (h *handler) setKeyScheme(w http.ResponseWriter, r *http.Request) error {
	value := string(ParseKeyScheme(field(r, "keyscheme")))
	if value == string(KeySchemeDefault) {
		value = ""
	}
	// #nosec G124 -- a display preference, readable by no script.
	http.SetCookie(w, &http.Cookie{
		Name: KeySchemeCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: cookieYear,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// cookieYear keeps a display preference for a year.
const cookieYear = 365 * 24 * 60 * 60

// safeNext keeps a post-login redirect on this origin.
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return RouteProjects
	}
	// A backslash is treated as a separator by some browsers, so "/\evil.com"
	// can navigate off-site despite the leading slash. A control character can
	// split the header. Parse it and require a purely relative reference.
	if strings.ContainsAny(next, "\\\r\n\t") {
		return RouteProjects
	}
	u, err := url.Parse(next)
	if err != nil || u.IsAbs() || u.Host != "" || u.Scheme != "" {
		return RouteProjects
	}
	return u.String()
}

// The tenant-wide activity feed and its live-update fragment live in
// activity.go, next to the grouping and sentence rendering they share with
// the per-task history view in history.go.
