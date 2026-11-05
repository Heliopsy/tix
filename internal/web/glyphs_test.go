// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// glyphUse is a text character the interface may draw because it is text
// there, not a picture of something.
type glyphUse struct {
	file, line, why string
}

// glyphAllowed lists every arrow, mark or symbol character the templates,
// scripts and stylesheet may hold, each with the reason it is text.
var glyphAllowed = []glyphUse{
	{"templates/partials.html", "<kbd>↑</kbd> <kbd>↓</kbd>", "the names printed on the arrow keys"},
	{"templates/partials.html", `<option value="&#`, "suggested project icons: the emoji is the value chosen"},
	{"assets/forms.js", `"⌘"`, "the name printed on the Mac command key"},
	{"assets/palette.js", `"⌘K"`, "the shortcut as printed on a Mac keyboard"},
}

// iconGlyph reports whether r is an arrow, mark, shape or pictograph: a
// character that, drawn as an icon, comes from whatever font the reader has.
func iconGlyph(r rune) bool {
	switch {
	case r == 0x00D7, r == 0xFE0E, r == 0xFE0F:
		return true
	case r >= 0x2190 && r <= 0x21FF, r >= 0x2295 && r <= 0x22A1, r >= 0x22EE && r <= 0x22EF:
		return true
	case r >= 0x2300 && r <= 0x23FF, r >= 0x2500 && r <= 0x27BF, r >= 0x27F0 && r <= 0x27FF:
		return true
	case r >= 0x2900 && r <= 0x297F, r >= 0x2B00 && r <= 0x2BFF, r >= 0x1F300 && r <= 0x1FAFF:
		return true
	}
	return false
}

var (
	jsEscape  = regexp.MustCompile(`\\u([0-9a-fA-F]{4})|\\u\{([0-9a-fA-F]+)\}`)
	cssEscape = regexp.MustCompile(`\\([0-9a-fA-F]{1,6}) ?`)
)

// decodeEscapes turns the escapes a file of this kind may spell a character
// with into the character, so `&larr;`, `←` and `\2190` are all seen.
func decodeEscapes(name, line string) string {
	unhex := func(h string) string {
		n, err := strconv.ParseUint(h, 16, 32)
		if err != nil || !utf8.ValidRune(rune(n)) {
			return h
		}
		return string(rune(n))
	}
	switch filepath.Ext(name) {
	case ".html":
		return html.UnescapeString(line)
	case ".js":
		return jsEscape.ReplaceAllStringFunc(line, func(m string) string {
			sub := jsEscape.FindStringSubmatch(m)
			return unhex(sub[1] + sub[2])
		})
	case ".css":
		return cssEscape.ReplaceAllStringFunc(line, func(m string) string {
			return unhex(strings.TrimSpace(m[1:]))
		})
	}
	return line
}

// glyphSources are the files the interface draws from, read from the tree
// that is embedded into the binary.
func glyphSources(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pattern := range []string{"templates/*.html", "assets/*.js", "assets/*.css"} {
		found, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, found...)
	}
	var out []string
	for _, f := range files {
		if filepath.Base(f) != "htmx.min.js" {
			out = append(out, f)
		}
	}
	if len(out) < 30 {
		t.Fatalf("found only %d template, script and stylesheet files", len(out))
	}
	return out
}

// glyphFindings returns every line of every source holding an icon glyph that
// no allowlist entry explains, and marks which entries were used.
func glyphFindings(t *testing.T, files []string, used map[int]bool) []string {
	t.Helper()
	var bad []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			var glyphs []string
			for _, r := range decodeEscapes(f, line) {
				if iconGlyph(r) {
					glyphs = append(glyphs, fmt.Sprintf("%c U+%04X", r, r))
				}
			}
			if len(glyphs) == 0 {
				continue
			}
			allowed := false
			for k, a := range glyphAllowed {
				if filepath.ToSlash(f) == a.file && strings.Contains(line, a.line) {
					allowed, used[k] = true, true
				}
			}
			if !allowed {
				bad = append(bad, fmt.Sprintf("%s:%d draws %s as text", f, i+1, strings.Join(glyphs, ", ")))
			}
		}
	}
	return bad
}

// TestNoTextGlyphStandsInForAnIcon fails on an arrow, cross, triangle or
// other symbol character in the templates, scripts or stylesheet.
//
// The back links were "&larr; Templates", the group menu's moves were
// &#8593; and &#8595;, the pager's arrows and every disclosure triangle were
// characters. A character comes from whatever font the reader has, at that
// font's weight and on its baseline, so the arrows sat off the label's line
// and thinner than every icon beside them, and differed between machines.
// Icons come from assets/icons.svg through the "icon" template. A character
// that is genuinely text goes in glyphAllowed with the reason.
func TestNoTextGlyphStandsInForAnIcon(t *testing.T) {
	t.Parallel()
	used := map[int]bool{}
	for _, b := range glyphFindings(t, glyphSources(t), used) {
		t.Error(b)
	}
	for k, a := range glyphAllowed {
		if !used[k] {
			t.Errorf("glyphAllowed names %q in %s, which is no longer there; remove the entry", a.line, a.file)
		}
	}
}

// TestTheGlyphScanSeesEveryEscape keeps the scan honest about spellings: a
// test that only looked for the raw character would pass every entity.
func TestTheGlyphScanSeesEveryEscape(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ file, line string }{
		{"a.html", `<a href="/">&larr; Back</a>`},
		{"a.html", `<span>&#8593;</span>`},
		{"a.html", `<span>&#x21bb;</span>`},
		{"a.html", `<b>&times;</b>`},
		{"a.js", `el.textContent = "→";`},
		{"a.css", `a::before { content: "\25B8"; }`},
		{"a.css", `a::before { content: "└"; }`},
	} {
		found := false
		for _, r := range decodeEscapes(c.file, c.line) {
			found = found || iconGlyph(r)
		}
		if !found {
			t.Errorf("the scan misses the glyph in %s: %s", c.file, c.line)
		}
	}
}

var iconUse = regexp.MustCompile(`\{\{template "icon" "([a-z0-9-]+)"\}\}`)

// TestEveryIconTheTemplatesDrawIsInTheSprite fails on a name the sprite does
// not define, which renders as nothing at all rather than as an error.
func TestEveryIconTheTemplatesDrawIsInTheSprite(t *testing.T) {
	t.Parallel()
	sprite, err := os.ReadFile("assets/icons.svg")
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range iconUse.FindAllStringSubmatch(string(raw), -1) {
			seen++
			if !strings.Contains(string(sprite), `<symbol id="`+m[1]+`"`) {
				t.Errorf("%s draws icon %q, which assets/icons.svg does not define", f, m[1])
			}
		}
	}
	if seen < 10 {
		t.Fatalf("found only %d icon uses in the templates", seen)
	}
}
