package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/wire"
)

// HeaderRequestID carries the identifier correlating a request with its logs.
const HeaderRequestID = "X-Request-Id"

type ctxKey int

const ctxKeyRequestID ctxKey = iota

// RequestIDFrom returns the identifier assigned to the request, if any.
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// recorder captures the status a handler wrote so it can be logged.
type recorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (rec *recorder) WriteHeader(status int) {
	if rec.written {
		return
	}
	rec.status = status
	rec.written = true
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *recorder) Write(b []byte) (int, error) {
	if !rec.written {
		rec.WriteHeader(http.StatusOK)
	}
	return rec.ResponseWriter.Write(b)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (rec *recorder) Unwrap() http.ResponseWriter { return rec.ResponseWriter }

// recoverPanic turns a panicking handler into an internal error.
func (rt *Router) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &recorder{ResponseWriter: w}
		defer func() {
			if v := recover(); v != nil {
				rt.cfg.Logger.Error("panic serving request",
					"method", r.Method, "path", r.URL.Path, "panic", v)
				if !rec.written {
					WriteError(rec, core.Internal("panic serving request"))
				}
			}
		}()
		next.ServeHTTP(rec, r)
	})
}

// withRequestID assigns an identifier, honouring one supplied by a proxy.
func (rt *Router) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := strings.TrimSpace(r.Header.Get(HeaderRequestID))
		if rid == "" || len(rid) > 128 {
			rid = id.New()
		}
		w.Header().Set(HeaderRequestID, rid)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID, rid)))
	})
}

// logRequest emits one structured record per request. No credential material
// is read here, so none can be logged.
func (rt *Router) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		tenant := ""
		if scope, ok := core.TenantFrom(r.Context()); ok {
			tenant = scope.TenantID
		}
		rt.cfg.Logger.Info("request",
			"request_id", RequestIDFrom(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(started).Milliseconds(),
			"tenant", tenant,
		)
	})
}

// muxErrorWriter replaces the plain text body the mux writes for an unmatched
// route with the standard envelope.
type muxErrorWriter struct {
	http.ResponseWriter
	swallow bool
	written bool
}

func (m *muxErrorWriter) WriteHeader(status int) {
	if m.written {
		return
	}
	m.written = true
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		if !isJSONContentType(m.Header().Get(wire.HeaderContentType)) {
			m.swallow = true
			m.Header().Set(wire.HeaderContentType, wire.ContentJSON)
			m.ResponseWriter.WriteHeader(status)
			_, _ = m.ResponseWriter.Write(muxErrorBody(status))
			return
		}
	}
	m.ResponseWriter.WriteHeader(status)
}

func (m *muxErrorWriter) Write(b []byte) (int, error) {
	if !m.written {
		m.WriteHeader(http.StatusOK)
	}
	if m.swallow {
		return len(b), nil
	}
	return m.ResponseWriter.Write(b)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (m *muxErrorWriter) Unwrap() http.ResponseWriter { return m.ResponseWriter }

// muxErrorBody renders the envelope for a routing failure.
func muxErrorBody(status int) []byte {
	body := wire.ErrorBody{Code: core.KindNotFound, Message: "no such route"}
	if status == http.StatusMethodNotAllowed {
		body = wire.ErrorBody{Code: core.KindInvalid, Message: "method not allowed for this route"}
	}
	return append(mustMarshal(wire.ErrorEnvelope{Error: body}), '\n')
}

// envelopeMuxErrors gives the mux's own not found and method not allowed
// responses the same envelope every handler uses.
func (rt *Router) envelopeMuxErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&muxErrorWriter{ResponseWriter: w}, r)
	})
}

// limitBody caps how much of a request body a handler can read.
func (rt *Router) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == wire.RouteEvents || r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		if r.ContentLength > rt.cfg.MaxBodyBytes {
			writeStatusError(w, http.StatusRequestEntityTooLarge, core.KindInvalid,
				"request body exceeds the configured limit")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, rt.cfg.MaxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// timeoutWriter answers with the timeout envelope once the request deadline
// has passed, whatever the handler was about to write.
type timeoutWriter struct {
	http.ResponseWriter
	ctx     context.Context
	swallow bool
	written bool
}

func (t *timeoutWriter) WriteHeader(status int) {
	if t.written {
		return
	}
	t.written = true
	if errors.Is(t.ctx.Err(), context.DeadlineExceeded) {
		t.swallow = true
		t.Header().Set(wire.HeaderContentType, wire.ContentJSON)
		t.ResponseWriter.WriteHeader(http.StatusGatewayTimeout)
		_, _ = t.ResponseWriter.Write(timeoutBody())
		return
	}
	t.ResponseWriter.WriteHeader(status)
}

func (t *timeoutWriter) Write(b []byte) (int, error) {
	if !t.written {
		t.WriteHeader(http.StatusOK)
	}
	if t.swallow {
		return len(b), nil
	}
	return t.ResponseWriter.Write(b)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (t *timeoutWriter) Unwrap() http.ResponseWriter { return t.ResponseWriter }

// timeoutBody renders the envelope a timed out request answers with.
func timeoutBody() []byte {
	return append(mustMarshal(wire.ErrorEnvelope{Error: wire.ErrorBody{
		Code:    core.KindInternal,
		Message: "the request exceeded the server timeout",
	}}), '\n')
}

// withTimeout bounds how long a request may run.
func (rt *Router) withTimeout(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == wire.RouteEvents {
			next.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), rt.cfg.RequestTimeout)
		defer cancel()

		tw := &timeoutWriter{ResponseWriter: w, ctx: ctx}
		next.ServeHTTP(tw, r.WithContext(ctx))
		if !tw.written && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			tw.WriteHeader(http.StatusGatewayTimeout)
		}
	})
}

// resolveTenant pins the request to the tenant its Host names, before any
// credential is examined.
func (rt *Router) resolveTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isUnscopedPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		scope, err := rt.tenantForHost(r)
		if err != nil {
			WriteError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(core.WithTenant(r.Context(), scope)))
	})
}

// tenantForHost resolves the Host header, falling back to the configured
// default tenant when one is set.
func (rt *Router) tenantForHost(r *http.Request) (core.TenantScope, error) {
	host := hostOf(r)
	tenant, err := rt.cfg.Resolver.ResolveDomain(r.Context(), host)
	switch {
	case err == nil && tenant != nil:
		return core.TenantScope{TenantID: tenant.ID}, nil
	case err != nil && !core.IsKind(err, core.KindNotFound) && !core.IsKind(err, core.KindInvalid):
		return core.TenantScope{}, err
	case rt.cfg.DefaultTenantID != "":
		return core.TenantScope{TenantID: rt.cfg.DefaultTenantID}, nil
	default:
		return core.TenantScope{}, core.NotFound("no tenant serves host %q", host)
	}
}

// hostOf returns the request host without its port.
func hostOf(r *http.Request) string {
	host := r.Host
	if h, _, found := strings.Cut(host, ":"); found {
		host = h
	}
	return strings.ToLower(strings.TrimSpace(host))
}

// authenticate resolves the caller and refuses a credential minted for another
// tenant than the one the Host resolved to.
func (rt *Router) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		actor, err := rt.cfg.Authenticator.Authenticate(r.Context(), r)
		if err != nil {
			// The browser surface manages its own session: it redirects an
			// anonymous visitor to its login page rather than answering with a
			// JSON envelope. Carrying on without an actor is what lets it do
			// that. Anything under the API prefix must present a credential.
			if strings.HasPrefix(r.URL.Path, wire.APIPrefix) {
				WriteError(w, err)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		scope, ok := core.TenantFrom(r.Context())
		if ok && actor.TenantID != "" && actor.TenantID != scope.TenantID {
			WriteError(w, core.Unauthenticated("credential is not valid for this host"))
			return
		}
		if actor.TenantID == "" && ok {
			actor.TenantID = scope.TenantID
		}
		ctx := core.WithSource(core.WithActor(r.Context(), actor), core.SourceAPI)
		// The event stream outlives this request and has to ask again.
		ctx = context.WithValue(ctx, ctxKeyRevalidate, rt.revalidator(r))
		// Logout ends the session the caller presented, so it has to know which
		// one that was. Only a cookie identifies a session; a bearer token does not.
		if c, err := r.Cookie(wire.SessionCookieName); err == nil && c.Value != "" {
			ctx = service.WithSessionToken(ctx, c.Value)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isUnscopedPath reports whether a route runs before tenant resolution.
func isUnscopedPath(path string) bool {
	return path == wire.RouteHealth || path == wire.RouteReady
}

// isPublicPath reports whether a route runs without a credential.
func isPublicPath(path string) bool {
	return isUnscopedPath(path) || path == wire.RouteLogin
}

// Context keys for the values the forwarded-header and credential middleware
// resolve once per request.
const (
	ctxKeyClientIP ctxKey = iota + 1
	ctxKeyScheme
	ctxKeyRevalidate
)

// Revalidate re-resolves the actor a long-lived connection was opened for. A
// connection that outlives one request has to ask again, because logout, a
// revocation, an expiry or a role change all happen after the upgrade.
type Revalidate func(ctx context.Context) (*core.Actor, error)

// RevalidateFrom returns the revalidator for the credential the request
// presented, if the request carried one.
func RevalidateFrom(ctx context.Context) (Revalidate, bool) {
	v, ok := ctx.Value(ctxKeyRevalidate).(Revalidate)
	return v, ok && v != nil
}

// ClientIPFrom returns the address the request is attributed to. It is invalid
// when the peer address could not be parsed.
func ClientIPFrom(ctx context.Context) netip.Addr {
	v, _ := ctx.Value(ctxKeyClientIP).(netip.Addr)
	return v
}

// SchemeFrom returns the scheme the client reached the deployment over, which
// is the proxy's scheme when a trusted proxy reported one.
func SchemeFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyScheme).(string)
	if v == "" {
		return auth.SchemeHTTP
	}
	return v
}

// withForwarded resolves the client address and the effective scheme once,
// honouring the forwarded headers only when the immediate peer is a configured
// trusted proxy. Every later decision reads the answer from the context rather
// than parsing the headers again.
func (rt *Router) withForwarded(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ctxKeyClientIP, rt.proxies.ClientIP(r))
		ctx = context.WithValue(ctx, ctxKeyScheme, rt.proxies.Scheme(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// secureCookie reports whether a cookie set on this request must be Secure.
//
// A cookie without Secure travels over plain HTTP, so the answer follows the
// scheme the client actually used rather than whether this process holds a
// certificate: behind a TLS-terminating proxy it holds none. CookieSecurity
// overrides the derivation in both directions, because an operator on a
// private plaintext network needs a way to say so.
func (rt *Router) secureCookie(r *http.Request) bool {
	switch rt.cfg.CookieSecurity {
	case CookieSecurityAlways:
		return true
	case CookieSecurityNever:
		return false
	default:
		return rt.cfg.SecureCookies || SchemeFrom(r.Context()) == auth.SchemeHTTPS
	}
}

// credentialRequest copies just the credential-bearing headers, so a
// revalidation outlives the body and the routing of the request it came from.
func credentialRequest(r *http.Request) *http.Request {
	clone := &http.Request{
		Method:     http.MethodGet,
		URL:        r.URL,
		Header:     make(http.Header, 2),
		Host:       r.Host,
		RemoteAddr: r.RemoteAddr,
	}
	if v := r.Header.Get(wire.HeaderAuth); v != "" {
		clone.Header.Set(wire.HeaderAuth, v)
	}
	if v := r.Header.Get("Cookie"); v != "" {
		clone.Header.Set("Cookie", v)
	}
	return clone
}

// revalidator returns a function re-running authentication for the credential
// this request presented, under the same tenant check the request passed.
func (rt *Router) revalidator(r *http.Request) Revalidate {
	cred := credentialRequest(r)
	return func(ctx context.Context) (*core.Actor, error) {
		actor, err := rt.cfg.Authenticator.Authenticate(ctx, cred.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		scope, ok := core.TenantFrom(ctx)
		if ok && actor.TenantID != "" && actor.TenantID != scope.TenantID {
			return nil, core.Unauthenticated("credential is not valid for this host")
		}
		if actor.TenantID == "" && ok {
			actor.TenantID = scope.TenantID
		}
		return actor, nil
	}
}
