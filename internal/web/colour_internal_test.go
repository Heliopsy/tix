// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// schemeBlock is the body of the stylesheet rule that opens with selector.
func schemeBlock(t *testing.T, sheet, selector string) string {
	t.Helper()
	at := strings.Index(sheet, selector+" {")
	if at < 0 {
		t.Fatalf("app.css has no %q block", selector)
	}
	body := sheet[at+len(selector)+2:]
	return body[:strings.Index(body, "\n}")]
}

// token reads one custom property's #rrggbb out of a rule body.
func cssHex(t *testing.T, block, name string) core.RGB {
	t.Helper()
	m := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(name) + `:\s*(#[0-9a-fA-F]{6});`).FindStringSubmatch(block)
	if m == nil {
		t.Fatalf("the block has no %s", name)
	}
	rgb, _ := core.ParseHex(m[1])
	return rgb
}

// The backgrounds a custom colour is shaded against are copies of the
// stylesheet's. Read them out of it, so a retuned scheme cannot leave the
// shading aiming at a page that no longer exists.
func TestColourSchemesMatchTheStylesheet(t *testing.T) {
	raw, err := os.ReadFile("assets/app.css")
	if err != nil {
		t.Fatal(err)
	}
	sheet := string(raw)
	blocks := map[string]string{
		"light": schemeBlock(t, sheet, ":root"),
		"dim":   schemeBlock(t, sheet, `:root[data-theme="dim"]`),
		"dark":  schemeBlock(t, sheet, `:root[data-theme="dark"]`),
	}
	for _, s := range colourSchemes {
		block := blocks[s.Name]
		if got := cssHex(t, block, "--bg"); got != s.Page {
			t.Errorf("%s page = %s, app.css says %s", s.Name, s.Page.Hex(), got.Hex())
		}
		if got := cssHex(t, block, "--surface"); got != s.Surface {
			t.Errorf("%s surface = %s, app.css says %s", s.Name, s.Surface.Hex(), got.Hex())
		}
	}
	for _, c := range core.ProjectColors() {
		shade, _ := core.PaletteShade(c)
		if got := cssHex(t, blocks["light"], "--proj-"+string(c)); got != shade {
			t.Errorf("core's shade of %s is %s, app.css says %s", c, shade.Hex(), got.Hex())
		}
	}
	for _, s := range colourSchemes {
		if !strings.Contains(sheet, "var("+s.Var+")") {
			t.Errorf("app.css never reads %s, so the %s scheme draws no custom colour", s.Var, s.Name)
		}
	}
}

// sweep is a spread of custom colours, the extremes included.
func sweep() []core.RGB {
	var out []core.RGB
	for _, r := range []uint8{0, 0x40, 0x80, 0xc0, 0xff} {
		for _, g := range []uint8{0, 0x55, 0xaa, 0xff} {
			for _, b := range []uint8{0, 0x80, 0xff} {
				out = append(out, core.RGB{R: r, G: g, B: b})
			}
		}
	}
	return out
}

// A custom colour has to read on every scheme the way a palette token does,
// and the words on a row tinted by it have to stay readable too.
func TestEveryCustomColourReadsOnEveryScheme(t *testing.T) {
	ink := map[string]core.RGB{
		"light": {R: 0x0f, G: 0x13, B: 0x19},
		"dim":   {R: 0xc7, G: 0xcc, B: 0xd3},
		"dark":  {R: 0xe7, G: 0xea, B: 0xee},
	}
	for _, s := range colourSchemes {
		for _, c := range sweep() {
			shade := schemeShade(c, s)
			for _, bg := range []core.RGB{s.Page, s.Surface} {
				if got := core.Contrast(shade, bg); got < projectColourFloor {
					t.Errorf("%s: %s drawn as %s reaches %.2f on %s", s.Name, c.Hex(), shade.Hex(), got, bg.Hex())
				}
			}
			wash := s.Surface.Mix(shade, 0.12)
			if got := core.Contrast(ink[s.Name], wash); got < projectColourFloor {
				t.Errorf("%s: text on the %s wash reaches %.2f, under %.1f", s.Name, c.Hex(), got, projectColourFloor)
			}
		}
	}
	if got := schemeShade(core.RGB{R: 0x1a, G: 0x7f, B: 0x5a}, colourSchemes[0]); got.Hex() != "#1a7f5a" {
		t.Errorf("a colour that already reads on light was redrawn as %s", got.Hex())
	}
}

// Nothing but a formatted colour reaches the style attribute.
func TestProjStyleCarriesOnlyFormattedShades(t *testing.T) {
	want := regexp.MustCompile(`^--proj-l: #[0-9a-f]{6}; --proj-m: #[0-9a-f]{6}; --proj-d: #[0-9a-f]{6}$`)
	if got := string(projStyle("#1a7f5a")); !want.MatchString(got) {
		t.Errorf("projStyle(custom) = %q", got)
	}
	for _, c := range []core.ProjectColor{"", "green", "#1A7F5A", "#1a7f5a;color:red", "#1a7", "</style>"} {
		if got := projStyle(c); got != "" {
			t.Errorf("projStyle(%q) = %q, want nothing", c, got)
		}
	}
	classes := map[core.ProjectColor]string{"": "", "green": "proj-green", "#1a7f5a": "proj-custom",
		"chartreuse": "", "#1a7f5a;x": ""}
	for c, want := range classes {
		if got := projClass(c); got != want {
			t.Errorf("projClass(%q) = %q, want %q", c, got, want)
		}
	}
}

// The custom radio defers to the hex box; anything else is taken as posted.
func TestColourFieldReadsTheChoiceBack(t *testing.T) {
	tests := []struct {
		form url.Values
		want string
	}{
		{url.Values{"color": {"teal"}, "color_custom": {"#123456"}}, "teal"},
		{url.Values{"color": {""}, "color_custom": {"#123456"}}, ""},
		{url.Values{"color": {"custom"}, "color_custom": {" #1A7F5A "}}, "#1A7F5A"},
		{url.Values{"color": {"custom"}, "color_custom": {"1a7f5a"}}, "#1a7f5a"},
		{url.Values{"color": {"custom"}, "color_custom": {""}}, "custom"},
	}
	for _, tc := range tests {
		r := httptest.NewRequest("POST", "/", strings.NewReader(tc.form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if got := colourField(r, "color"); got != tc.want {
			t.Errorf("colourField(%v) = %q, want %q", tc.form, got, tc.want)
		}
	}
}
