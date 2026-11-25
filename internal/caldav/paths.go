// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// kind is which resource a path names.
type kind int

// The resources under /dav/.
const (
	kindRoot kind = iota + 1
	kindPrincipal
	kindHome
	kindCollection
	kindObject
)

// objectSuffix ends every object's resource name.
const objectSuffix = ".ics"

// target is a parsed path: its kind, and for a collection or an object their
// names, the object's without the suffix.
type target struct {
	kind       kind
	collection string
	name       string
}

// parsePath reads a path under /dav/ for the principal. A tenant key or an
// actor that is not the principal's, a segment that is empty or unknown, or
// anything past an object answers false, which the handler answers as the
// same 404 as a path that never existed.
func parsePath(path string, p *core.DAVPrincipal) (target, bool) {
	if path == core.DAVPath || path == core.DAVRoot {
		return target{kind: kindRoot}, true
	}
	rest, ok := strings.CutPrefix(path, core.DAVRoot)
	if !ok {
		return target{}, false
	}
	segs := strings.Split(rest, "/")
	if segs[len(segs)-1] == "" {
		segs = segs[:len(segs)-1]
	}
	for _, s := range segs {
		if s == "" {
			return target{}, false
		}
	}
	if len(segs) < 3 || segs[0] != p.TenantKey || segs[2] != p.ActorID {
		return target{}, false
	}
	switch {
	case segs[1] == "principals" && len(segs) == 3:
		return target{kind: kindPrincipal}, true
	case segs[1] != "calendars":
		return target{}, false
	case len(segs) == 3:
		return target{kind: kindHome}, true
	case len(segs) == 4:
		return target{kind: kindCollection, collection: segs[3]}, true
	case len(segs) == 5 && !strings.HasSuffix(path, "/"):
		name, ok := strings.CutSuffix(segs[4], objectSuffix)
		if !ok || name == "" {
			return target{}, false
		}
		return target{kind: kindObject, collection: segs[3], name: name}, true
	}
	return target{}, false
}

// parseHref reads an href a client sent, a path or an absolute URL, as
// parsePath does.
func parseHref(href string, p *core.DAVPrincipal) (target, bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return target{}, false
	}
	return parsePath(u.Path, p)
}

// The hrefs tix answers with: absolute paths, each segment escaped.
func principalHref(p *core.DAVPrincipal) string {
	return core.DAVRoot + url.PathEscape(p.TenantKey) + "/principals/" + url.PathEscape(p.ActorID) + "/"
}

func homeHref(p *core.DAVPrincipal) string {
	return core.DAVRoot + url.PathEscape(p.TenantKey) + "/calendars/" + url.PathEscape(p.ActorID) + "/"
}

func collectionHref(p *core.DAVPrincipal, collection string) string {
	return homeHref(p) + url.PathEscape(collection) + "/"
}

func objectHref(p *core.DAVPrincipal, collection, name string) string {
	return collectionHref(p, collection) + url.PathEscape(name+objectSuffix)
}

// href is the canonical href of a target.
func (t target) href(p *core.DAVPrincipal) string {
	switch t.kind {
	case kindPrincipal:
		return principalHref(p)
	case kindHome:
		return homeHref(p)
	case kindCollection:
		return collectionHref(p, t.collection)
	case kindObject:
		return objectHref(p, t.collection, t.name)
	}
	return core.DAVRoot
}
