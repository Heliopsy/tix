// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/i18n"
)

// LanguageCookie remembers a language this browser chose on settings over
// the one its Accept-Language asks for. Absent, empty or a language this
// build does not draw all mean the browser's own.
const LanguageCookie = "tix_lang"

// langOf is the language a request's page is drawn in: the one chosen on
// settings, else the browser's own Accept-Language, else English.
func langOf(r *http.Request) string {
	if r == nil {
		return i18n.English
	}
	if chosen := cookieValue(r, LanguageCookie); chosen != "" && i18n.Supported(chosen) {
		return chosen
	}
	return i18n.Negotiate(r.Header.Get("Accept-Language"))
}

// languageNames are the languages as their own readers write them, which is
// how a language picker names them in every language.
var languageNames = map[string]string{i18n.English: "English", i18n.Bulgarian: "Български"}

// languageChoice is one option of the language picker. The empty value is
// the browser's own language.
type languageChoice struct {
	Value    string
	Label    string
	Selected bool
}

// languageChoices is the picker, with this browser's choice marked.
func languageChoices(r *http.Request) []languageChoice {
	current := cookieValue(r, LanguageCookie)
	if !i18n.Supported(current) {
		current = ""
	}
	out := []languageChoice{{Value: "", Label: "Browser default", Selected: current == ""}}
	for _, lang := range i18n.Languages() {
		out = append(out, languageChoice{Value: lang, Label: languageNames[lang], Selected: lang == current})
	}
	return out
}

// setLanguage records the language this browser chose. The browser default
// stores nothing, the way the other preferences store nothing for theirs.
func (h *handler) setLanguage(w http.ResponseWriter, r *http.Request) error {
	value := field(r, "lang")
	if !i18n.Supported(value) {
		value = ""
	}
	age := cookieYear
	if value == "" {
		age = -1
	}
	// #nosec G124 -- a display preference, readable by no script; Secure
	// tracks TLS like every other cookie here.
	http.SetCookie(w, &http.Cookie{
		Name: LanguageCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: h.secureCookie(r), SameSite: http.SameSiteLaxMode,
		MaxAge: age,
	})
	// #nosec G710 -- safeNext rejects anything that is not a relative path on
	// this origin, including protocol-relative, backslash and control forms.
	http.Redirect(w, r, safeNext(field(r, "next")), http.StatusSeeOther)
	return nil
}

// phrase is one shape of text Go builds around a value: the prefix and
// suffix it adds, the key that translates the whole, and whether the value
// inside is itself a key (a listing name) or data (a project key).
type phrase struct {
	prefix, suffix, key string
	inner               bool
}

// phrases are every shape the flashes and back links build, so a text built
// from parts is translated as its parts.
var phrases = []phrase{
	{suffix: " saved", key: "%s saved", inner: true},
	{prefix: "Columns on ", key: "Columns on %s", inner: true},
	{prefix: "Back to the ", suffix: " board", key: "Back to the %s board"},
}

// phraseText is a flash or a back link in lang: the whole text when the
// catalog holds it, else the shape it was built in with its value put back,
// else the English.
func phraseText(lang, text string) string {
	if out, ok := i18n.Lookup(lang, text); ok {
		return out
	}
	for _, p := range phrases {
		value, ok := strings.CutPrefix(text, p.prefix)
		if !ok {
			continue
		}
		if value, ok = strings.CutSuffix(value, p.suffix); !ok || value == "" {
			continue
		}
		if p.inner {
			value = phraseText(lang, value)
		}
		return i18n.T(lang, p.key, value)
	}
	return text
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
		"th":     func(key string) template.HTML { return template.HTML(i18n.T(lang, key)) },
		"tn":     func(one, other string, n int, args ...any) string { return i18n.N(lang, one, other, n, args...) },
		"tc":     func(ctx, text string) string { return i18n.C(lang, ctx, text) },
		"tip":    newTip,
		"phrase": func(text string) string { return phraseText(lang, text) },
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
	"That change was refused.",
	"A task needs a title.",
	"The title could not be saved.",
	"Title of %s",
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
