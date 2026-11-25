// SPDX-License-Identifier: AGPL-3.0-or-later

// Package caldav serves CalDAV (RFC 4791) for tasks under /dav/. It translates
// WebDAV, CalDAV and sync-collection (RFC 6578) to core.TaskCollections and
// holds no rule of its own: who reads what, which tasks a collection holds and
// what a sync token stands for are decided by the service. PUT and DELETE
// change tasks through it; every other write method is refused.
package caldav

import (
	"context"
	"encoding/xml"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// Handler answers every request under /dav/.
type Handler struct {
	svc     core.TaskCollections
	origin  func(*http.Request) string
	methods map[string]method
	log     *slog.Logger
}

// WithLogger sets where the handler reports what it accepted but could not
// keep. The default is slog.Default.
func (h *Handler) WithLogger(log *slog.Logger) *Handler {
	if log != nil {
		h.log = log
	}
	return h
}

// method answers one request whose principal and path are resolved.
type method func(h *Handler, w http.ResponseWriter, req *request) error

// request is one DAV request with its principal and target.
type request struct {
	r *http.Request
	p *core.DAVPrincipal
	t target
}

// ctx is the request's context.
func (q *request) ctx() context.Context { return q.r.Context() }

// davHeader is the DAV compliance classes OPTIONS announces.
const davHeader = "1, 3, access-control, calendar-access"

// readMethods are the methods every principal may use; PUT and DELETE join
// them for one that writes.
var readMethods = []string{http.MethodOptions, http.MethodGet, http.MethodHead, "PROPFIND", "PROPPATCH", "REPORT"}

// New returns a handler over the task collections. origin gives the scheme and
// host a request reached the server at, which every object's URL is built
// under.
func New(svc core.TaskCollections, origin func(*http.Request) string) *Handler {
	h := &Handler{svc: svc, origin: origin, log: slog.Default()}
	h.methods = map[string]method{
		http.MethodOptions: (*Handler).options,
		http.MethodGet:     (*Handler).get,
		http.MethodHead:    (*Handler).get,
		"PROPFIND":         (*Handler).propfind,
		"PROPPATCH":        (*Handler).proppatch,
		"REPORT":           (*Handler).report,
		http.MethodPut:     (*Handler).put,
		http.MethodDelete:  (*Handler).delete,
		"MKCOL":            refuseCreate,
		"MKCALENDAR":       refuseCreate,
		"MOVE":             refuse(davName("unbind")),
		"COPY":             refuse(davName("bind")),
		"ACL":              refuse(davName("write-acl")),
		"LOCK":             refuse(xml.Name{}),
		"UNLOCK":           refuse(xml.Name{}),
		http.MethodPost:    refuse(xml.Name{}),
	}
	return h
}

// ServeHTTP resolves the app password's principal and the path, then answers
// by method. No actor, or a password that no longer signs anybody in, is 401
// with the Basic challenge.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, ok := core.ActorFrom(r.Context()); !ok {
		challenge(w)
		return
	}
	m, ok := h.methods[r.Method]
	if !ok {
		w.Header().Set("Allow", strings.Join(readMethods, ", "))
		plain(w, http.StatusMethodNotAllowed)
		return
	}
	p, err := h.svc.DAVPrincipal(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	t, ok := parsePath(r.URL.Path, p)
	if !ok {
		notFound(w)
		return
	}
	if err := m(h, w, &request{r: r, p: p, t: t}); err != nil {
		fail(w, err)
	}
}

// WellKnown answers /.well-known/caldav, for any method and without a
// credential, with a 301 to /dav/.
func (h *Handler) WellKnown() http.Handler { return WellKnown(h.origin) }

// WellKnown returns the /.well-known/caldav redirect, to /dav/ under origin
// when origin gives one.
func WellKnown(origin func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := ""
		if origin != nil {
			base = strings.TrimRight(origin(r), "/")
		}
		w.Header().Set("Location", base+core.DAVRoot)
		w.WriteHeader(http.StatusMovedPermanently)
	})
}

// originOf is the origin objects are built under for a request.
func (h *Handler) originOf(r *http.Request) string {
	if h.origin == nil {
		return ""
	}
	return strings.TrimRight(h.origin(r), "/")
}

// allowed is the Allow header for a principal: write methods join only when
// the principal may change tasks.
func allowed(p *core.DAVPrincipal) string {
	out := readMethods
	if p.Write {
		out = append(append([]string{}, out...), http.MethodPut, http.MethodDelete)
	}
	return strings.Join(out, ", ")
}

func (h *Handler) options(w http.ResponseWriter, req *request) error {
	w.Header().Set("Allow", allowed(req.p))
	w.Header().Set("DAV", davHeader)
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusOK)
	return nil
}

// get answers GET and HEAD of an object, honouring If-None-Match. Anything
// else is not a resource with a body.
func (h *Handler) get(w http.ResponseWriter, req *request) error {
	if req.t.kind != kindObject {
		if err := h.exists(req); err != nil {
			return err
		}
		w.Header().Set("Allow", allowed(req.p))
		plain(w, http.StatusMethodNotAllowed)
		return nil
	}
	objects, err := h.svc.GetDAVObjects(req.ctx(), core.DAVObjectsInput{Collection: req.t.collection,
		Names: []string{req.t.name}, Origin: h.originOf(req.r)})
	if err != nil {
		return err
	}
	if len(objects) != 1 {
		notFound(w)
		return nil
	}
	o := objects[0]
	w.Header().Set("ETag", o.ETag)
	if etagMatches(req.r.Header.Get("If-None-Match"), o.ETag) {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(o.Data)))
	w.WriteHeader(http.StatusOK)
	if req.r.Method != http.MethodHead {
		_, _ = w.Write(o.Data)
	}
	return nil
}

// etagMatches reports whether an If-None-Match value names the entity tag,
// by the weak comparison RFC 9110 asks of If-None-Match.
func etagMatches(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, v := range strings.Split(header, ",") {
		v = strings.TrimPrefix(strings.TrimSpace(v), "W/")
		if v == "*" || v == strings.TrimPrefix(etag, "W/") {
			return true
		}
	}
	return false
}

// proppatch answers 207 with 403 for every property named: nothing is
// stored, and Apple treats a bare error as a broken account.
func (h *Handler) proppatch(w http.ResponseWriter, req *request) error {
	root, err := readBody(req.r.Body)
	if err != nil {
		return err
	}
	if !root.is(nsDAV, "propertyupdate") {
		return errBadBody
	}
	if err := h.exists(req); err != nil {
		return err
	}
	denied := propstat{status: http.StatusForbidden}
	for _, op := range root.children {
		if !op.is(nsDAV, "set") && !op.is(nsDAV, "remove") {
			continue
		}
		for _, p := range op.all(nsDAV, "prop") {
			for _, c := range p.children {
				denied.props = append(denied.props, prop{name: c.name})
			}
		}
	}
	writeXML(w, http.StatusMultiStatus, multistatus([]response{{href: req.t.href(req.p),
		propstats: []propstat{denied}}}, nil))
	return nil
}

// exists answers not found for a collection or an object the app password
// does not expose.
func (h *Handler) exists(req *request) error {
	switch req.t.kind {
	case kindCollection:
		_, err := h.collection(req)
		return err
	case kindObject:
		objects, err := h.svc.GetDAVObjects(req.ctx(), core.DAVObjectsInput{Collection: req.t.collection,
			Names: []string{req.t.name}, Origin: h.originOf(req.r)})
		if err != nil {
			return err
		}
		if len(objects) == 0 {
			return core.NotFound("no such object")
		}
	}
	return nil
}

// collection finds the target's collection among those the app password
// exposes.
func (h *Handler) collection(req *request) (core.DAVCollection, error) {
	all, err := h.svc.ListDAVCollections(req.ctx())
	if err != nil {
		return core.DAVCollection{}, err
	}
	for _, c := range all {
		if c.Name == req.t.collection {
			return c, nil
		}
	}
	return core.DAVCollection{}, core.NotFound("no such collection")
}

// refuse answers a write method with 403, naming the privilege it needs when
// one applies, once the collection it reaches into is known to exist.
func refuse(privilege xml.Name) method {
	return func(h *Handler, w http.ResponseWriter, req *request) error {
		if req.t.kind == kindCollection || req.t.kind == kindObject {
			if _, err := h.collection(req); err != nil {
				return err
			}
		}
		if privilege.Local == "" {
			plain(w, http.StatusForbidden)
			return nil
		}
		writeXML(w, http.StatusForbidden, errorBody(davName("need-privileges"),
			"<d:resource>"+hrefXML(req.t.href(req.p))+"<d:privilege>"+emptyTag(privilege)+
				"</d:privilege></d:resource>"))
		return nil
	}
}

// refuseCreate answers MKCOL and MKCALENDAR, whose target is new by
// definition, with 403 for the bind privilege on the calendar home.
func refuseCreate(_ *Handler, w http.ResponseWriter, req *request) error {
	writeXML(w, http.StatusForbidden, errorBody(davName("need-privileges"),
		"<d:resource>"+hrefXML(homeHref(req.p))+"<d:privilege><d:bind/></d:privilege></d:resource>"))
	return nil
}

// davError is an answer with a status and an XML body, a precondition the
// request failed.
type davError struct {
	status int
	body   []byte
}

func (e *davError) Error() string { return "dav precondition failed: " + http.StatusText(e.status) }

// precondition is a 403, or another status, naming a failed precondition.
func precondition(status int, n xml.Name) error {
	return &davError{status: status, body: errorBody(n, "")}
}

// fail answers an error: a refused body is 400, a password that signs nobody
// in 401, a forgotten sync token 403 with DAV:valid-sync-token, anything not
// found the one 404.
func fail(w http.ResponseWriter, err error) {
	var de *davError
	switch {
	case errors.As(err, &de):
		writeXML(w, de.status, de.body)
	case errors.Is(err, errBadBody):
		plain(w, http.StatusBadRequest)
	case core.DAVReason(err) != "":
		writeRefusal(w, err)
	case core.IsKind(err, core.KindUnauthenticated):
		challenge(w)
	case core.IsKind(err, core.KindNotFound):
		notFound(w)
	case core.IsDAVSyncToken(err):
		writeXML(w, http.StatusForbidden, errorBody(davName("valid-sync-token"), ""))
	case core.IsKind(err, core.KindForbidden):
		plain(w, http.StatusForbidden)
	case core.IsKind(err, core.KindInvalid):
		plain(w, http.StatusBadRequest)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		plain(w, http.StatusServiceUnavailable)
	default:
		plain(w, http.StatusInternalServerError)
	}
}

// challenge is the 401 every unauthenticated request gets, naming nothing.
func challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", auth.DAVChallenge)
	plain(w, http.StatusUnauthorized)
}

// notFound is the one answer for every path that does not exist for the
// caller, whatever the reason.
func notFound(w http.ResponseWriter) { plain(w, http.StatusNotFound) }

// plain answers a status with its text alone.
func plain(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(http.StatusText(status) + "\n"))
}

// writeXML answers an XML body.
func writeXML(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", xmlContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
