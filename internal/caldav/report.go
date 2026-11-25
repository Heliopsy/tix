// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/ical"
)

// maxMultigetHrefs bounds the hrefs one calendar-multiget names.
const maxMultigetHrefs = 1000

// report answers REPORT: calendar-query, calendar-multiget and
// sync-collection on a collection, the first two also on one object.
func (h *Handler) report(w http.ResponseWriter, req *request) error {
	root, err := readBody(req.r.Body)
	if err != nil {
		return err
	}
	if root == nil {
		return errBadBody
	}
	onCollection := req.t.kind == kindCollection
	onObject := req.t.kind == kindObject
	switch {
	case root.is(nsCalDAV, "calendar-query") && (onCollection || onObject):
		return h.calendarQuery(w, req, root)
	case root.is(nsCalDAV, "calendar-multiget") && (onCollection || onObject):
		return h.multiget(w, req, root)
	case root.is(nsDAV, "sync-collection") && onCollection:
		return h.syncCollection(w, req, root)
	}
	if err := h.exists(req); err != nil {
		return err
	}
	return precondition(http.StatusForbidden, davName("supported-report"))
}

// answerAll answers the property request over each resource.
func answerAll(req *request, q propRequest, resources []resource) ([]response, error) {
	out := make([]response, 0, len(resources))
	for _, res := range resources {
		if err := req.ctx().Err(); err != nil {
			return nil, err
		}
		r, err := res.answer(q)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// calendarQuery evaluates the filter in memory over the collection's
// objects, which caldav.max_objects bounds.
func (h *Handler) calendarQuery(w http.ResponseWriter, req *request, root *element) error {
	f, err := parseFilter(root.child(nsCalDAV, "filter"))
	if err != nil {
		return err
	}
	var names []string
	if req.t.kind == kindObject {
		names = []string{req.t.name}
	}
	found, err := h.svc.GetDAVObjects(req.ctx(), core.DAVObjectsInput{Collection: req.t.collection, Names: names,
		Origin: h.originOf(req.r)})
	if err != nil {
		return err
	}
	if req.t.kind == kindObject && len(found) == 0 {
		return core.NotFound("no such object")
	}
	var matched []resource
	for _, o := range found {
		if err := req.ctx().Err(); err != nil {
			return err
		}
		cal, err := ical.Parse(o.Data)
		if err != nil {
			continue
		}
		if f.matches(cal) {
			matched = append(matched, objectResource(req.p, req.t.collection, o))
		}
	}
	out, err := answerAll(req, parsePropRequest(root), matched)
	if err != nil {
		return err
	}
	writeXML(w, http.StatusMultiStatus, multistatus(out, nil))
	return nil
}

// multiget answers each href: the object when the collection holds it, its
// own 404 otherwise.
func (h *Handler) multiget(w http.ResponseWriter, req *request, root *element) error {
	hrefs := root.all(nsDAV, "href")
	if len(hrefs) > maxMultigetHrefs {
		return precondition(http.StatusForbidden, davName("number-of-matches-within-limits"))
	}
	type wanted struct {
		href string
		name string
	}
	var (
		order []wanted
		names []string
	)
	for _, e := range hrefs {
		href := strings.TrimSpace(e.text)
		t, ok := parseHref(href, req.p)
		item := wanted{href: href}
		if ok && t.kind == kindObject && t.collection == req.t.collection {
			item.name = strings.ToLower(t.name)
			names = append(names, item.name)
		}
		order = append(order, item)
	}
	found := map[string]core.DAVObject{}
	if len(names) > 0 {
		var err error
		if found, err = h.objects(req, req.t.collection, names); err != nil {
			return err
		}
	} else if _, err := h.collection(req); err != nil {
		return err
	}
	q := parsePropRequest(root)
	var out []response
	for _, wt := range order {
		o, ok := found[wt.name]
		if wt.name == "" || !ok {
			out = append(out, response{href: wt.href, status: http.StatusNotFound})
			continue
		}
		rs, err := answerAll(req, q, []resource{objectResource(req.p, req.t.collection, o)})
		if err != nil {
			return err
		}
		out = append(out, rs...)
	}
	writeXML(w, http.StatusMultiStatus, multistatus(out, nil))
	return nil
}

// syncCollection answers RFC 6578 at sync level 1: what changed since the
// token with the properties asked for, what left as 404, a 507 for the
// collection when a limit cut the answer short, and the new token.
func (h *Handler) syncCollection(w http.ResponseWriter, req *request, root *element) error {
	if lvl := root.child(nsDAV, "sync-level"); lvl != nil {
		if v := strings.TrimSpace(lvl.text); v != "1" && v != "infinite" {
			return errBadBody
		}
	}
	token := ""
	if t := root.child(nsDAV, "sync-token"); t != nil {
		token = strings.TrimSpace(t.text)
	}
	limit := 0
	if l := root.child(nsDAV, "limit"); l != nil {
		n, err := strconv.Atoi(strings.TrimSpace(l.child(nsDAV, "nresults").textOrEmpty()))
		if err != nil || n < 1 {
			return errBadBody
		}
		limit = n
	}
	s, err := h.svc.DAVSnapshot(req.ctx(), core.DAVSnapshotInput{Collection: req.t.collection, Since: token,
		Limit: limit, Origin: h.originOf(req.r)})
	if err != nil {
		return err
	}
	q := parsePropRequest(root)
	objects := once(func() (map[string]core.DAVObject, error) {
		names := make([]string, len(s.Changed))
		for i, m := range s.Changed {
			names[i] = m.Name
		}
		if len(names) == 0 {
			return map[string]core.DAVObject{}, nil
		}
		return h.objects(req, req.t.collection, names)
	})
	resources := make([]resource, 0, len(s.Changed))
	for _, m := range s.Changed {
		resources = append(resources, memberResource(req.p, req.t.collection, m, func() (string, error) {
			all, err := objects()
			if err != nil {
				return "", err
			}
			return escape(string(all[m.Name].Data)), nil
		}))
	}
	out, err := answerAll(req, q, resources)
	if err != nil {
		return err
	}
	for _, name := range s.Removed {
		out = append(out, response{href: objectHref(req.p, req.t.collection, name), status: http.StatusNotFound})
	}
	if s.Truncated {
		out = append(out, response{href: collectionHref(req.p, req.t.collection), status: http.StatusInsufficientStorage})
	}
	writeXML(w, http.StatusMultiStatus, multistatus(out, &s.Token))
	return nil
}

// textOrEmpty is an element's text, empty for no element.
func (e *element) textOrEmpty() string {
	if e == nil {
		return ""
	}
	return e.text
}
