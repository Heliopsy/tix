// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// bundledSheet is app.css as the server joins it.
func bundledSheet(t *testing.T) string {
	t.Helper()
	sub, err := fs.Sub(assetFS, "assets")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := bundleStylesheet(sub)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

var declRe = regexp.MustCompile(`(?m)^\s*(--[a-z0-9-]+|color-scheme):\s*([^;]+);`)

// schemeDecls is every custom property and color-scheme one rule declares,
// read from the rule alone.
func schemeDecls(t *testing.T, sheet, selector string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, m := range declRe.FindAllStringSubmatch(schemeBlock(t, sheet, selector), -1) {
		out[m[1]] = strings.TrimSpace(m[2])
	}
	if len(out) == 0 {
		t.Fatalf("%s declares nothing", selector)
	}
	return out
}

const (
	darkRule       = `:root[data-theme="dark"]`
	oledRule       = `:root[data-theme="oled"]`
	systemOLEDRule = `:root[data-theme="system-oled"]`
)

// A token added to dark and not to OLED would leave OLED drawing the light
// value of it, since OLED does not inherit from dark. And system-oled is OLED
// under a dark device, so its block is OLED's, declaration for declaration.
func TestEveryDarkTokenHasAnOLEDValue(t *testing.T) {
	sheet := bundledSheet(t)
	dark := schemeDecls(t, sheet, darkRule)
	oled := schemeDecls(t, sheet, oledRule)
	system := schemeDecls(t, sheet, systemOLEDRule)
	for name := range dark {
		if _, ok := oled[name]; !ok {
			t.Errorf("dark declares %s and OLED does not", name)
		}
	}
	for name, v := range oled {
		if system[name] != v {
			t.Errorf("OLED has %s: %s, system-oled has %q", name, v, system[name])
		}
	}
	for name := range system {
		if _, ok := oled[name]; !ok {
			t.Errorf("system-oled declares %s and OLED does not", name)
		}
	}
	if oled["--bg"] != "#000000" {
		t.Errorf("the OLED page is %s, not black", oled["--bg"])
	}
}

// The OLED scheme is held to the floor the others are tuned to: text,
// accents and state colours at 4.5:1 on the page and the surface, the words
// on every soft tint at 4.5:1, and the strong line at 3:1, because with the
// surface all but black it is the only edge a field has.
func TestTheOLEDSchemeClearsTheContrastFloor(t *testing.T) {
	sheet := bundledSheet(t)
	block := schemeBlock(t, sheet, oledRule)
	hex := func(name string) core.RGB { return cssHex(t, block, name) }
	bg, surface := hex("--bg"), hex("--surface")
	check := func(what string, fg, on core.RGB, floor float64) {
		t.Helper()
		if got := core.Contrast(fg, on); got < floor {
			t.Errorf("%s: %s on %s reaches %.2f, under %.1f", what, fg.Hex(), on.Hex(), got, floor)
		}
	}
	text := []string{"--ink", "--muted", "--accent", "--danger", "--warn", "--ok"}
	for _, c := range core.ProjectColors() {
		text = append(text, "--proj-"+string(c))
	}
	for _, name := range text {
		check(name+" on the page", hex(name), bg, 4.5)
		check(name+" on the surface", hex(name), surface, 4.5)
	}
	check("--line-strong on the page", hex("--line-strong"), bg, 3)
	check("--line-strong on the surface", hex("--line-strong"), surface, 3)
	check("--on-accent", hex("--on-accent"), hex("--accent"), 4.5)
	check("--on-ok", hex("--on-ok"), hex("--ok"), 4.5)
	for _, tone := range []string{"accent", "danger", "warn", "ok"} {
		soft := hex("--" + tone + "-soft")
		check("--ink on --"+tone+"-soft", hex("--ink"), soft, 4.5)
		check("--"+tone+" on --"+tone+"-soft", hex("--"+tone), soft, 4.5)
	}

	reg, err := core.NewThemeRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, th := range reg.List() {
		accent, _ := core.ParseHex(th.AccentDark)
		soft, _ := core.ParseHex(th.AccentSoftDark)
		check(th.Name+" accent on the page", accent, bg, 4.5)
		check(th.Name+" accent on the surface", accent, surface, 4.5)
		check("ink on "+th.Name+"'s tint", hex("--ink"), soft, 4.5)
	}
}

var selectorRe = regexp.MustCompile(`([^{}]+)\{`)
var commentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)

// selectors is every selector in the sheet, one entry per comma separated
// part, at-rule preludes left out.
func selectors(sheet string) []string {
	var out []string
	for _, m := range selectorRe.FindAllStringSubmatch(commentRe.ReplaceAllString(sheet, ""), -1) {
		list := strings.TrimSpace(m[1])
		if strings.HasPrefix(list, "@") {
			continue
		}
		for _, part := range strings.Split(list, ",") {
			out = append(out, strings.Join(strings.Fields(part), " "))
		}
	}
	return out
}

// Every rule that decides something for the dark scheme has to decide it
// for OLED too, or OLED quietly draws the light scheme's version of it. A
// dark rule's OLED twin is the same selector with the theme swapped, an OLED
// rule has a system-oled twin for a dark device, and a system-dark rule that
// excludes dim has to exclude both OLED values, because it outranks the plain
// OLED selector on specificity and would paint OLED in dark's greys.
func TestEverySchemeRuleKnowsOLED(t *testing.T) {
	all := selectors(bundledSheet(t))
	have := map[string]bool{}
	for _, s := range all {
		have[s] = true
	}
	twins := 0
	for _, s := range all {
		if suffix, ok := strings.CutPrefix(s, darkRule); ok {
			twins++
			if !have[oledRule+suffix] {
				t.Errorf("%q has no OLED twin %q", s, oledRule+suffix)
			}
		}
		if suffix, ok := strings.CutPrefix(s, oledRule); ok && !have[systemOLEDRule+suffix] {
			t.Errorf("%q has no system-oled twin %q", s, systemOLEDRule+suffix)
		}
		if strings.Contains(s, `:not([data-theme="dim"])`) &&
			(!strings.Contains(s, `:not([data-theme="oled"])`) || !strings.Contains(s, `:not([data-theme="system-oled"])`)) {
			t.Errorf("%q excludes dim but not both OLED values", s)
		}
	}
	if twins < 3 {
		t.Fatalf("found %d dark rules; the selector reader is not reading the sheet", twins)
	}
}
