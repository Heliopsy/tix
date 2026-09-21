// Package web serves the tix browser interface from the single binary.
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

//go:embed templates
var templateFS embed.FS

//go:embed assets
var assetFS embed.FS

// DefaultEventsPath is the WebSocket route the live activity feed subscribes to.
const DefaultEventsPath = "/api/v1/events"

// Option configures the browser interface.
type Option func(*handler)

// WithSecureCookies marks the CSRF and session cookies Secure, which is correct
// whenever the server is reached over TLS.
func WithSecureCookies(secure bool) Option {
	return func(h *handler) { h.secure = secure }
}

// WithLogger sets the logger internal failures are reported to.
func WithLogger(l *slog.Logger) Option {
	return func(h *handler) {
		if l != nil {
			h.logger = l
		}
	}
}

// WithEventsPath sets the WebSocket path the live activity feed subscribes to.
func WithEventsPath(path string) Option {
	return func(h *handler) {
		if strings.TrimSpace(path) != "" {
			h.eventsPath = path
		}
	}
}

// WithTargetHint sets the resolved --db or --ctx target the sign-in screen's
// "can't sign in" disclosure appends to the CLI commands it shows. Without
// it, those commands are shown with no target flag, which is only correct
// for a process that resolved its own zero-config default. The caller
// decides whether to pass this at all: on a non-loopback listener it names
// the store on the wire to anyone who loads the login page, which is a
// judgement call for whoever starts the server, not this package.
func WithTargetHint(hint string) Option {
	return func(h *handler) { h.targetHint = strings.TrimSpace(hint) }
}

// WithTimeStyle sets the TimeStyle every screen renders its timestamps
// through. Without it a handler uses the zero-value TimeStyle, which reads
// the machine's local zone in the same layout output.time_format's "iso"
// default renders, matching the CLI's own unconfigured default.
//
// This is server-configured, not per user: the web is multi-user, and a
// tenant's members may sit in different zones, but the configuration key
// this reads describes a server setting, not a per-account one. A per-user
// version would need a place to store each actor's preferred format and
// zone (most naturally alongside the theme and key-scheme cookies this
// package already keeps per browser, or on core.Actor if it should survive
// a new device) and would resolve the TimeStyle per request from that
// instead of once at Handler construction.
func WithTimeStyle(style output.TimeStyle) Option {
	return func(h *handler) { h.style = style }
}

// handler serves every browser screen over the service.
type handler struct {
	svc        core.Service
	mux        *http.ServeMux
	templates  map[string]*template.Template
	assets     http.Handler
	logger     *slog.Logger
	secure     bool
	eventsPath string
	targetHint string
	style      output.TimeStyle
}

// Handler returns an http.Handler serving the browser interface.
func Handler(svc core.Service, opts ...Option) http.Handler {
	h := &handler{
		svc:        svc,
		mux:        http.NewServeMux(),
		logger:     slog.New(slog.NewTextHandler(discard{}, nil)),
		eventsPath: DefaultEventsPath,
	}
	for _, opt := range opts {
		opt(h)
	}
	h.templates = parseTemplates(h.style)
	h.assets = assetHandler()
	h.register()
	return h
}

// ServeHTTP serves one browser request.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// register binds the asset route and every screen route.
func (h *handler) register() {
	h.mux.Handle("GET /assets/", h.assets)
	for _, rt := range h.routes() {
		h.mux.Handle(rt.Method+" "+rt.Pattern, h.wrap(rt))
	}
}

// discard swallows log output when no logger is configured.
type discard struct{}

// Write reports every byte as written and keeps none of them.
func (discard) Write(p []byte) (int, error) { return len(p), nil }

// assetHandler serves the embedded stylesheet and scripts.
func assetHandler() http.Handler {
	sub, err := fs.Sub(assetFS, "assets")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/assets/", http.FileServer(http.FS(sub)))
}

// HasTemplate reports whether a template is embedded in the binary, so a
// binding can be checked against what is actually shipped.
func HasTemplate(name string) bool {
	_, err := templateFS.Open("templates/" + name)
	return err == nil
}
