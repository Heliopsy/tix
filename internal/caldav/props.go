// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"encoding/xml"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/heliopsy/tix/internal/core"
)

// objectContentType is the media type of every object.
const objectContentType = "text/calendar; charset=utf-8; component=VTODO"

// propDef is a property a resource serves. A hidden one is answered only when
// named, never for allprop, as RFC 4791 asks of calendar-data.
type propDef struct {
	name   xml.Name
	hidden bool
	value  func() (string, error)
}

// resource is one href and the properties it serves.
type resource struct {
	href  string
	props []propDef
}

// propMode is how a PROPFIND or a report asks for properties.
type propMode int

// The three ways to ask.
const (
	modeAllProp propMode = iota
	modePropName
	modeProp
)

// propRequest is what a request asks of each resource.
type propRequest struct {
	mode    propMode
	names   []xml.Name
	include []xml.Name
}

// wants reports whether the request asks for a property by name.
func (q propRequest) wants(n xml.Name) bool {
	return slices.Contains(q.names, n) || slices.Contains(q.include, n)
}

// parsePropRequest reads the prop, allprop or propname child of a PROPFIND or
// report body. None of them, or no body, is allprop.
func parsePropRequest(root *element) propRequest {
	if root == nil {
		return propRequest{mode: modeAllProp}
	}
	if root.child(nsDAV, "propname") != nil {
		return propRequest{mode: modePropName}
	}
	if p := root.child(nsDAV, "prop"); p != nil {
		q := propRequest{mode: modeProp}
		for _, c := range p.children {
			q.names = append(q.names, c.name)
		}
		return q
	}
	q := propRequest{mode: modeAllProp}
	if inc := root.child(nsDAV, "include"); inc != nil {
		for _, c := range inc.children {
			q.include = append(q.include, c.name)
		}
	}
	return q
}

// answer evaluates the request over the resource: what it serves in a 200
// propstat and what it does not in a 404 one.
func (r resource) answer(q propRequest) (response, error) {
	ok := propstat{status: http.StatusOK}
	missing := propstat{status: http.StatusNotFound}
	find := func(n xml.Name) *propDef {
		for i := range r.props {
			if r.props[i].name == n {
				return &r.props[i]
			}
		}
		return nil
	}
	add := func(d *propDef) error {
		v, err := d.value()
		if err != nil {
			return err
		}
		ok.props = append(ok.props, prop{name: d.name, value: v})
		return nil
	}
	switch q.mode {
	case modePropName:
		for _, d := range r.props {
			ok.props = append(ok.props, prop{name: d.name})
		}
	case modeAllProp:
		for i := range r.props {
			if r.props[i].hidden && !q.wants(r.props[i].name) {
				continue
			}
			if err := add(&r.props[i]); err != nil {
				return response{}, err
			}
		}
		for _, n := range q.include {
			if find(n) == nil {
				missing.props = append(missing.props, prop{name: n})
			}
		}
	case modeProp:
		seen := map[xml.Name]bool{}
		for _, n := range q.names {
			if seen[n] {
				continue
			}
			seen[n] = true
			d := find(n)
			if d == nil {
				missing.props = append(missing.props, prop{name: n})
				continue
			}
			if err := add(d); err != nil {
				return response{}, err
			}
		}
	}
	return response{href: r.href, propstats: []propstat{ok, missing}}, nil
}

// static is a property whose value is known up front.
func static(n xml.Name, value string) propDef {
	return propDef{name: n, value: func() (string, error) { return value, nil }}
}

// hrefXML is a DAV:href element.
func hrefXML(href string) string { return "<d:href>" + escape(href) + "</d:href>" }

// privileges is the current-user-privilege-set of a resource: DAV:read, and
// the write privileges only once the principal may change tasks.
func privileges(p *core.DAVPrincipal, writable bool) string {
	out := "<d:privilege><d:read/></d:privilege>"
	if p.Write && writable {
		out += "<d:privilege><d:write-content/></d:privilege><d:privilege><d:bind/></d:privilege>" +
			"<d:privilege><d:unbind/></d:privilege>"
	}
	return out
}

// reports is a supported-report-set.
func reports(names ...xml.Name) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString("<d:supported-report><d:report>" + emptyTag(n) + "</d:report></d:supported-report>")
	}
	return b.String()
}

// rootResource is /dav/, which points a client at its principal.
func rootResource(p *core.DAVPrincipal) resource {
	return resource{href: core.DAVRoot, props: []propDef{
		static(davName("resourcetype"), "<d:collection/>"),
		static(davName("current-user-principal"), hrefXML(principalHref(p))),
		static(davName("principal-URL"), hrefXML(principalHref(p))),
		static(calName("calendar-home-set"), hrefXML(homeHref(p))),
	}}
}

// principalResource is the person the app password signs in as.
func principalResource(p *core.DAVPrincipal) resource {
	addresses := ""
	if p.Email != "" {
		addresses = hrefXML("mailto:" + p.Email)
	}
	addresses += hrefXML(principalHref(p))
	return resource{href: principalHref(p), props: []propDef{
		static(davName("resourcetype"), "<d:principal/>"),
		static(davName("displayname"), escape(p.DisplayName)),
		static(davName("principal-URL"), hrefXML(principalHref(p))),
		static(davName("current-user-principal"), hrefXML(principalHref(p))),
		static(calName("calendar-home-set"), hrefXML(homeHref(p))),
		static(calName("calendar-user-address-set"), addresses),
		static(davName("supported-report-set"), ""),
		static(davName("current-user-privilege-set"), privileges(p, false)),
	}}
}

// homeResource is the calendar home, which holds the collections.
func homeResource(p *core.DAVPrincipal) resource {
	return resource{href: homeHref(p), props: []propDef{
		static(davName("resourcetype"), "<d:collection/>"),
		static(davName("displayname"), escape(p.DisplayName)),
		static(davName("current-user-principal"), hrefXML(principalHref(p))),
		static(davName("owner"), hrefXML(principalHref(p))),
		static(davName("current-user-privilege-set"), privileges(p, false)),
		static(davName("supported-report-set"), ""),
	}}
}

// appleColor is a #rrggbb colour as Apple's clients write it, #rrggbbff.
func appleColor(c string) string {
	if len(c) == len("#rrggbb") {
		return c + "ff"
	}
	return c
}

// collectionResource is one calendar collection. snapshot is called only
// when a property needs it.
func collectionResource(p *core.DAVPrincipal, c core.DAVCollection, snapshot func() (*core.DAVSnapshot, error)) resource {
	token := func() (string, error) {
		s, err := snapshot()
		if err != nil {
			return "", err
		}
		return escape(s.Token), nil
	}
	description := func() (string, error) {
		s, err := snapshot()
		if err != nil {
			return "", err
		}
		return escape(s.Collection.Description), nil
	}
	props := []propDef{
		static(davName("resourcetype"), "<d:collection/><c:calendar/>"),
		static(davName("displayname"), escape(c.DisplayName)),
		{name: calName("calendar-description"), value: description},
		static(calName("supported-calendar-component-set"), `<c:comp name="VTODO"/>`),
		static(davName("supported-report-set"),
			reports(calName("calendar-query"), calName("calendar-multiget"), davName("sync-collection"))),
		{name: davName("sync-token"), value: token},
		{name: csName("getctag"), value: token},
		static(davName("getcontenttype"), "text/calendar; charset=utf-8"),
		static(davName("current-user-privilege-set"), privileges(p, true)),
		static(davName("owner"), hrefXML(principalHref(p))),
		static(davName("current-user-principal"), hrefXML(principalHref(p))),
	}
	if c.Color != "" {
		props = append(props, static(appleName("calendar-color"), escape(appleColor(c.Color))))
	}
	return resource{href: collectionHref(p, c.Name), props: props}
}

// memberResource is one object of a collection. data is called only when
// calendar-data is asked for.
func memberResource(p *core.DAVPrincipal, collection string, m core.DAVMember, data func() (string, error)) resource {
	return resource{href: objectHref(p, collection, m.Name), props: []propDef{
		static(davName("resourcetype"), ""),
		static(davName("getetag"), escape(m.ETag)),
		static(davName("getcontenttype"), objectContentType),
		static(davName("current-user-privilege-set"), privileges(p, true)),
		{name: calName("calendar-data"), hidden: true, value: data},
	}}
}

// objectResource is a rendered object, which carries its own data.
func objectResource(p *core.DAVPrincipal, collection string, o core.DAVObject) resource {
	return memberResource(p, collection, core.DAVMember{Name: o.Name, ETag: o.ETag},
		func() (string, error) { return escape(string(o.Data)), nil })
}

// once memoizes a call that may fail.
func once[T any](f func() (T, error)) func() (T, error) {
	var (
		mu   sync.Mutex
		done bool
		v    T
		err  error
	)
	return func() (T, error) {
		mu.Lock()
		defer mu.Unlock()
		if !done {
			v, err = f()
			done = true
		}
		return v, err
	}
}
