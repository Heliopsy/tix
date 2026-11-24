// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"encoding/xml"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/ical"
)

// put answers PUT of an object: 204 for an update, 201 for a create, and
// never an ETag, because what tix stores is not byte for byte what was sent;
// RFC 4791 then has the client fetch the object again.
func (h *Handler) put(w http.ResponseWriter, req *request) error {
	if req.t.kind != kindObject {
		return h.refuseWrite(w, req, davName("write-content"))
	}
	if !req.p.Write {
		return h.refuseWrite(w, req, davName("write-content"))
	}
	if !isCalendar(req.r.Header.Get("Content-Type")) {
		if _, err := h.collection(req); err != nil {
			return err
		}
		plain(w, http.StatusUnsupportedMediaType)
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(req.r.Body, ical.MaxObjectBytes+1))
	if err != nil {
		return errBadBody
	}
	if len(data) > ical.MaxObjectBytes {
		return core.DAVRefusal(core.ReasonDAVInvalidObject, "a calendar object is at most %d bytes", ical.MaxObjectBytes)
	}
	res, err := h.svc.PutDAVObject(req.ctx(), core.DAVPutInput{Collection: req.t.collection, Name: req.t.name,
		IfMatch: req.r.Header.Get("If-Match"), IfNoneMatch: strings.TrimSpace(req.r.Header.Get("If-None-Match")) == "*",
		Data: data, Origin: h.originOf(req.r)})
	if err != nil {
		return err
	}
	if res.KeptDropped {
		h.log.WarnContext(req.ctx(), "caldav: properties tix has no field for were over the bound and were not kept",
			"collection", req.t.collection, "object", req.t.name, "bound", "8 KiB")
	}
	status := http.StatusNoContent
	if res.Created {
		status = http.StatusCreated
	}
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(status)
	return nil
}

// delete answers DELETE of an object: the open task is cancelled, a done one
// is left alone, and nothing is ever trashed.
func (h *Handler) delete(w http.ResponseWriter, req *request) error {
	if req.t.kind != kindObject || !req.p.Write {
		return h.refuseWrite(w, req, davName("unbind"))
	}
	err := h.svc.DeleteDAVObject(req.ctx(), core.DAVDeleteInput{Collection: req.t.collection, Name: req.t.name,
		IfMatch: req.r.Header.Get("If-Match"), Origin: h.originOf(req.r)})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// refuseWrite is the 403 a write gets from a principal that may not write,
// or at a resource that is not an object.
func (h *Handler) refuseWrite(w http.ResponseWriter, req *request, privilege xml.Name) error {
	return refuse(privilege)(h, w, req)
}

// isCalendar reports whether a PUT's media type is text/calendar. A client
// sending none is given the benefit of the doubt.
func isCalendar(contentType string) bool {
	if strings.TrimSpace(contentType) == "" {
		return true
	}
	mt, _, err := mime.ParseMediaType(contentType)
	return err == nil && mt == "text/calendar"
}

// writeRefusal answers a refused write by its reason.
func writeRefusal(w http.ResponseWriter, err error) {
	var e *core.Error
	msg := err.Error()
	if errors.As(err, &e) {
		msg = e.Message
	}
	switch core.DAVReason(err) {
	case core.ReasonDAVIfMatchRequired:
		plain(w, http.StatusPreconditionRequired)
	case core.ReasonDAVStale:
		plain(w, http.StatusPreconditionFailed)
	case core.ReasonDAVUIDConflict:
		writeXML(w, http.StatusForbidden, errorBody(calName("no-uid-conflict"), ""))
	case core.ReasonDAVInvalidObject:
		writeXML(w, http.StatusForbidden, errorBody(calName("valid-calendar-object-resource"), ""))
	case core.ReasonDAVReadOnly:
		writeXML(w, http.StatusForbidden, errorBody(davName("need-privileges"), ""))
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(msg + "\n"))
	}
}
