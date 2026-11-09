// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tasklink builds a link to one task's screen in the browser
// interface, and the forms it is pasted in: the address, a Markdown link and
// the reference with the title. assets/share.js builds the same forms in the
// browser, and the tests on both sides share their cases.
package tasklink

import (
	"net"
	"net/url"
	"strings"
)

// Path is the task's screen under any host, from its stored reference.
func Path(ref string) string { return "/tasks/" + url.PathEscape(ref) }

// URL is the task's screen under base, the address the reader reaches the
// interface at.
func URL(base, ref string) string { return strings.TrimRight(base, "/") + Path(ref) }

// Text is the reference and the title on one line, as INFRA-12 Fix the build.
func Text(ref, title string) string {
	if t := oneLine(title); t != "" {
		return ref + " " + t
	}
	return ref
}

// Markdown is a link to address labelled with the reference and the title.
// The label escapes what would end it or the link early.
func Markdown(ref, title, address string) string {
	return "[" + Label(ref, title) + "](" + address + ")"
}

// Label is the reference and the title escaped for Markdown, what Markdown
// links: on its own, the form shared where there is no address to link to.
func Label(ref, title string) string { return mdEscape.Replace(Text(ref, title)) }

// Rehost is base with its host name replaced by host, keeping the scheme,
// the port and the base path: the address a tenant served on a domain of its
// own is reached at, when base is the server's public address. It returns
// base unchanged when either is empty or base does not parse.
func Rehost(base, host string) string {
	u, err := url.Parse(base)
	if err != nil || host == "" || u.Host == "" {
		return base
	}
	if port := u.Port(); port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else {
		u.Host = host
	}
	return strings.TrimRight(u.String(), "/")
}

var mdEscape = strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`)

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
