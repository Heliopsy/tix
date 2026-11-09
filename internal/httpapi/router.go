// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/wire"
)

// Defaults for the limits every request is subject to.
const (
	DefaultMaxBodyBytes   = 1 << 20
	DefaultRequestTimeout = 30 * time.Second
)

// TenantResolver maps a hostname to the tenant that owns it.
type TenantResolver interface {
	ResolveDomain(ctx context.Context, hostname string) (*core.Tenant, error)
}

// DBProbe reports database reachability and the applied schema version.
type DBProbe interface {
	Health(ctx context.Context) error
	SchemaVersion(ctx context.Context) (int, error)
}

// Config assembles a Router.
type Config struct {
	Service       core.Service
	Authenticator auth.Authenticator
	Resolver      TenantResolver
	Logger        *slog.Logger

	// DefaultTenantID pins requests whose Host maps to no tenant. When it is
	// empty an unknown host is answered with not found.
	DefaultTenantID string

	MaxBodyBytes   int64
	RequestTimeout time.Duration

	// MaxImportBytes caps a project task import, which carries a file, in
	// place of MaxBodyBytes. ImportRoutes are further mux patterns that take
	// it, the browser's import upload among them.
	MaxImportBytes int64
	ImportRoutes   []string

	// SecureCookies forces the session and CSRF cookies Secure regardless of
	// the scheme a request arrived on. It is the floor, not the ceiling: a
	// request whose effective scheme is https gets a Secure cookie anyway.
	SecureCookies bool

	// CookieSecurity overrides how Secure is decided. The default derives it
	// from the effective scheme of each request. Its vocabulary is owned by
	// internal/config, which is where server.cookie_security is parsed and
	// validated; this package names those constants rather than restating them.
	CookieSecurity string

	// PublicURL is server.public_url, the address people reach the browser
	// interface at. Links built from a request on a host that serves several
	// tenants use it in place of the request's own host; see Origin.
	PublicURL string

	// TrustedProxies lists the proxy addresses, as IPs or CIDR blocks, whose
	// X-Forwarded-Proto and X-Forwarded-For are believed. Leaving it empty
	// believes neither header from anybody, because any client can send them.
	TrustedProxies []string

	Probe          DBProbe
	ExpectedSchema int

	// Idempotency keeps the answers of writes sent with an Idempotency-Key.
	// Left nil, the router uses its service when the service provides it.
	Idempotency Idempotency

	// EventHandler serves the WebSocket event route when it is supplied.
	EventHandler http.Handler

	// WebHandler serves the browser interface at the root when it is supplied.
	// It is registered last and only for paths the API does not claim, so an
	// API route can never be shadowed by a page.
	WebHandler http.Handler
}

// Router serves the tix REST API.
type Router struct {
	cfg       Config
	publicURL string
	proxies   *auth.ProxyPolicy
	mux       *routeMux
	handler   http.Handler
	// imports matches the routes that take MaxImportBytes; it serves nothing.
	imports *http.ServeMux
}

// routeMux is a ServeMux that remembers every pattern registered on it, so
// the routes the server answers can be compared with the ones it documents.
type routeMux struct {
	*http.ServeMux
	patterns []string
}

// Handle registers a handler and records its pattern.
func (m *routeMux) Handle(pattern string, h http.Handler) {
	m.patterns = append(m.patterns, pattern)
	m.ServeMux.Handle(pattern, h)
}

// HandleFunc registers a handler function and records its pattern.
func (m *routeMux) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
	m.Handle(pattern, http.HandlerFunc(h))
}

// Patterns returns every pattern registered on the router, in the order it
// was registered.
func (rt *Router) Patterns() []string { return slices.Clone(rt.mux.patterns) }

// New builds a Router over the configured service.
func New(cfg Config) (*Router, error) {
	if cfg.Service == nil {
		return nil, core.Invalid("an http router requires a service")
	}
	if cfg.Authenticator == nil {
		return nil, core.Invalid("an http router requires an authenticator")
	}
	if cfg.Resolver == nil {
		cfg.Resolver = cfg.Service
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if cfg.MaxImportBytes <= 0 {
		cfg.MaxImportBytes = config.DefaultMaxImportBytes
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = DefaultRequestTimeout
	}
	if cfg.Idempotency == nil {
		cfg.Idempotency, _ = cfg.Service.(Idempotency)
	}

	if cfg.CookieSecurity == "" {
		cfg.CookieSecurity = config.CookieSecurityAuto
	}
	if !slices.Contains(config.CookieSecurities, cfg.CookieSecurity) {
		return nil, core.Invalid("cookie security %q is not one of %s",
			cfg.CookieSecurity, strings.Join(config.CookieSecurities, ", "))
	}
	proxies, err := auth.NewProxyPolicy(cfg.TrustedProxies)
	if err != nil {
		return nil, err
	}

	rt := &Router{cfg: cfg, proxies: proxies, mux: &routeMux{ServeMux: http.NewServeMux()}, imports: http.NewServeMux(),
		publicURL: strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/")}
	for _, pattern := range append([]string{http.MethodPost + " " + wire.RouteProjectTaskImport}, cfg.ImportRoutes...) {
		rt.imports.Handle(pattern, http.NotFoundHandler())
	}
	rt.register()
	rt.handler = chain(rt.mux,
		rt.idempotent,
		rt.authenticate,
		rt.resolveTenant,
		rt.withTimeout,
		rt.limitBody,
		rt.envelopeMuxErrors,
		rt.logRequest,
		rt.withRequestID,
		rt.withForwarded,
		rt.recoverPanic,
	)
	return rt, nil
}

// ServeHTTP serves one request through the middleware chain.
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt.handler.ServeHTTP(w, r)
}

// chain wraps h with each middleware, the last one becoming the outermost.
func chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for _, m := range mw {
		h = m(h)
	}
	return h
}

// register binds every route constant except the event stream, which the
// WebSocket handler owns.
func (rt *Router) register() {
	rt.mux.HandleFunc("GET "+wire.RouteHealth, rt.handleHealth)
	rt.mux.HandleFunc("GET "+wire.RouteReady, rt.handleReady)
	rt.mux.HandleFunc("GET "+wire.RouteWhoAmI, rt.handleWhoAmI)
	rt.mux.HandleFunc("GET "+wire.RouteOpenAPI, rt.handleOpenAPI)

	rt.registerAuthRoutes()
	rt.registerInviteRoutes()
	rt.registerTenantRoutes()
	rt.registerGlobalAdminRoutes()
	rt.registerProjectRoutes()
	rt.registerTaskRoutes()
	rt.registerClaimRoutes()
	rt.registerHistoryRoutes()
	rt.registerEventRoutes()
	rt.registerStatsRoutes()
	rt.registerStatusRoutes()
	rt.registerWebhookRoutes()
	rt.registerTransferRoutes()
	rt.registerBundleRoutes()
	rt.registerConnectionRoutes()
	rt.registerViewRoutes()
	rt.registerFilterRoutes()
	rt.registerCalendarRoutes()
	rt.registerInboxRoutes()
	rt.registerTimeRoutes()
	rt.registerTemplateRoutes()

	if rt.cfg.EventHandler != nil {
		rt.mux.Handle(wire.RouteEvents, rt.cfg.EventHandler)
	}
	if rt.cfg.WebHandler != nil {
		rt.mux.Handle("/", rt.cfg.WebHandler)
	}
}

// readJSON decodes a JSON request body, answering the client on failure.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if ct := r.Header.Get(wire.HeaderContentType); ct != "" && !isJSONContentType(ct) {
		writeStatusError(w, http.StatusUnsupportedMediaType, core.KindInvalid,
			"request body must be "+wire.ContentJSON)
		return false
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeStatusError(w, http.StatusRequestEntityTooLarge, core.KindInvalid, tooLargeMessage(r))
			return false
		}
		if errors.Is(err, io.EOF) {
			WriteError(w, core.Invalid("a request body is required"))
			return false
		}
		WriteError(w, core.Invalid("request body is not valid json: %v", err))
		return false
	}
	return true
}

// readOptionalJSON decodes a body when one is present, tolerating an empty one.
func readOptionalJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.ContentLength == 0 {
		return true
	}
	return readJSON(w, r, v)
}

// isJSONContentType reports whether a Content-Type names JSON.
func isJSONContentType(ct string) bool {
	media, _, _ := strings.Cut(ct, ";")
	return strings.EqualFold(strings.TrimSpace(media), wire.ContentJSON)
}

// writeStatusError renders the standard envelope at a status the error taxonomy
// does not itself name, such as payload too large.
func writeStatusError(w http.ResponseWriter, status int, kind core.Kind, message string) {
	WriteJSON(w, status, wire.ErrorEnvelope{Error: wire.ErrorBody{Code: kind, Message: message}})
}

// taskRef parses the task reference in a route path.
func taskRef(r *http.Request) (core.TaskRef, error) {
	return core.ParseTaskRef(r.PathValue("ref"))
}

// pageFrom builds a page from the standard list query parameters.
func pageFrom(r *http.Request) (core.Page, error) {
	q := r.URL.Query()
	page := core.Page{
		Cursor:    q.Get("cursor"),
		Sort:      q.Get("sort"),
		Direction: core.SortDirection(q.Get("direction")),
	}
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return core.Page{}, core.Invalid("limit %q is not a number", raw)
		}
		page.Limit = n
	}
	if page.Cursor != "" {
		if _, err := core.DecodeCursor(page.Cursor); err != nil {
			return core.Page{}, err
		}
	}
	return page.Normalize()
}

// wantsNDJSON reports whether the client asked for a streamed listing.
func wantsNDJSON(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get(wire.HeaderAccept)), wire.ContentNDJSON)
}

// writeList renders a page of items, streaming as ndjson when negotiated.
func writeList[T any](w http.ResponseWriter, r *http.Request, items []T, next string) {
	writePage(w, r, wire.Page[T]{Items: items, NextCursor: next})
}

// writePage renders a whole page envelope, streaming its items as ndjson when
// negotiated, which carries no envelope and so no total.
func writePage[T any](w http.ResponseWriter, r *http.Request, page wire.Page[T]) {
	items := page.Items
	if items == nil {
		items = []T{}
	}
	if !wantsNDJSON(r) {
		page.Items = items
		WriteJSON(w, http.StatusOK, page)
		return
	}
	w.Header().Set(wire.HeaderContentType, wire.ContentNDJSON)
	w.WriteHeader(http.StatusOK)
	stream := output.NewStream(output.FormatNDJSON, w)
	for _, item := range items {
		if err := stream.Write(item); err != nil {
			return
		}
	}
	_ = stream.Close()
}

// writeNoContent answers a successful mutation that returns no body.
func writeNoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// mustMarshal renders a value that cannot fail to marshal.
func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"error":{"code":"internal","message":"internal error"}}`)
	}
	return b
}

// boolParam reads a boolean query parameter.
func boolParam(r *http.Request, name string) bool {
	v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(name)))
	return v == "1" || v == "true" || v == "yes"
}
