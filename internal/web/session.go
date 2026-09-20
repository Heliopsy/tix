package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/thereisnotime/tix/internal/auth"
	"github.com/thereisnotime/tix/internal/core"
)

// sessionRoutes are the sign-in, sign-out, landing and activity screens.
func (h *handler) sessionRoutes() []route {
	return []route{
		get(RouteRoot, "", h.showRoot, "WhoAmI"),
		public(get(RouteLogin, "login.html", h.showLogin)),
		public(post(RouteLogin, h.doLogin, "Login")),
		post(RouteLogout, h.doLogout, "Logout"),
		get(RouteActivity, "activity.html", h.showActivity, "ListAudit"),
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

// showLogin renders the sign-in screen.
func (h *handler) showLogin(w http.ResponseWriter, r *http.Request) error {
	if _, ok := core.ActorFrom(r.Context()); ok {
		http.Redirect(w, r, RouteProjects, http.StatusSeeOther)
		return nil
	}
	return h.render(w, r, "login.html", "Sign in", nil)
}

// doLogin exchanges an email and password for a browser session.
func (h *handler) doLogin(w http.ResponseWriter, r *http.Request) error {
	session, err := h.svc.Login(r.Context(), field(r, "email"), r.PostFormValue("password"))
	if err != nil {
		return err
	}
	http.SetCookie(w, auth.NewSessionCookie(session.Token, session.ExpiresAt, h.secure))
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// doLogout ends the session the browser presented.
func (h *handler) doLogout(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.Logout(r.Context()); err != nil && !core.IsKind(err, core.KindUnauthenticated) {
		return err
	}
	http.SetCookie(w, auth.ClearSessionCookie(h.secure))
	http.Redirect(w, r, RouteLogin, http.StatusSeeOther)
	return nil
}

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

// activityView is what the activity screen renders.
type activityView struct {
	Entries    []core.AuditEntry
	NextCursor string
}

// showActivity renders recent tenant activity, newest first.
func (h *handler) showActivity(w http.ResponseWriter, r *http.Request) error {
	filter := core.AuditFilter{Page: core.Page{
		Cursor:    r.URL.Query().Get("cursor"),
		Sort:      "seq",
		Direction: core.Descending,
		Limit:     50,
	}}
	entries, next, err := h.svc.ListAudit(r.Context(), filter)
	if err != nil {
		return err
	}
	return h.render(w, r, "activity.html", "Activity",
		activityView{Entries: entries, NextCursor: next})
}
