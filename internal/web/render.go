// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// layoutFiles are parsed into every screen.
var layoutFiles = []string{"templates/layout.html", "templates/partials.html"}

// branding is the per-tenant identity applied to a page.
type branding struct {
	Title      string
	Monogram   string
	Accent     template.CSS
	AccentSoft template.CSS
	// The same pair for the dark and dim schemes. See layout.html: the scheme
	// rules in app.css are selector-specific (:root[data-theme="dark"]), so a
	// tenant accent emitted only as plain :root loses to them and the theme
	// silently does nothing on the scheme the browser defaults to.
	AccentDark     template.CSS
	AccentSoftDark template.CSS
}

// defaultAccent and defaultAccentSoft are the brand colour shown before any
// tenant has been resolved (the sign-in screen, most of all). They must stay
// equal to assets/app.css's :root --accent and --accent-soft: the inline
// style block below always overrides those tokens with one of these two
// values, so if they drifted apart the stylesheet's own default would never
// actually be seen by anyone.
const (
	defaultAccent     = "#0f766e"
	defaultAccentSoft = "#e6f4f1"
)

// defaultBranding is applied when no tenant has been resolved.
func defaultBranding() branding {
	// #nosec G203 -- fixed constants above, never a caller.
	d := builtInDefault()
	return branding{Title: "tix", Monogram: "t",
		Accent: template.CSS(defaultAccent), AccentSoft: template.CSS(defaultAccentSoft), // #nosec G203
		AccentDark: template.CSS(d.AccentDark), AccentSoftDark: template.CSS(d.AccentSoftDark)} // #nosec G203
}

// brandFor derives a tenant's branding from its own record and the theme it
// names.
//
// The palette moved to core so the terminal interface could read the same one:
// this package used to hash the tenant into six pairs of its own, which made a
// tenant one colour in a browser and something unrelated in a terminal. The
// hash still decides for a tenant that names no theme, but it lives in
// core.DerivedTheme now and both surfaces call it.
func brandFor(t *core.Tenant, themes *core.ThemeRegistry) branding {
	if t == nil || strings.TrimSpace(t.Name) == "" {
		return defaultBranding()
	}
	theme := themes.Resolve(t)
	return branding{
		Title:    t.Name,
		Monogram: strings.ToUpper(t.Name[:1]),
		// Every value core resolves has passed its hex validation, which is
		// the boundary that keeps a configuration file out of the stylesheet;
		// no caller can place a value here.
		Accent:     template.CSS(theme.Accent),     // #nosec G203
		AccentSoft: template.CSS(theme.AccentSoft), // #nosec G203
		// Empty would emit an empty custom property and blank the accent, so
		// a theme with no dark pair falls back to its light one.
		AccentDark:     template.CSS(orElse(theme.AccentDark, theme.Accent)),         // #nosec G203
		AccentSoftDark: template.CSS(orElse(theme.AccentSoftDark, theme.AccentSoft)), // #nosec G203
	}
}

// orElse is the fallback for a theme that names no dark colour.
func orElse(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// builtInDefault is the shipped default theme, for the branding shown before
// any tenant has been resolved.
func builtInDefault() core.Theme {
	r, _ := core.NewThemeRegistry(nil)
	return r.Default()
}

// view is what every template is executed against.
type view struct {
	Title         string
	Path          string
	Flash         string
	Error         string
	CSRF          string
	EventsPath    string
	Brand         branding
	Actor         *core.Actor
	Advanced      bool
	DragMove      bool
	Theme         string
	KeyScheme     string
	Schemes       []KeyScheme
	ShortcutsJSON template.JS
	Columns       columnPrefs
	ColumnPage    string
	Here          string
	Upgrade       upgrade
	Data          any
}

// parseTemplates builds one template set per screen, so that two screens can
// define the same block without colliding. Every timestamp a template renders
// goes through style, so the browser and the CLI table cannot disagree about
// what an instant means.
func parseTemplates(style output.TimeStyle) map[string]*template.Template {
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		panic(err)
	}
	out := make(map[string]*template.Template, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "layout.html" || name == "partials.html" {
			continue
		}
		files := append(append([]string{}, layoutFiles...), "templates/"+name)
		out[name] = template.Must(template.New("layout.html").Funcs(funcs(style)).ParseFS(templateFS, files...))
	}
	return out
}

// funcs are the formatting helpers templates may call, bound to the handler's
// configured TimeStyle.
func funcs(style output.TimeStyle) template.FuncMap {
	return template.FuncMap{
		"compact":         style.Format,
		"stamp":           style.FormatPtr,
		"absolute":        style.Absolute,
		"join":            joinValues,
		"scopes":          joinScopes,
		"counts":          formatCounts,
		"envName":         envName,
		"prio":            priorityName,
		"slug":            slug,
		"kindLabel":       componentKindLabel,
		"schemeLabel":     keySchemeLabel,
		"relativeAt":      style.Relative,
		"sentence":        sentenceFor,
		"sentenceSubject": sentenceForSubject,
		"percent":         barPercent,
		"age":             humanDuration,
		"claim":           claimState,
		"expiredAgo":      expiredAgo,
		"category":        categoryLabel,
		"moves":           targetsFrom,
	}
}

// priorityName renders a priority as the word the CLI and the forms use, so a
// task does not read as a bare number on one surface and a name on another.
func priorityName(p core.Priority) string {
	for _, c := range priorityChoices {
		if c.Value == p {
			return c.Label
		}
	}
	return "normal"
}

// slug reduces a value to a css class suffix.
func slug(v any) string {
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		}
		return '-'
	}, fmt.Sprint(v))
	return out
}

// componentKindLabels names each component kind for display. Most of
// core.ComponentKind's values already read as English; the two that do not
// ("field_def", "project_template") get a label here rather than reaching a
// screen as a raw Go identifier.
var componentKindLabels = map[core.ComponentKind]string{
	core.ComponentWorkflow: "workflow",
	core.ComponentFieldDef: "field definition",
	core.ComponentTag:      "tag",
	core.ComponentProject:  "project template",
	core.ComponentWebhook:  "webhook",
}

// componentKindLabel renders a component kind as the words above, falling
// back to the raw value for a kind this build does not know, so an unknown
// kind is visible rather than silently blank.
func componentKindLabel(k core.ComponentKind) string {
	if label, ok := componentKindLabels[k]; ok {
		return label
	}
	return string(k)
}

// joinValues renders a list of strings as a comma separated line.
func joinValues(values []string) string { return strings.Join(values, ", ") }

// joinScopes renders a token's scopes as a comma separated line.
func joinScopes(values []core.Scope) string {
	out := make([]string, 0, len(values))
	for _, s := range values {
		out = append(out, string(s))
	}
	return strings.Join(out, ", ")
}

// formatCounts renders an import tally in a stable order.
func formatCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, counts[k]))
	}
	return strings.Join(parts, ", ")
}

// newView assembles the common part of every page.
func (h *handler) newView(r *http.Request, name, title string, data any) view {
	actor, _ := core.ActorFrom(r.Context())
	scheme := keySchemeOf(r)
	return view{
		Title:         title,
		Path:          r.URL.Path,
		Here:          here(r),
		Upgrade:       h.releases.state(),
		ColumnPage:    columnPage(name),
		Columns:       columnsOf(r),
		Flash:         r.URL.Query().Get("flash"),
		CSRF:          csrfFrom(r),
		EventsPath:    h.eventsPath,
		Brand:         h.brand(r),
		Actor:         actor,
		Advanced:      advancedMode(r),
		DragMove:      dragMoveMode(r),
		Theme:         themeOf(r),
		KeyScheme:     string(scheme),
		Schemes:       KeySchemes(),
		ShortcutsJSON: shortcutsJSON(scheme),
		Data:          data,
	}
}

// here is the page a preference form returns to, filters and page position
// intact, minus the flash a previous redirect left behind.
func here(r *http.Request) string {
	query := r.URL.Query()
	query.Del("flash")
	if len(query) == 0 {
		return r.URL.Path
	}
	return r.URL.Path + "?" + query.Encode()
}

// AdvancedCookie remembers what this browser has said about the configuration
// and data screens. It has three states, not two: "1" shown, "0" hidden, and
// absent, which leaves the answer to who is reading.
const AdvancedCookie = "tix_advanced"

// advancedChoice is what a browser has said about the configuration screens.
type advancedChoice int

const (
	advancedUnset advancedChoice = iota
	advancedOn
	advancedOff
)

// advancedChoiceOf reads the preference this browser has recorded.
func advancedChoiceOf(r *http.Request) advancedChoice {
	c, err := r.Cookie(AdvancedCookie)
	if err != nil {
		return advancedUnset
	}
	switch c.Value {
	case "1":
		return advancedOn
	case "0":
		return advancedOff
	}
	return advancedUnset
}

// advancedPaths are the screens the Configure and Data groups lead to. Being
// on one of them expands its group whatever the preference says, so a reader
// is never on a page the navigation beside them denies exists.
var advancedPaths = []string{
	RouteWorkflows, RouteTransfer, RouteBundles, RouteSync, "/admin/",
}

// onAdvancedPath reports whether this request is for one of those screens.
func onAdvancedPath(path string) bool {
	for _, prefix := range advancedPaths {
		if path == prefix || strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/") {
			return true
		}
	}
	return false
}

// advancedMode reports whether this page shows the configuration and data
// groups in its navigation.
//
// The default follows the reader rather than being off for everybody. A
// tenant administrator arriving for the first time could not find the tenant
// screen at all: it was reachable by typing its URL and by no other means,
// because the only entry to it lived behind a preference on a settings page
// they had no reason to open. Anybody else keeps the simple view, since every
// screen in those groups refuses them, and an entry that leads to a refusal is
// worse than no entry.
//
// The preference still decides whenever it has been set, in either direction,
// so an administrator who wants the shorter navigation gets it and keeps it.
func advancedMode(r *http.Request) bool {
	if onAdvancedPath(r.URL.Path) {
		return true
	}
	switch advancedChoiceOf(r) {
	case advancedOn:
		return true
	case advancedOff:
		return false
	}
	actor, ok := core.ActorFrom(r.Context())
	return ok && actor != nil && actor.HasScope(core.ScopeTenantAdmin)
}

// DragMoveCookie remembers whether this browser wants drag-and-drop on the
// board, on top of the per-card Move disclosure that is always there. It
// defaults on, the opposite polarity of AdvancedCookie: most people who can
// drag a card would rather drag it than open a disclosure and pick from a
// select, and the people for whom that never works (no pointer, or no
// pointer gesture) are unaffected either way since the Move disclosure never
// leaves.
const DragMoveCookie = "tix_drag_move"

// dragMoveMode reports whether this browser wants drag-and-drop on the
// board. Unset, or any value other than "0", means on.
func dragMoveMode(r *http.Request) bool {
	c, err := r.Cookie(DragMoveCookie)
	return err != nil || c.Value != "0"
}

// ThemeCookie remembers the colour scheme this browser asked for.
const ThemeCookie = "tix_theme"

// Themes are the schemes on offer. An empty value follows the system, and
// "dim" is a low contrast scheme for people who find the default too stark.
var Themes = []string{"", "light", "dark", "dim"}

// themeOf reports the scheme this browser asked for, or an empty string when
// it has not asked and the system preference should decide.
func themeOf(r *http.Request) string {
	c, err := r.Cookie(ThemeCookie)
	if err != nil {
		return ""
	}
	for _, t := range Themes {
		if t != "" && c.Value == t {
			return t
		}
	}
	return ""
}

// TimeFormatCookie remembers the timestamp layout this browser asked for.
const TimeFormatCookie = "tix_time_format"

// TimezoneCookie remembers the zone this browser asked for.
const TimezoneCookie = "tix_timezone"

// Timezones are the zones on offer. An empty value follows the deployment's
// own output.timezone. The list is a selection rather than every name the
// system zone database carries: a select of six hundred entries is not a
// control anybody can use, and a fixed list bounds how many template sets a
// process can be made to parse.
var Timezones = []string{
	"", "UTC",
	"Pacific/Auckland", "Australia/Sydney", "Australia/Perth",
	"Asia/Tokyo", "Asia/Shanghai", "Asia/Singapore", "Asia/Bangkok",
	"Asia/Kolkata", "Asia/Karachi", "Asia/Dubai", "Asia/Jerusalem",
	"Africa/Cairo", "Africa/Johannesburg", "Africa/Lagos",
	"Europe/Moscow", "Europe/Sofia", "Europe/Berlin", "Europe/Madrid",
	"Europe/London", "America/Sao_Paulo", "America/New_York",
	"America/Chicago", "America/Denver", "America/Los_Angeles",
}

// timeFormatOf reports the layout this browser asked for, or an empty string
// when it has not asked and the deployment's configuration should decide.
func timeFormatOf(r *http.Request) string {
	c, err := r.Cookie(TimeFormatCookie)
	if err != nil {
		return ""
	}
	return resolvedTimeFormat(c.Value)
}

// resolvedTimeFormat returns want when it names a layout this build renders,
// and an empty string otherwise, so a value typed by hand into the cookie can
// never reach the renderer.
func resolvedTimeFormat(want string) string {
	for _, f := range output.TimeFormats {
		if want == f {
			return f
		}
	}
	return ""
}

// timezoneOf reports the zone this browser asked for, or an empty string when
// it has not asked and the deployment's configuration should decide.
func timezoneOf(r *http.Request) string {
	c, err := r.Cookie(TimezoneCookie)
	if err != nil {
		return ""
	}
	return resolvedZone(c.Value)
}

// resolvedZone returns want when it is on offer and this system's zone
// database can load it. A name this system does not know falls back to the
// deployment default rather than failing the page: tzdata is a property of the
// host, not of the person reading, and a missing zone must not take a screen
// down.
func resolvedZone(want string) string {
	if want == "" {
		return ""
	}
	for _, z := range Timezones {
		if z == "" || z != want {
			continue
		}
		if _, err := time.LoadLocation(z); err != nil {
			return ""
		}
		return z
	}
	return ""
}

// styleKey identifies one resolved way of rendering an instant.
type styleKey struct{ format, zone string }

// styleTemplates caches one parsed template set per style a browser has asked
// for.
//
// The formatting helpers are bound into a template at parse time, so a
// per-viewer zone cannot be a value handed to Execute; it has to be a set
// parsed against that viewer's style. A set is immutable once parsed and is
// only ever executed under the key it was parsed for, so no request can be
// served another's preference, and Timezones above bounds how many sets exist.
var styleTemplates sync.Map

// templatesFor returns the template set this request renders through: the
// deployment's own when the browser has chosen neither preference, and one
// parsed for the chosen style otherwise.
//
// Choosing one of the two leaves the other at output's neutral default, which
// is what NewTimeStyle reads an empty value as, because a TimeStyle does not
// give its format and zone back and this package therefore cannot compose a
// half-overridden one. Those neutral values are the shipped configuration
// defaults, so only a deployment that moved one of them away sees a
// difference.
func (h *handler) templatesFor(r *http.Request) map[string]*template.Template {
	if r == nil {
		return h.templates
	}
	key := styleKey{format: timeFormatOf(r), zone: timezoneOf(r)}
	if key.format == "" && key.zone == "" {
		return h.templates
	}
	if set, ok := styleTemplates.Load(key); ok {
		return set.(map[string]*template.Template)
	}
	style, err := output.NewTimeStyle(key.format, key.zone)
	if err != nil {
		// Both halves of the key were validated on the way in, so reaching
		// here means the host's zone database moved under a running process.
		// The fallback keeps the screen up, but it serves a zone nobody chose,
		// which is the kind of wrong that has to be visible to an operator.
		h.logger.Error("rendering in the deployment default", "format", key.format,
			"zone", key.zone, "error", err)
		return h.templates
	}
	set, _ := styleTemplates.LoadOrStore(key, parseTemplates(style))
	return set.(map[string]*template.Template)
}

// brand resolves the signed-in tenant's branding, falling back to the default
// when the caller may not read the tenant record.
func (h *handler) brand(r *http.Request) branding {
	if _, ok := core.ActorFrom(r.Context()); !ok {
		return defaultBranding()
	}
	tenant, err := h.svc.GetTenant(r.Context(), "")
	if err != nil {
		return defaultBranding()
	}
	return brandFor(tenant, h.themes)
}

// render writes one screen.
func (h *handler) render(w http.ResponseWriter, r *http.Request, name, title string, data any) error {
	return h.renderStatus(w, r, http.StatusOK, name, title, data)
}

// renderStatus writes one screen with an explicit status.
func (h *handler) renderStatus(w http.ResponseWriter, r *http.Request, status int, name, title string, data any) error {
	tmpl, ok := h.templatesFor(r)[name]
	if !ok {
		return core.Internal("no template named %q", name)
	}
	v := h.newView(r, name, title, data)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	return tmpl.Execute(w, v)
}

// errorView is what the error screen renders.
type errorView struct {
	Heading string
	Message string
	Hint    string
	Back    string
}

// fail renders the error screen, disclosing the domain message only. An
// internal failure is reported generically, so no query text or stack trace
// can reach a browser.
func (h *handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	kind := core.KindOf(err)
	message := safeMessage(err, kind)
	if kind == core.KindInternal {
		h.logger.Error("serving page", "path", r.URL.Path, "error", err)
	}
	data := errorView{Heading: headingFor(kind), Message: message,
		Hint: hintFor(kind), Back: backFrom(r)}
	if renderErr := h.renderStatus(w, r, kind.HTTPStatus(), "error.html", headingFor(kind), data); renderErr != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// hintFor says what to do next, for the failures where the answer is not
// obvious from the message. A refused save is the one that needs it: two forms
// on a screen carry the same version, so the second submission is refused and
// the reader has to be told that reloading is the way out.
func hintFor(kind core.Kind) string {
	switch kind {
	case core.KindConflict, core.KindLeaseExpired:
		return "Somebody, or an agent, changed this while the page was open. Reload it and apply your change to the current version."
	case core.KindForbidden:
		return "Ask an administrator of this tenant for the scope the message names."
	default:
		return ""
	}
}

// backFrom is the screen a failed mutation came from, so the reader returns to
// their work rather than to the dashboard.
func backFrom(r *http.Request) string {
	if r.Method != http.MethodPost {
		return RouteRoot
	}
	return safeNext(r.URL.Path)
}

// safeMessage returns the text an error may disclose to a browser.
func safeMessage(err error, kind core.Kind) string {
	if kind == core.KindInternal {
		return "internal error"
	}
	var domain *core.Error
	if errors.As(err, &domain) {
		return domain.Message
	}
	return "internal error"
}

// headingFor names the failure in words an operator can act on.
func headingFor(kind core.Kind) string {
	switch kind {
	case core.KindNotFound, core.KindNoTaskAvailable:
		return "Not found"
	case core.KindUnauthenticated:
		return "Sign in required"
	case core.KindForbidden:
		return "Not permitted"
	case core.KindInvalid:
		return "That request was not valid"
	case core.KindConflict, core.KindLeaseExpired:
		return "That change conflicts"
	case core.KindPrecondition:
		return "That change is not allowed here"
	default:
		return "Something went wrong"
	}
}

// redirect answers a mutation with a see-other and a message for the next page.
// redirectTo is redirect with a fragment, so the browser restores the reader's
// place instead of dropping them at the top of the page.
//
// A fragment rather than a scroll script: it needs no JavaScript, it survives
// the back button, and the browser already knows how to do it.
func redirectTo(w http.ResponseWriter, r *http.Request, path, fragment, flash string) {
	target := path
	if flash != "" {
		sep := "?"
		if strings.Contains(target, "?") {
			sep = "&"
		}
		target += sep + "flash=" + url.QueryEscape(flash)
	}
	if fragment != "" {
		target += "#" + url.PathEscape(fragment)
	}
	// #nosec G710 -- path has been through safeNext, the flash is
	// query-escaped and the fragment is path-escaped.
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func redirect(w http.ResponseWriter, r *http.Request, path, flash string) {
	target := path
	if flash != "" {
		target += "?flash=" + url.QueryEscape(flash)
	}
	// #nosec G710 -- path is a route constant chosen by the handler, and the
	// flash text is query-escaped; neither is a caller-supplied destination.
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// barPercent is one bar's width as a percentage of the tallest.
//
// It clamps rather than trusting arithmetic to stay in range: the value goes
// straight into a style attribute, and a width over 100 would push a bar
// outside its track for whatever rounding produced it. A zero maximum returns
// zero instead of dividing, which is the empty-window case.
func barPercent(v, max int) int {
	if max <= 0 || v <= 0 {
		return 0
	}
	p := v * 100 / max
	if p > 100 {
		return 100
	}
	// A count that is present but tiny still gets a sliver, so "one completed"
	// does not render as an empty row indistinguishable from none.
	if p < 2 {
		return 2
	}
	return p
}

// humanDuration renders a span the way someone reading a board says it.
//
// core.Duration prints Go's own syntax, which carries every digit it has:
// a lead time came out as "936h11m23.133303457s", which overflowed its card
// and collided with the next one. Nobody reading "how long did this take"
// wants nanoseconds, and a figure that does not fit its box is worse than a
// rounder one.
//
// Two units at most, largest first, because "15d 8h" answers the question and
// "15d 8h 36m 42s" makes the reader do the rounding themselves.
// claimState reports how a task's lease reads on a listing: held while it is
// live, expired once a claim has run out, and empty when nobody has held it
// recently.
//
// Expired used to be read off the lease columns still being populated past
// their time, which made the badge all but unreachable: the sweeper clears
// those columns within a minute of the lease lapsing, and from then on the
// task looked exactly like one nobody had ever touched. The durable evidence
// the sweeper now leaves behind (core.Task.LeaseExpiredAt, and
// core.Task.ClaimExpiredRecently over core.LeaseExpiryEvidenceWindow) is what
// this reads instead, so a claim that lapsed overnight is still visible in
// the morning whether or not anything has swept it.
func claimState(t core.Task) string {
	now := time.Now()
	// A claim with no lease at all is held until something releases it: there
	// is no time at which it lapses, so it can never become evidence.
	if t.ClaimedByActorID != "" && t.LeaseExpiresAt == nil {
		return "held"
	}
	if t.ClaimedAtTime(now) {
		return "held"
	}
	if t.ClaimExpiredRecently(now) {
		return "expired"
	}
	return ""
}

// expiredAgo renders how long ago a task's last claim lapsed, for the detail
// beside the badge. Empty when nothing has lapsed, so a template can ask
// without checking first.
func expiredAgo(t core.Task) string {
	if t.LeaseExpiredAt == nil {
		return ""
	}
	return humanDuration(core.Duration(time.Since(*t.LeaseExpiredAt)))
}

// categoryLabels names each workflow state category in words.
//
// The value is the contract: core.CategoryInProgress is "in_progress" on the
// wire, in the store and in every filter, and renaming it would break all
// three. What a statistics screen showed, though, was that identifier, so
// "Where the work is" answered with a Go constant. The label belongs here,
// at the point of presentation, and nowhere nearer the data.
var categoryLabels = map[core.StateCategory]string{
	core.CategoryTodo:       "To do",
	core.CategoryInProgress: "In progress",
	core.CategoryDone:       "Done",
}

// categoryLabel renders one state category for a reader, falling back to the
// raw value so a category this build does not know is visible rather than
// blank.
func categoryLabel(c core.StateCategory) string {
	if label, ok := categoryLabels[c]; ok {
		return label
	}
	return string(c)
}

// humanDuration defers to the type, which is where the rendering lives so the
// three surfaces showing these figures cannot drift apart on them.
func humanDuration(d core.Duration) string { return d.Human() }
