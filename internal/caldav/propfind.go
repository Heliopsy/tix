// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// depth reads the Depth header. A missing one is infinity, as RFC 4918 says,
// and infinity is refused with DAV:propfind-finite-depth.
func depth(r *http.Request) (int, error) {
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Depth"))) {
	case "0":
		return 0, nil
	case "1":
		return 1, nil
	case "", "infinity":
		return 0, precondition(http.StatusForbidden, davName("propfind-finite-depth"))
	}
	return 0, errBadBody
}

// propfind answers PROPFIND at depth 0 or 1 on any resource.
func (h *Handler) propfind(w http.ResponseWriter, req *request) error {
	d, err := depth(req.r)
	if err != nil {
		return err
	}
	root, err := readBody(req.r.Body)
	if err != nil {
		return err
	}
	if root != nil && !root.is(nsDAV, "propfind") {
		return errBadBody
	}
	resources, err := h.resources(req, d)
	if err != nil {
		return err
	}
	q := parsePropRequest(root)
	out := make([]response, 0, len(resources))
	for _, res := range resources {
		if err := req.ctx().Err(); err != nil {
			return err
		}
		r, err := res.answer(q)
		if err != nil {
			return err
		}
		out = append(out, r)
	}
	writeXML(w, http.StatusMultiStatus, multistatus(out, nil))
	return nil
}

// resources are the target and, at depth 1, its members.
func (h *Handler) resources(req *request, d int) ([]resource, error) {
	p := req.p
	switch req.t.kind {
	case kindPrincipal:
		return []resource{principalResource(p)}, nil
	case kindHome:
		out := []resource{homeResource(p)}
		if d == 0 {
			return out, nil
		}
		collections, err := h.svc.ListDAVCollections(req.ctx())
		if err != nil {
			return nil, err
		}
		for _, c := range collections {
			out = append(out, collectionResource(p, c, h.snapshot(req, c.Name)))
		}
		return out, nil
	case kindCollection:
		snap := h.snapshot(req, req.t.collection)
		s, err := snap()
		if err != nil {
			return nil, err
		}
		out := []resource{collectionResource(p, s.Collection, snap)}
		if d == 0 {
			return out, nil
		}
		objects := once(func() (map[string]core.DAVObject, error) {
			return h.objects(req, req.t.collection, nil)
		})
		for _, m := range s.Members {
			out = append(out, memberResource(p, req.t.collection, m, func() (string, error) {
				all, err := objects()
				if err != nil {
					return "", err
				}
				return escape(string(all[m.Name].Data)), nil
			}))
		}
		return out, nil
	case kindObject:
		found, err := h.objects(req, req.t.collection, []string{req.t.name})
		if err != nil {
			return nil, err
		}
		for _, o := range found {
			return []resource{objectResource(p, req.t.collection, o)}, nil
		}
		return nil, core.NotFound("no such object")
	}
	return []resource{rootResource(p)}, nil
}

// snapshot is a collection's members and token, read at most once.
func (h *Handler) snapshot(req *request, collection string) func() (*core.DAVSnapshot, error) {
	return once(func() (*core.DAVSnapshot, error) {
		return h.svc.DAVSnapshot(req.ctx(), core.DAVSnapshotInput{Collection: collection, Origin: h.originOf(req.r)})
	})
}

// objects renders the named members of a collection, or every member for no
// names, by name.
func (h *Handler) objects(req *request, collection string, names []string) (map[string]core.DAVObject, error) {
	found, err := h.svc.GetDAVObjects(req.ctx(), core.DAVObjectsInput{Collection: collection, Names: names,
		Origin: h.originOf(req.r)})
	if err != nil {
		return nil, err
	}
	out := make(map[string]core.DAVObject, len(found))
	for _, o := range found {
		out[o.Name] = o
	}
	return out, nil
}
