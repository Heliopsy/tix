// SPDX-License-Identifier: AGPL-3.0-or-later

// Package i18n holds the browser interface's message catalogs and picks the
// language a request is answered in.
//
// A key is the English text itself, gettext style, so English needs no
// catalog and a template still reads as the page it draws. Every other
// language registers one entry per key; a key a catalog lacks renders in
// English, which is the documented fallback and the reason a missing entry is
// a test failure rather than a blank on screen.
package i18n

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// English is the source language. Its catalog is the keys themselves.
const English = "en"

// catalogs maps a language tag to its translations, keyed by the English text.
var catalogs = map[string]map[string]string{}

// register adds one area's entries to a language's catalog. Areas are split
// across files so a screen's strings sit together; the same key may appear in
// two areas only with the same translation.
func register(lang string, entries map[string]string) {
	c, ok := catalogs[lang]
	if !ok {
		c = map[string]string{}
		catalogs[lang] = c
	}
	for k, v := range entries {
		if prev, dup := c[k]; dup && prev != v {
			panic(fmt.Sprintf("i18n: %s key %q registered twice with different text", lang, k))
		}
		c[k] = v
	}
}

// Languages lists every language a page can be drawn in, English first.
func Languages() []string {
	out := make([]string, 0, len(catalogs)+1)
	for lang := range catalogs {
		out = append(out, lang)
	}
	sort.Strings(out)
	return append([]string{English}, out...)
}

// Supported reports whether lang is a language this build can draw.
func Supported(lang string) bool {
	_, ok := catalogs[lang]
	return ok || lang == English
}

// Catalog returns a copy of one language's entries, for the completeness tests.
func Catalog(lang string) map[string]string {
	out := make(map[string]string, len(catalogs[lang]))
	for k, v := range catalogs[lang] {
		out[k] = v
	}
	return out
}

// Lookup returns the text key reads as in lang, and whether the catalog held
// it. English always holds it.
func Lookup(lang, key string) (string, bool) {
	if lang == English {
		return key, true
	}
	v, ok := catalogs[lang][key]
	if !ok {
		return key, false
	}
	return v, true
}

// T translates key into lang and formats it with args, fmt style. With no
// args the text is returned as is, so a literal percent sign survives.
func T(lang, key string, args ...any) string {
	text, _ := Lookup(lang, key)
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

// Context separates a context from its text in a catalog key, gettext's
// EOT, for the rare English word two places need translated differently.
const Context = "\x04"

// C translates text in a named context: the catalog key is ctx, Context and
// text, and the fallback is text alone.
func C(lang, ctx, text string) string {
	if v, ok := Lookup(lang, ctx+Context+text); ok && lang != English {
		return v
	}
	return text
}

// N picks the singular or plural key by n, translates it and formats it with
// n followed by args. A form that formats nothing is returned as it is, so a
// form may leave the count out. Every language shipped so far splits one
// from the rest.
func N(lang, one, other string, n int, args ...any) string {
	key := other
	if n == 1 {
		key = one
	}
	text, _ := Lookup(lang, key)
	if !strings.Contains(text, "%") {
		return text
	}
	return fmt.Sprintf(text, append([]any{n}, args...)...)
}

// Negotiate picks the language for an Accept-Language header: the supported
// one with the highest weight, English when none is.
func Negotiate(header string) string {
	best, bestQ := English, -1.0
	for _, part := range strings.Split(header, ",") {
		tag, q := parseRange(part)
		if tag == "" || q <= 0 {
			continue
		}
		base, _, _ := strings.Cut(tag, "-")
		for _, candidate := range []string{tag, base} {
			if Supported(candidate) && q > bestQ {
				best, bestQ = candidate, q
				break
			}
		}
	}
	return best
}

// parseRange reads one language range and its weight from an Accept-Language
// header. A missing or unreadable weight is 1, the header's own default.
func parseRange(part string) (string, float64) {
	tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
	tag = strings.ToLower(strings.TrimSpace(tag))
	q := 1.0
	for _, p := range strings.Split(params, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(p), "=")
		if !ok || strings.TrimSpace(name) != "q" {
			continue
		}
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			q = parsed
		}
	}
	return tag, q
}
