// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tasklink builds a link to one task's screen in the browser
// interface, and the forms it is pasted in: the address, a Markdown link and
// the reference with the title. assets/share.js builds the same forms in the
// browser, and the tests on both sides share their cases.
package tasklink

import (
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
	return "[" + mdEscape.Replace(Text(ref, title)) + "](" + address + ")"
}

var mdEscape = strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`)

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
