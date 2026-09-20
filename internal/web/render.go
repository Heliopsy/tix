package web

import (
	"errors"
	"fmt"
	"hash/fnv"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/output"
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

// accents is the palette a tenant's colour is chosen from, so the value placed
// in the stylesheet is always one this package wrote.
var accents = [][2]string{
	{"#3b5bdb", "#edf0ff"},
	{"#2b8a3e", "#e9f7ec"},
	{"#c2410c", "#fdf0e8"},
	{"#7048e8", "#f1ecfd"},
	{"#0b7285", "#e6f4f6"},
	{"#a61e4d", "#fbeaf0"},
}

// defaultBranding is applied when no tenant has been resolved.
func defaultBranding() branding {
	// #nosec G203 -- values come from the fixed accents palette, never a caller.
	return branding{Title: "tix", Monogram: "t",
		Accent: template.CSS(accents[0][0]), AccentSoft: template.CSS(accents[0][1])} // #nosec G203
}

// brandFor derives a tenant's branding from its own record.
func brandFor(t *core.Tenant) branding {
	if t == nil || strings.TrimSpace(t.Name) == "" {
		return defaultBranding()
	}
	sum := fnv.New32a()
	_, _ = sum.Write([]byte(t.Key + t.ID))
	pair := accents[int(sum.Sum32())%len(accents)]
	return branding{
		Title:    t.Name,
		Monogram: strings.ToUpper(t.Name[:1]),
		// Both values come from the fixed accents palette above, selected by
		// hash; no caller can place a value here.
		Accent:     template.CSS(pair[0]), // #nosec G203
		AccentSoft: template.CSS(pair[1]), // #nosec G203
	}
}

// view is what every template is executed against.
type view struct {
	Title      string
	Path       string
	Flash      string
	Error      string
	CSRF       string
	EventsPath string
	Brand      branding
	Actor      *core.Actor
	Data       any
}

// parseTemplates builds one template set per screen, so that two screens can
// define the same block without colliding.
func parseTemplates() map[string]*template.Template {
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
		out[name] = template.Must(template.New("layout.html").Funcs(funcs()).ParseFS(templateFS, files...))
	}
	return out
}

// funcs are the formatting helpers templates may call.
func funcs() template.FuncMap {
	return template.FuncMap{
		"compact": output.FormatCompact,
		"stamp":   output.FormatTimestampPtr,
		"join":    joinValues,
		"scopes":  joinScopes,
		"counts":  formatCounts,
	}
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
func (h *handler) newView(r *http.Request, title string, data any) view {
	actor, _ := core.ActorFrom(r.Context())
	return view{
		Title:      title,
		Path:       r.URL.Path,
		Flash:      r.URL.Query().Get("flash"),
		CSRF:       csrfFrom(r),
		EventsPath: h.eventsPath,
		Brand:      h.brand(r),
		Actor:      actor,
		Data:       data,
	}
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
	return brandFor(tenant)
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
	v := h.newView(r, title, data)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	return tmpl.Execute(w, v)
}

// errorView is what the error screen renders.
type errorView struct {
	Heading string
	Message string
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
	data := errorView{Heading: headingFor(kind), Message: message}
	if renderErr := h.renderStatus(w, r, kind.HTTPStatus(), "error.html", headingFor(kind), data); renderErr != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
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
