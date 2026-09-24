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
	return branding{Title: "tix", Monogram: "t",
		Accent: template.CSS(defaultAccent), AccentSoft: template.CSS(defaultAccentSoft)} // #nosec G203
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
	}
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

// AdvancedCookie remembers whether this browser wants the administrative
// screens. The simple view is the default: most people are here to work
// through a list, not to configure domains and tokens.
const AdvancedCookie = "tix_advanced"

// advancedMode reports whether this browser asked for the full interface.
func advancedMode(r *http.Request) bool {
	c, err := r.Cookie(AdvancedCookie)
	return err == nil && c.Value == "1"
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
	tmpl, ok := h.templates[name]
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
