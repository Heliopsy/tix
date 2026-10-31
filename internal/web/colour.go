// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// colourScheme is one colour scheme's page and surface, the two backgrounds a
// project colour is drawn on. They mirror --bg and --surface in app.css,
// which colour_test.go reads to keep them equal.
type colourScheme struct {
	Name     string
	Var      string
	Page     core.RGB
	Surface  core.RGB
	Selector string
}

// colourSchemes are the schemes a custom colour gets a shade for, each
// written into its own custom property on the element.
var colourSchemes = []colourScheme{
	{Name: "light", Var: "--proj-l", Page: core.RGB{R: 0xff, G: 0xff, B: 0xff}, Surface: core.RGB{R: 0xf6, G: 0xf7, B: 0xf9}},
	{Name: "dim", Var: "--proj-m", Page: core.RGB{R: 0x1d, G: 0x21, B: 0x26}, Surface: core.RGB{R: 0x23, G: 0x28, B: 0x2e}},
	{Name: "dark", Var: "--proj-d", Page: core.RGB{R: 0x14, G: 0x17, B: 0x1b}, Surface: core.RGB{R: 0x1b, G: 0x1f, B: 0x24}},
}

// projectColourFloor is the contrast every palette token clears against its
// scheme's page and surface, and so what a custom colour is shaded to clear.
const projectColourFloor = 4.5

// customClass is the class a custom colour is drawn through; app.css picks
// one of the per-scheme shades into --proj under it.
const customClass = "proj-custom"

// projClass names the class that draws a project colour: the palette token's
// own, the custom one, or none.
func projClass(c core.ProjectColor) string {
	switch {
	case c == core.ColorNone || !c.Valid():
		return ""
	case c.IsCustom():
		return customClass
	default:
		return "proj-" + string(c)
	}
}

// projStyle is the declarations that carry a custom colour's per-scheme
// shades, empty for anything else. Every value in it was formatted here from
// a parsed colour, which is what makes it safe to hand the stylesheet.
func projStyle(c core.ProjectColor) template.CSS {
	rgb, ok := c.RGB()
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(colourSchemes))
	for _, s := range colourSchemes {
		parts = append(parts, s.Var+": "+schemeShade(rgb, s).Hex())
	}
	return template.CSS(strings.Join(parts, "; ")) // #nosec G203 -- formatted from a parsed #rrggbb
}

// schemeShade is the shade of a custom colour one scheme draws.
func schemeShade(c core.RGB, s colourScheme) core.RGB {
	return core.Readable(c, projectColourFloor, s.Page, s.Surface)
}

// colourOption is one predefined choice of a colour field.
type colourOption struct {
	Value    string
	Label    string
	Swatch   template.CSS
	Selected bool
	// Configured marks a theme that comes from configuration rather than
	// shipping with tix.
	Configured bool
}

// colourFieldView is what the shared colour-field partial draws: predefined
// swatches, optionally none or a default, and a custom colour.
type colourFieldView struct {
	Name    string
	None    bool
	Default bool
	Value   string
	Options []colourOption
	// Custom is the current value when it is a custom colour, which the hex
	// field shows; Picker is what the native colour input starts on.
	Custom string
	Picker template.CSS
}

// pickerStart is what the native colour input shows before a custom colour
// has been chosen: the palette's slate, a neutral starting point.
const pickerStart = "#5e6e85"

// projectColourField builds the colour field for a project form.
func projectColourField(name string, current core.ProjectColor) colourFieldView {
	v := colourFieldView{Name: name, None: true, Value: string(current)}
	for _, c := range core.ProjectColors() {
		v.Options = append(v.Options, colourOption{Value: string(c), Label: string(c),
			Swatch: template.CSS("var(--proj-" + string(c) + ")"), Selected: c == current}) // #nosec G203 -- a palette token
	}
	v.withCustom(current.IsCustom(), string(current))
	return v
}

// themeColourField builds the colour field for the tenant theme, offering
// every resolvable theme by its accent and a custom accent.
func themeColourField(name, current string, themes []core.Theme) colourFieldView {
	current = strings.ToLower(strings.TrimSpace(current))
	v := colourFieldView{Name: name, Default: true, Value: current}
	for _, t := range themes {
		if _, ok := core.ParseHex(t.Accent); !ok {
			continue
		}
		v.Options = append(v.Options, colourOption{Value: t.Name, Label: t.Name,
			Swatch: template.CSS(t.Accent), Selected: t.Name == current, Configured: !t.BuiltIn}) // #nosec G203 -- parsed above
	}
	_, custom := core.CustomTheme(current)
	v.withCustom(custom, current)
	return v
}

func (v *colourFieldView) withCustom(custom bool, value string) {
	v.Picker = template.CSS(pickerStart) // #nosec G203 -- a constant
	if custom {
		v.Custom = value
		v.Picker = template.CSS(value) // #nosec G203 -- a validated #rrggbb
	}
}

// customChoice is the value the custom radio posts; the colour itself comes
// from the hex field beside it.
const customChoice = "custom"

// colourField reads a colour field back: the chosen predefined value, or the
// hex field when custom was chosen. A hex typed without its # gets one. An
// empty hex field under custom is sent on as "custom", so the service refuses
// it by name rather than quietly clearing the colour.
func colourField(r *http.Request, name string) string {
	choice := field(r, name)
	if choice != customChoice {
		return choice
	}
	hex := strings.TrimSpace(field(r, name+"_custom"))
	if hex == "" {
		return customChoice
	}
	if !strings.HasPrefix(hex, "#") {
		hex = "#" + hex
	}
	return hex
}
