// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/heliopsy/tix/internal/i18n"
)

// langOf is the language a request's page is drawn in, from the browser's own
// Accept-Language. English when it names nothing this build can draw.
func langOf(r *http.Request) string {
	if r == nil {
		return i18n.English
	}
	return i18n.Negotiate(r.Header.Get("Accept-Language"))
}

// translators are the template helpers that put a page's words in its
// language, bound at parse time the way the time helpers are.
//
// t takes the English text and fmt arguments; tn picks the singular or plural
// key by a count; th is t for the few catalog entries that carry inline
// markup, and takes no arguments, so nothing a reader supplies can reach the
// unescaped output. A key with no entry in the language reads in English.
func translators(lang string) template.FuncMap {
	return template.FuncMap{
		"t": func(key string, args ...any) string { return i18n.T(lang, key, args...) },
		// #nosec G203 -- key is a literal in a template, and every catalog
		// entry for it is checked to carry the same markup as the key.
		"th":  func(key string) template.HTML { return template.HTML(i18n.T(lang, key)) },
		"tn":  func(one, other string, n int, args ...any) string { return i18n.N(lang, one, other, n, args...) },
		"tip": newTip,
	}
}

// tipView is one info tip: what it is about, the explanation, and the page in
// docs/ that says more, if any.
type tipView struct {
	Topic string
	Text  string
	Doc   string
}

// newTip builds the "tip" partial's argument. A doc is optional.
func newTip(topic, text string, doc ...string) tipView {
	v := tipView{Topic: topic, Text: text}
	if len(doc) > 0 {
		v.Doc = doc[0]
	}
	return v
}

// jsMessages are the texts the scripts write into a page themselves, which the
// head carries translated for assets/i18n.js. A test reads every say("...") in
// the assets and fails on one missing here.
var jsMessages = []string{
	"Copied",
	"Copy failed",
	"Nothing has changed yet",
	"%d files chosen",
	"No file chosen",
	"Collapsed",
	"Expand sidebar",
	"Collapse sidebar",
}

// jsMessagesFor is the table the head carries for lang, empty for English,
// which the scripts already speak.
func jsMessagesFor(lang string) template.JS {
	if lang == i18n.English {
		return ""
	}
	table := make(map[string]string, len(jsMessages))
	for _, key := range jsMessages {
		table[key] = i18n.T(lang, key)
	}
	out, err := json.Marshal(table)
	if err != nil {
		return ""
	}
	// #nosec G203 -- JSON marshalled from the compiled-in catalog, rendered
	// into a script element of type application/json.
	return template.JS(out)
}
