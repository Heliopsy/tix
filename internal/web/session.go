// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// sessionRoutes are the sign-in, sign-out and landing screens, plus the
// per-browser display preferences.
func (h *handler) sessionRoutes() []route {
	return []route{
		get(RouteRoot, "", h.showRoot, "WhoAmI"),
		public(get(RouteLogin, "login.html", h.showLogin)),
		public(post(RouteLogin, h.doLogin, "Login")),
		post(RouteLogout, h.doLogout, "Logout"),
		get(RouteSettings, "settings.html", h.showSettings, "WhoAmI", "TOTPStatus", "GetCalendarFeed", "GetInboxPrefs", "ListInboxChannels"),
		get(RouteChannels, "settings.html", h.showSettings, "WhoAmI", "TOTPStatus", "GetCalendarFeed", "GetInboxPrefs", "ListInboxChannels"),
		get(RouteRelease, "release.html", h.showRelease, "WhoAmI"),
		post(RouteAdvanced, h.toggleAdvanced, "Logout"),
		post(RouteTheme, h.setTheme, "Logout"),
		post(RouteKeyScheme, h.setKeyScheme, "Logout"),
		post(RouteTimeFormat, h.setTimeFormat, "Logout"),
		post(RouteTimezone, h.setTimezone, "Logout"),
		post(RouteColumns, h.setColumns, "Logout"),
		post(RouteVisibility, h.setVisibility, "Logout"),
		post(RouteTaskView, h.setTaskView, "Logout"),
		post(RouteListGroup, h.setListGroup, "Logout"),
		post(RouteTint, h.setTint, "Logout"),
		post(RouteDragMove, h.toggleDragMove, "Logout"),
		post(RouteOpenAfterAdd, h.toggleOpenAfterAdd, "Logout"),
		post(RouteHover, h.toggleHover, "Logout"),
		post(RouteTaskOpen, h.toggleTaskOpen, "Logout"),
		post(RouteTitleClick, h.toggleTitleClick, "Logout"),
		post(RouteLanguage, h.setLanguage, "Logout"),
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
	// Next is where the visitor was going when they were bounced here. It is
	// carried in the form so a deep link survives sign-in, second step and
	// all.
	Next string
}

// showLogin renders the sign-in screen.
func (h *handler) showLogin(w http.ResponseWriter, r *http.Request) error {
	if _, ok := core.ActorFrom(r.Context()); ok {
		http.Redirect(w, r, RouteProjects, http.StatusSeeOther)
		return nil
	}
	return h.render(w, r, "login.html", "Sign in",
		loginView{TargetHint: h.targetHint, Next: safeNext(r.URL.Query().Get("next"))})
}

// doLogin is both steps of a browser sign-in: the password, and the second
// factor when the tenant requires one or the account holds one.
//
// One route serves both because the second step is the same screen posting
// back with the ticket the first step returned. The ticket proves only that
// the password was accepted a moment ago, so carrying it in a hidden field
// grants nothing on its own, and no session exists until the code is taken.
func (h *handler) doLogin(w http.ResponseWriter, r *http.Request) error {
	next := safeNext(field(r, "next"))
	ticket := field(r, "ticket")
	in := core.LoginInput{Surface: core.SurfaceWeb}
	if ticket != "" {
		in.Ticket, in.Code = ticket, field(r, "code")
	} else {
		in.Email, in.Password = field(r, "email"), r.PostFormValue("password")
	}

	result, err := h.svc.Login(r.Context(), in)
	if err != nil {
		if ticket == "" {
			return err
		}
		// A refused code redraws the step it was typed on, refusal and all.
		// The error screen would lose the ticket and send the reader back to
		// the password, and an expired ticket says so on this screen beside
		// the way to start again.
		return h.renderTOTP(w, r, core.KindOf(err).HTTPStatus(),
			signInChallenge(ticket, next, field(r, "uri"), totpRefusal(err)))
	}
	if challenge := result.Challenge; challenge != nil {
		uri := ""
		if challenge.Enrolment != nil {
			uri = challenge.Enrolment.URI
		}
		return h.renderTOTP(w, r, http.StatusOK, signInChallenge(challenge.Ticket, next, uri, nil))
	}

	session := result.Session
	http.SetCookie(w, auth.NewSessionCookie(session.Token, session.ExpiresAt, h.secureCookie(r)))
	if len(result.RecoveryCodes) > 0 {
		// A sign-in that completed by enrolling issues the codes, and nothing
		// can produce them again, so they are shown before the reader goes on
		// to where they were headed.
		return h.renderRecoveryCodes(w, r, result.RecoveryCodes, next,
			"You are signed in. Keep these codes somewhere safe: each one signs you in "+
				"once if you lose the authenticator, and they are not shown again.")
	}
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, next, http.StatusSeeOther)
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
	TimeFormat     string
	Timezone       string
	TimeFormats    []timeChoice
	Timezones      []timeChoice

	// Prefs holds the three preferences whose controls also sit on the screen
	// that uses them: the column picker of every listing, the project
	// visibility choice and the view switch. They are rendered here from the
	// same constructor and the same partials those screens use, which is what
	// makes it impossible for this screen and that one to show different
	// answers for one cookie, including whether the project set behind the
	// visibility control is the whole tenant. See prefs.go.
	Prefs prefsView

	// TOTP is the reader's own second factor: whether they hold one, how many
	// recovery codes are left, and whether this tenant requires it here.
	TOTP totpSection

	// Calendar is the reader's own feed of due dates.
	Calendar calendarView
	// InboxKinds are the switches that mute a kind of inbox item.
	InboxKinds []inboxKindChoice

	// Notifications is the reader's own channels and the form adding one.
	Notifications notificationsSection
}

// showSettings renders the settings screen: every per-browser display
// preference, grouped and explained, replacing the sidebar disclosure that
// used to hold the same controls. Keeping both would let them drift, so the
// sidebar now only links here.
func (h *handler) showSettings(w http.ResponseWriter, r *http.Request) error {
	calendar, err := h.calendarFor(r)
	if err != nil {
		return err
	}
	return h.renderSettings(w, r, http.StatusOK, calendar, h.notifications(r, channelForm{}, nil))
}

// renderSettings draws the settings screen with the given calendar and
// Notifications sections, which a refused form replaces with what it refused.
func (h *handler) renderSettings(w http.ResponseWriter, r *http.Request, status int, calendar calendarView, notes notificationsSection) error {
	zone := timezoneOf(r)
	// The same walk the task screen's visibility control is built from, so
	// the two controls are offered one set of projects rather than one a page
	// deep and one whole.
	projects, whole, err := h.allProjects(r, false)
	if err != nil {
		return err
	}
	totp, err := h.totpFor(r)
	if err != nil {
		return err
	}
	return h.renderStatus(w, r, status, "settings.html", "Settings", settingsView{
		TargetDescribe: h.targetDescribe,
		Shortcuts:      buildShortcutTable(),
		TimeFormat:     timeFormatOf(r),
		Timezone:       zone,
		TimeFormats:    timeFormatChoices(zone),
		Timezones:      timezoneChoices(),
		Prefs:          settingsPrefs(h.prefsFor(r, projects, whole)),
		TOTP:           totp,
		Calendar:       calendar,
		InboxKinds:     inboxKindChoices(h.inboxPrefs(r)),
		Notifications:  notes,
	})
}

// settingsPrefs points the shared preference controls back at the Listings
// section with a toast, as every other form on the settings screen is. The
// controls are the ones the task screen renders too; only where they return
// to differs.
func settingsPrefs(p prefsView) prefsView {
	p.TaskView.Here = savedNext("Task view", "listings")
	p.ListGroup.Here = savedNext("List groups", "listings")
	p.Projects.Here = savedNext("Project choice", "listings")
	for i := range p.Columns {
		p.Columns[i].Here = savedNext("Columns on "+p.Columns[i].Label, "listings")
	}
	return p
}

// showRelease renders what the newest release says about itself.
//
// The footer has said "v0.20.0 is available" for a while and left the reader
// with a version number and nowhere to go: deciding whether to upgrade a task
// tracker means knowing what changed, and the only way to find out was to
// leave and search for it. This is that page.
//
// The notes are fetched by the same six-hourly check the footer already runs,
// so opening this waits on nothing and an install with no route out simply
// finds it empty, which is the same state in which the footer offers no link
// to it.
func (h *handler) showRelease(w http.ResponseWriter, r *http.Request) error {
	return h.render(w, r, "release.html", "Release", releaseView{})
}

// releaseView carries nothing of its own. Every figure on the screen is in
// Upgrade, which render attaches to every page for the footer, and a second
// copy here would be a second thing to keep true.
type releaseView struct{}

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
	// The cookie records a decision either way, because the default now
	// depends on who is reading: an empty value would put an administrator
	// who hid the groups straight back to having them.
	value := "1"
	if advancedMode(r) {
		value = "0"
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

// toggleOpenAfterAdd decides whether adding a task leaves the listing.
//
// Default off. Both states are written explicitly, "1" and "0", so a stored
// choice never depends on how an absent cookie is read.
func (h *handler) toggleOpenAfterAdd(w http.ResponseWriter, r *http.Request) error {
	value := "1"
	if openAfterAdd(r) {
		value = "0"
	}
	// #nosec G124 -- a display preference, readable by no script.
	http.SetCookie(w, &http.Cookie{
		Name: OpenAfterAddCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: cookieYear,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// toggleHover turns the pointer's hover highlight on or off for this browser.
// Default on, with the polarity of Drag to move: "0" is the only value that
// turns it off.
func (h *handler) toggleHover(w http.ResponseWriter, r *http.Request) error {
	value := "0"
	if !hoverEffects(r) {
		value = ""
	}
	// #nosec G124 -- a display preference, readable by no script.
	http.SetCookie(w, &http.Cookie{
		Name: HoverCookie, Value: value, Path: "/",
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

// Route patterns for the two date preferences. They belong with the rest of
// the patterns, and move there whenever routes.go is next edited.
const (
	RouteTimeFormat = "/timeformat"
	RouteTimezone   = "/timezone"
)

// setTimeFormat records the timestamp layout this browser wants, following the
// pattern setTheme and setKeyScheme use: a cookie, not a tenant setting,
// because two people sharing a tenant read a date in different conventions
// just as they type in different shortcuts. A layout this build does not
// render is stored as the empty value, which means the deployment's own.
func (h *handler) setTimeFormat(w http.ResponseWriter, r *http.Request) error {
	h.setDisplayCookie(w, r, TimeFormatCookie, resolvedTimeFormat(field(r, "time_format")))
	return nil
}

// setTimezone records the zone this browser wants. A name that is not on offer,
// or that this system's zone database cannot load, is stored as the empty
// value, so a timestamp is never rendered in a zone nobody chose.
func (h *handler) setTimezone(w http.ResponseWriter, r *http.Request) error {
	h.setDisplayCookie(w, r, TimezoneCookie, resolvedZone(field(r, "timezone")))
	return nil
}

// setDisplayCookie stores one already-validated display preference and returns
// the browser to the page the form named.
func (h *handler) setDisplayCookie(w http.ResponseWriter, r *http.Request, name, value string) {
	// #nosec G124 -- a display preference, readable by no script.
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: cookieYear,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
}

// timeChoice is one option in a date preference select.
type timeChoice struct {
	Value string
	Label string
}

// timeFormatNames are the words a layout is offered under, since the
// configuration keys ("iso", "rfc3339") are not how anybody reads them.
var timeFormatNames = map[string]string{
	output.TimeISO:      "ISO",
	output.TimeRFC3339:  "RFC 3339",
	output.TimeShort:    "Short",
	output.TimeUS:       "US",
	output.TimeRelative: "Relative",
}

// timeFormatExample is the instant every absolute layout is demonstrated with.
var timeFormatExample = time.Date(2026, time.September, 21, 14, 5, 9, 0, time.UTC)

// timeFormatChoices offers every layout with an example rendered by the
// renderer itself, in the zone this browser reads in, so an example can never
// claim a layout the screens do not actually produce.
func timeFormatChoices(zone string) []timeChoice {
	out := make([]timeChoice, 0, len(output.TimeFormats)+1)
	out = append(out, timeChoice{Value: "", Label: "Server default"})
	for _, name := range output.TimeFormats {
		style, err := output.NewTimeStyle(name, zone)
		if err != nil {
			continue
		}
		sample := timeFormatExample
		if name == output.TimeRelative {
			sample = time.Now().Add(-18 * time.Minute)
		}
		out = append(out, timeChoice{Value: name,
			Label: timeFormatNames[name] + " (" + style.Format(sample) + ")"})
	}
	return out
}

// timezoneChoices offers every zone this system can load, so a host without a
// zone database offers only the deployment default rather than names that
// would silently fall back to it.
func timezoneChoices() []timeChoice {
	out := make([]timeChoice, 0, len(Timezones))
	for _, zone := range Timezones {
		if zone == "" {
			out = append(out, timeChoice{Value: "", Label: "Server default"})
			continue
		}
		if _, err := time.LoadLocation(zone); err != nil {
			continue
		}
		out = append(out, timeChoice{Value: zone, Label: strings.ReplaceAll(zone, "_", " ")})
	}
	return out
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
