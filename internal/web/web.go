// SPDX-License-Identifier: AGPL-3.0-or-later

// Package web serves the tix browser interface from the single binary.
package web

import (
	"embed"
	"github.com/heliopsy/tix/internal/httpapi"
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

// WithSecureCookies forces the CSRF and session cookies Secure regardless of
// how a request arrived. It is a floor, not the whole rule: a server holding
// its own certificate sets it, and a server behind a trusted proxy derives the
// same answer per request from the forwarded scheme instead.
func WithSecureCookies(secure bool) Option {
	return func(h *handler) { h.secure = secure }
}

// secureCookie reports whether a cookie set for this request must be Secure.
//
// Deriving it from a TLS certificate alone was wrong in the deployment the
// documentation recommends: behind a reverse proxy tix speaks plain HTTP, so
// every session and CSRF cookie went out without the flag even though the
// browser was on https. The effective scheme is resolved once per request by
// the API router's forwarded-header middleware, which believes those headers
// only when the immediate peer is a configured trusted proxy.
func (h *handler) secureCookie(r *http.Request) bool {
	if h.secure {
		return true
	}
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return httpapi.SchemeFrom(r.Context()) == "https"
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

// WithTargetDescribe sets the redacted, human-readable description of the
// database or server this process is talking to, shown on the settings page
// so an operator running several instances can tell them apart. Unlike
// WithTargetHint, whose value is pasted verbatim into the CLI commands the
// sign-in screen's "can't sign in" disclosure shows, this string is meant for
// broad display: pass internal/connect's Target.Describe, which already
// strips any password from a DSN before producing it, never a raw DSN or
// server URL. Without it the settings page names no target, which is the
// correct fallback for a build that has no notion of connect.Target at all.
func WithTargetDescribe(describe string) Option {
	return func(h *handler) { h.targetDescribe = strings.TrimSpace(describe) }
}

// WithThemes supplies the registry a tenant's accent is resolved from.
//
// Without it the handler falls back to the built-in themes, which is what the
// sign-in screen and any deployment that configures no palettes get.
func WithThemes(r *core.ThemeRegistry) Option {
	return func(h *handler) { h.themes = r }
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
	svc            core.Service
	mux            *http.ServeMux
	templates      map[string]*template.Template
	assets         http.Handler
	logger         *slog.Logger
	secure         bool
	eventsPath     string
	targetHint     string
	targetDescribe string
	style          output.TimeStyle
	releases       *releaseWatch
	themes         *core.ThemeRegistry
}

// Handler returns an http.Handler serving the browser interface.
func Handler(svc core.Service, opts ...Option) http.Handler {
	h := &handler{
		releases:   newReleaseWatch(),
		svc:        svc,
		mux:        http.NewServeMux(),
		logger:     slog.New(slog.NewTextHandler(discard{}, nil)),
		eventsPath: DefaultEventsPath,
	}
	for _, opt := range opts {
		opt(h)
	}
	if h.themes == nil {
		// A registry of built-ins only. NewThemeRegistry(nil) cannot fail:
		// the built-ins are constants that this package's own tests validate.
		h.themes, _ = core.NewThemeRegistry(nil)
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
