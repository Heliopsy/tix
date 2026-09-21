package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
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

	// SecureCookies marks the session cookie Secure, which is correct whenever
	// the server is reached over TLS.
	SecureCookies bool

	Probe          DBProbe
	ExpectedSchema int

	// EventHandler serves the WebSocket event route when it is supplied.
	EventHandler http.Handler

	// WebHandler serves the browser interface at the root when it is supplied.
	// It is registered last and only for paths the API does not claim, so an
	// API route can never be shadowed by a page.
	WebHandler http.Handler
}

// Router serves the tix REST API.
type Router struct {
	cfg     Config
	mux     *http.ServeMux
	handler http.Handler
}

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
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = DefaultRequestTimeout
	}

	rt := &Router{cfg: cfg, mux: http.NewServeMux()}
	rt.register()
	rt.handler = chain(rt.mux,
		rt.authenticate,
		rt.resolveTenant,
		rt.withTimeout,
		rt.limitBody,
		rt.envelopeMuxErrors,
		rt.logRequest,
		rt.withRequestID,
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
	rt.mux.HandleFunc("GET "+RouteHealth, rt.handleHealth)
	rt.mux.HandleFunc("GET "+RouteReady, rt.handleReady)
	rt.mux.HandleFunc("GET "+RouteWhoAmI, rt.handleWhoAmI)

	rt.registerAuthRoutes()
	rt.registerTenantRoutes()
	rt.registerProjectRoutes()
	rt.registerTaskRoutes()
	rt.registerClaimRoutes()
	rt.registerHistoryRoutes()
	rt.registerWebhookRoutes()
	rt.registerTransferRoutes()
	rt.registerBundleRoutes()

	if rt.cfg.EventHandler != nil {
		rt.mux.Handle(RouteEvents, rt.cfg.EventHandler)
	}
	if rt.cfg.WebHandler != nil {
		rt.mux.Handle("/", rt.cfg.WebHandler)
	}
}

// readJSON decodes a JSON request body, answering the client on failure.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if ct := r.Header.Get(HeaderContentType); ct != "" && !isJSONContentType(ct) {
		writeStatusError(w, http.StatusUnsupportedMediaType, core.KindInvalid,
			"request body must be "+ContentJSON)
		return false
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeStatusError(w, http.StatusRequestEntityTooLarge, core.KindInvalid,
				"request body exceeds the configured limit")
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
	return strings.EqualFold(strings.TrimSpace(media), ContentJSON)
}

// writeStatusError renders the standard envelope at a status the error taxonomy
// does not itself name, such as payload too large.
func writeStatusError(w http.ResponseWriter, status int, kind core.Kind, message string) {
	WriteJSON(w, status, ErrorEnvelope{Error: ErrorBody{Code: kind, Message: message}})
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
	return strings.Contains(strings.ToLower(r.Header.Get(HeaderAccept)), ContentNDJSON)
}

// writeList renders a page of items, streaming as ndjson when negotiated.
func writeList[T any](w http.ResponseWriter, r *http.Request, items []T, next string) {
	if items == nil {
		items = []T{}
	}
	if !wantsNDJSON(r) {
		WriteJSON(w, http.StatusOK, Page[T]{Items: items, NextCursor: next})
		return
	}
	w.Header().Set(HeaderContentType, ContentNDJSON)
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
