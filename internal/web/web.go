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

// handler serves every browser screen over the service.
type handler struct {
	svc        core.Service
	mux        *http.ServeMux
	templates  map[string]*template.Template
	assets     http.Handler
	logger     *slog.Logger
	secure     bool
	eventsPath string
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
	h.templates = parseTemplates()
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
