// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/wire"
)

// HeaderRequestID carries the identifier correlating a request with its logs.
const HeaderRequestID = "X-Request-Id"

type ctxKey int

const ctxKeyRequestID ctxKey = iota

// loggedTenantKey is the context key of the request record's tenant holder. It
// has a type of its own so it cannot share a value with the ctxKey constants,
// which are numbered in more than one block.
type loggedTenantKey struct{}

// loggedTenant is where resolveTenant leaves the tenant it pinned for the
// request record. logRequest runs outside resolveTenant, so the context it
// holds never carries the tenant itself.
type loggedTenant struct{ id string }

// noteTenant records the tenant a request was pinned to for its log record.
func noteTenant(ctx context.Context, tenantID string) {
	if lt, ok := ctx.Value(loggedTenantKey{}).(*loggedTenant); ok {
		lt.id = tenantID
	}
}

// RequestIDFrom returns the identifier assigned to the request, if any.
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// StatusClientClosed is the status recorded for a request whose client went
// away before it was answered, after nginx's 499. No client ever reads it.
const StatusClientClosed = 499

// clientClosed reports whether the request's own context was cancelled, which
// net/http does when the client disconnects. A server deadline is not this.
func clientClosed(ctx context.Context) bool {
	return ctx != nil && errors.Is(ctx.Err(), context.Canceled)
}

// recorder captures the status a handler wrote so it can be logged. Given the
// request's context, it records a server error written after the client left
// as StatusClientClosed and drops its body.
type recorder struct {
	http.ResponseWriter
	ctx     context.Context
	status  int
	written bool
	swallow bool
}

func (rec *recorder) WriteHeader(status int) {
	if rec.written {
		return
	}
	if status >= http.StatusInternalServerError && clientClosed(rec.ctx) {
		status, rec.swallow = StatusClientClosed, true
	}
	rec.status = status
	rec.written = true
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *recorder) Write(b []byte) (int, error) {
	if !rec.written {
		rec.WriteHeader(http.StatusOK)
	}
	if rec.swallow {
		return len(b), nil
	}
	return rec.ResponseWriter.Write(b)
}

// isHTMLContentType reports whether a response is a page a handler rendered.
func isHTMLContentType(ct string) bool {
	media, _, _ := strings.Cut(ct, ";")
	return strings.EqualFold(strings.TrimSpace(media), "text/html")
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
					"method", r.Method, "path", loggedPath(r.URL.Path), "panic", v)
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
// is read here, so none can be logged. A request the client abandoned is
// recorded as StatusClientClosed rather than as the server error its handler
// made of the cancelled context.
func (rt *Router) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		tenant := &loggedTenant{}
		r = r.WithContext(context.WithValue(r.Context(), loggedTenantKey{}, tenant))
		rec := &recorder{ResponseWriter: w, ctx: r.Context(), status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if !rec.written && clientClosed(r.Context()) {
			rec.status = StatusClientClosed
		}

		attrs := []any{
			"request_id", RequestIDFrom(r.Context()),
			"method", r.Method,
			"path", loggedPath(r.URL.Path),
			"status", rec.status,
			"duration_ms", time.Since(started).Milliseconds(),
			"tenant", tenant.id,
		}
		if rec.status == StatusClientClosed {
			attrs = append(attrs, "note", "client closed request")
		}
		rt.cfg.Logger.Info("request", attrs...)
	})
}

// loggedPath is the path as the log records it. A feed address is a secret a
// calendar app holds, and an invite link one a person holds, and a log is read
// by more people than either, so the secret is replaced before the line is
// written.
func loggedPath(path string) string {
	if isCalendarFeedPath(path) {
		return wire.CalendarFeedPrefix + "[redacted]"
	}
	if isInviteTokenPath(path) {
		return wire.InviteTokenPrefix + "[redacted]"
	}
	if rest, ok := strings.CutPrefix(path, core.InvitePathPrefix); ok && rest != "" {
		return core.InvitePathPrefix + "[redacted]"
	}
	return path
}

// muxErrorWriter replaces the plain text body the mux writes for an unmatched
// route with the standard envelope. A handler that already answered in JSON
// or in HTML is left alone: the browser interface's own not-found screen is
// HTML, and replacing it put a JSON body in front of a reader who had
// followed a link to a task that no longer exists.
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
		if ct := m.Header().Get(wire.HeaderContentType); !isJSONContentType(ct) && !isHTMLContentType(ct) {
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

// limitBody caps how much of a request body a handler can read: the import
// limit on an import route, the body limit on every other.
func (rt *Router) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == wire.RouteEvents || r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		limit := rt.cfg.MaxBodyBytes
		if _, pattern := rt.imports.Handler(r); pattern != "" {
			limit = rt.cfg.MaxImportBytes
			r = withTooLargeMessage(r, config.ImportLimitRefusal(limit))
		} else if _, pattern := rt.artifacts.Handler(r); pattern != "" {
			limit = config.ArtifactBodyLimit(rt.cfg.MaxArtifactBytes, limit)
			r = withTooLargeMessage(r, config.ArtifactLimitRefusal(rt.cfg.MaxArtifactBytes))
		}
		if r.ContentLength > limit {
			writeStatusError(w, http.StatusRequestEntityTooLarge, core.KindInvalid, tooLargeMessage(r))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

// tooLargeKey carries the refusal a request over its body limit is answered
// with, when its route's limit is not the general one.
type tooLargeKey struct{}

// withTooLargeMessage sets the refusal for a body over the limit.
func withTooLargeMessage(r *http.Request, message string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), tooLargeKey{}, message))
}

// tooLargeMessage is the refusal for this request's body over its limit.
func tooLargeMessage(r *http.Request) string {
	if m, ok := r.Context().Value(tooLargeKey{}).(string); ok {
		return m
	}
	return "request body exceeds the configured limit"
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

// resolveTenant pins the request to a tenant, once, before any credential is
// examined. A host bound to one tenant serves that tenant alone. A host no
// domain claims, or one whose domain is marked shared, serves the tenant the
// X-Tix-Tenant header names, else the tenant of the credential presented,
// else its own tenant.
func (rt *Router) resolveTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isUnscopedPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		served, err := rt.tenantFor(r)
		if err != nil {
			WriteError(w, err)
			return
		}
		noteTenant(r.Context(), served.tenant)
		ctx := core.WithTenant(r.Context(), core.TenantScope{TenantID: served.tenant})
		next.ServeHTTP(w, r.WithContext(core.WithServedHost(ctx, served.ServedHost)))
	})
}

// servedTenant is the tenant a request is pinned to and the host it came by.
type servedTenant struct {
	core.ServedHost
	tenant string
}

// SharedHosts is what serving several tenants on one host needs beyond
// mapping a hostname to its tenant. service.Local provides it.
type SharedHosts interface {
	// ResolveHost maps a hostname to the tenant whose domain claims it and
	// whether the domain is marked shared; an unclaimed host is not found.
	ResolveHost(ctx context.Context, hostname string) (*core.Tenant, bool, error)
	// TenantForKey returns the live tenant a key names.
	TenantForKey(ctx context.Context, key string) (*core.Tenant, error)
	// TenantAddress returns a tenant's own address, or "".
	TenantAddress(ctx context.Context, tenantID string) (string, error)
	// CredentialTenant returns the tenant of a credential hash, or "".
	CredentialTenant(ctx context.Context, tokenHash string) (string, error)
}

// tenantFor resolves the request's host, and on a shared host the tenant the
// header or the credential names.
func (rt *Router) tenantFor(r *http.Request) (servedTenant, error) {
	hosts, ok := rt.cfg.Resolver.(SharedHosts)
	if !ok {
		scope, err := rt.tenantForHost(r)
		return servedTenant{tenant: scope.TenantID, ServedHost: core.ServedHost{Host: hostOf(r), Owner: scope.TenantID}}, err
	}
	ctx := r.Context()
	host := hostOf(r)
	header := strings.ToLower(strings.TrimSpace(r.Header.Get(core.HeaderTenant)))
	owner, shared, err := hosts.ResolveHost(ctx, host)
	served := servedTenant{ServedHost: core.ServedHost{Host: host, Shared: shared}}
	switch {
	case err == nil && owner != nil:
		served.Owner = owner.ID
	case err != nil && !core.IsKind(err, core.KindNotFound) && !core.IsKind(err, core.KindInvalid):
		return servedTenant{}, err
	default:
		served.Shared, served.Owner = true, rt.cfg.DefaultTenantID
	}
	if !served.Shared {
		if header != "" && header != owner.Key {
			return servedTenant{}, core.Forbidden("this host serves one tenant, and the %s header names another",
				core.HeaderTenant)
		}
		served.tenant = served.Owner
		return served, nil
	}
	if err := rt.namedTenant(ctx, hosts, r, header, &served); err != nil {
		return servedTenant{}, err
	}
	if served.tenant == "" {
		if served.Owner == "" {
			return servedTenant{}, core.NotFound("no tenant serves host %q", host)
		}
		served.tenant = served.Owner
	}
	return served, nil
}

// namedTenant sets the tenant the header or, failing that, the credential
// names on a shared host. A header naming a tenant served at an address of
// its own is refused; a credential of one is ignored, so it is looked up in
// the host's own tenant, finds nothing there, and the visitor can sign in.
func (rt *Router) namedTenant(ctx context.Context, hosts SharedHosts, r *http.Request, header string,
	served *servedTenant,
) error {
	tenantID := ""
	if header != "" {
		t, err := hosts.TenantForKey(ctx, header)
		if err != nil {
			return err
		}
		tenantID = t.ID
	} else if hash := credentialHash(r); hash != "" {
		id, err := hosts.CredentialTenant(ctx, hash)
		if err != nil {
			return err
		}
		tenantID = id
	}
	if tenantID == "" {
		return nil
	}
	if tenantID != served.Owner {
		addr, err := hosts.TenantAddress(ctx, tenantID)
		if err != nil {
			return err
		}
		if !served.Serves(tenantID, addr) {
			if header != "" {
				return core.Forbidden("tenant %q is served at its own address, %s", header, addr)
			}
			return nil
		}
	}
	served.tenant, served.Named = tenantID, true
	return nil
}

// credentialHash is the stored hash of the credential a request presents, a
// bearer token before a session cookie as the authenticator chain tries them,
// or "" when it presents neither.
func credentialHash(r *http.Request) string {
	if scheme, value, ok := strings.Cut(r.Header.Get(wire.HeaderAuth), " "); ok && strings.EqualFold(scheme, "bearer") {
		if v := strings.TrimSpace(value); v != "" {
			return auth.HashToken(v)
		}
	}
	if c, err := r.Cookie(wire.SessionCookieName); err == nil && c.Value != "" {
		return auth.HashToken(c.Value)
	}
	return ""
}

// tenantForHost resolves the Host header, falling back to the configured
// default tenant when one is set. It serves a resolver that cannot tell a
// shared host from a bound one.
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
		if IsPublicPath(r.URL.Path) {
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
		ctx = core.WithVia(ctx, credentialVia(r, actor))
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
	return path == wire.RouteHealth || path == wire.RouteReady || path == wire.RouteOpenAPI
}

// IsPublicPath reports whether a route runs without a credential.
func IsPublicPath(path string) bool {
	return isUnscopedPath(path) || path == wire.RouteLogin ||
		path == wire.RouteSSHChallenge || path == wire.RouteSSHVerify || isCalendarFeedPath(path) ||
		isInviteTokenPath(path)
}

// Context keys for the values the forwarded-header and credential middleware
// resolve once per request.
const (
	ctxKeyClientIP ctxKey = iota + 1
	ctxKeyScheme
	ctxKeyPublicURL
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
		ctx = WithPublicURL(ctx, rt.publicURL)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r.WithContext(core.WithVia(ctx, baseVia(r))))
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
	case config.CookieSecurityAlways:
		return true
	case config.CookieSecurityNever:
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

// baseVia is the access channel a request is on before any credential is
// read: web for the browser interface's paths, api under the API prefix, the
// latter refined by the client hint. It carries the client address, which the
// trusted proxy policy has already resolved, and a truncated user agent.
func baseVia(r *http.Request) core.Via {
	v := core.Via{Kind: core.ViaWeb, UserAgent: core.TruncateUserAgent(r.UserAgent())}
	if strings.HasPrefix(r.URL.Path, wire.APIPrefix) {
		v.Kind = core.ViaAPI
	}
	if ip := ClientIPFrom(r.Context()); ip.IsValid() {
		v.ClientAddr = ip.String()
	}
	return v.Refine(r.Header.Get(wire.HeaderClient))
}

// credentialVia is the access channel the credential decides. A session
// cookie is the browser and nothing a header says changes that; a bearer
// token is the API, which the client hint may narrow to the CLI or MCP. With
// neither, as in no-auth mode, the path decides as it did before any
// credential was read.
func credentialVia(r *http.Request, actor *core.Actor) core.Via {
	v := baseVia(r)
	switch {
	case actor.TokenID != "":
		v.Kind = core.ViaAPI
		v.TokenID, v.TokenName = actor.TokenID, actor.TokenName
		v = v.Refine(r.Header.Get(wire.HeaderClient))
	case actor.SessionID != "":
		v.Kind = core.ViaWeb
		v.Session = actor.SessionID
	}
	return v
}
