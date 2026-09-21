package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// sessionRoutes are the sign-in, sign-out, landing and activity screens.
func (h *handler) sessionRoutes() []route {
	return []route{
		get(RouteRoot, "", h.showRoot, "WhoAmI"),
		public(get(RouteLogin, "login.html", h.showLogin)),
		public(post(RouteLogin, h.doLogin, "Login")),
		post(RouteLogout, h.doLogout, "Logout"),
		post(RouteAdvanced, h.toggleAdvanced, "Logout"),
		post(RouteTheme, h.setTheme, "Logout"),
		post(RouteColumns, h.setColumns, "Logout"),
		post(RouteLists, h.setLists, "Logout"),
		get(RouteActivity, "activity.html", h.showActivity, "ListAudit", "GetActor", "GetTask"),
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
		HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode,
		MaxAge: cookieYear,
	})
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
		HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode,
		MaxAge: cookieYear,
	})
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

// activityRow is one audit entry with the words the feed shows for it: the
// kind of change, and a subject a reader recognises.
type activityRow struct {
	Entry   core.AuditEntry
	Verb    string
	Subject string
}

// activityView is what the activity screen renders.
type activityView struct {
	Rows       []activityRow
	Names      actorNames
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
	data := activityView{Rows: make([]activityRow, 0, len(entries)), NextCursor: next}
	actors := make([]string, 0, len(entries))
	for _, entry := range entries {
		data.Rows = append(data.Rows, activityRow{Entry: entry,
			Verb: actionVerb(entry.Action), Subject: h.subjectLabel(r, entry)})
		actors = append(actors, entry.ActorID)
	}
	data.Names = h.resolveActors(r, actors...)
	return h.render(w, r, "activity.html", "Activity", data)
}

// subjectLabel names what an entry happened to. A task is shown by the
// reference people quote to each other; anything else keeps its identifier,
// abbreviated, because inventing a name for a record would only mislead.
func (h *handler) subjectLabel(r *http.Request, entry core.AuditEntry) string {
	if entry.SubjectType != "task" || entry.SubjectID == "" {
		return shortID(entry.SubjectID)
	}
	task, err := h.svc.GetTask(r.Context(), core.TaskRef{ID: entry.SubjectID})
	if err != nil || task == nil || task.Ref == "" {
		return shortID(entry.SubjectID)
	}
	return task.Ref
}
