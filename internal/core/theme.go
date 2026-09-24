// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
)

// Theme is the accent a tenant presents itself with, on every surface.
//
// Two colours rather than a role per element. The web interface already has
// light, dark and dim for everything structural, and a terminal cannot honour
// a browser's palette anyway; what a tenant actually wants to choose is the
// accent, and one accent plus the tint behind it is enough to carry a brand
// through a header, a selected row and a reference.
//
// Colours that mean something rather than brand something are not in here.
// State category and priority keep their own, because a tenant that themes
// itself red should not lose the red that means blocked.
type Theme struct {
	Name string `json:"name" yaml:"name"`
	// Accent is the brand colour, as #rrggbb.
	Accent string `json:"accent" yaml:"accent"`
	// AccentSoft is the tint it sits on, as #rrggbb.
	AccentSoft string `json:"accent_soft" yaml:"accent_soft"`
	// BuiltIn reports whether this theme ships with tix rather than coming
	// from configuration. Listing says which, so an operator can tell what
	// they can redefine.
	BuiltIn bool `json:"built_in" yaml:"built_in"`
}

// DefaultThemeName is the theme used when nothing else resolves.
const DefaultThemeName = "default"

// builtInThemes is the palette set that ships with tix, in display order.
//
// The first is the colour the sign-in screen has always been, and it has to
// stay equal to the stylesheet's own --accent default, because the inline
// style block always overrides that token: if the two drifted, the value in
// the stylesheet would never actually be seen.
var builtInThemes = []Theme{
	{Name: DefaultThemeName, Accent: "#0f766e", AccentSoft: "#e6f4f1", BuiltIn: true},
	{Name: "indigo", Accent: "#3b5bdb", AccentSoft: "#edf0ff", BuiltIn: true},
	{Name: "forest", Accent: "#2b8a3e", AccentSoft: "#e9f7ec", BuiltIn: true},
	{Name: "ember", Accent: "#c2410c", AccentSoft: "#fdf0e8", BuiltIn: true},
	{Name: "violet", Accent: "#7048e8", AccentSoft: "#f1ecfd", BuiltIn: true},
	{Name: "ocean", Accent: "#0b7285", AccentSoft: "#e6f4f6", BuiltIn: true},
	{Name: "plum", Accent: "#a61e4d", AccentSoft: "#fbeaf0", BuiltIn: true},
	{Name: "slate", Accent: "#334155", AccentSoft: "#eef2f6", BuiltIn: true},
}

// derivedAccents is the set an unthemed tenant's colour is drawn from.
//
// It is the built-in list minus the default, and the order is fixed, because
// the choice is a hash of the tenant's own identity: reordering this slice
// repaints every unthemed tenant in every deployment that upgrades.
var derivedAccents = builtInThemes[1:7]

// ThemeRegistry resolves theme names. Configuration themes are merged over the
// built-ins, so an operator can redefine a shipped name without a patched
// binary, and a name that exists in neither does not resolve.
type ThemeRegistry struct {
	byName map[string]Theme
}

// NewThemeRegistry builds a registry from the built-in themes plus the given
// custom ones, which win on a name collision.
//
// Every custom theme is validated here rather than at the point of use. The
// values end up inside a stylesheet, where escaping is suppressed, so this is
// the boundary that keeps a configuration file out of the CSS.
func NewThemeRegistry(custom []Theme) (*ThemeRegistry, error) {
	byName := make(map[string]Theme, len(builtInThemes)+len(custom))
	for _, t := range builtInThemes {
		byName[t.Name] = t
	}
	for _, t := range custom {
		name := strings.ToLower(strings.TrimSpace(t.Name))
		if name == "" {
			return nil, fmt.Errorf("theme: a custom theme has no name")
		}
		if err := validateHexColor(name, "accent", t.Accent); err != nil {
			return nil, err
		}
		if err := validateHexColor(name, "accent_soft", t.AccentSoft); err != nil {
			return nil, err
		}
		byName[name] = Theme{
			Name: name, Accent: strings.ToLower(t.Accent),
			AccentSoft: strings.ToLower(t.AccentSoft), BuiltIn: false,
		}
	}
	return &ThemeRegistry{byName: byName}, nil
}

// validateHexColor refuses anything that is not #rrggbb.
//
// Hex rather than a colour name or rgb(), because the terminal interface has
// to parse the same value and lipgloss takes hex.
func validateHexColor(theme, field, v string) error {
	if len(v) != 7 || v[0] != '#' {
		return fmt.Errorf("theme %q: %s %q is not #rrggbb", theme, field, v)
	}
	for _, r := range v[1:] {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !isHex {
			return fmt.Errorf("theme %q: %s %q is not #rrggbb", theme, field, v)
		}
	}
	return nil
}

// Lookup returns the named theme, reporting whether it resolved.
func (r *ThemeRegistry) Lookup(name string) (Theme, bool) {
	if r == nil {
		return Theme{}, false
	}
	t, ok := r.byName[strings.ToLower(strings.TrimSpace(name))]
	return t, ok
}

// Default returns the theme used when nothing has been named.
func (r *ThemeRegistry) Default() Theme {
	if t, ok := r.Lookup(DefaultThemeName); ok {
		return t
	}
	return builtInThemes[0]
}

// List returns every resolvable theme, built-ins first and then configured
// ones, each group by name, so the order does not depend on map iteration.
func (r *ThemeRegistry) List() []Theme {
	if r == nil {
		return nil
	}
	var built, custom []Theme
	for _, t := range r.byName {
		if t.BuiltIn {
			built = append(built, t)
		} else {
			custom = append(custom, t)
		}
	}
	sort.Slice(built, func(i, j int) bool { return built[i].Name < built[j].Name })
	sort.Slice(custom, func(i, j int) bool { return custom[i].Name < custom[j].Name })
	return append(built, custom...)
}

// Resolve returns the theme a tenant presents itself with.
//
// The three cases are deliberately different. A named theme that resolves is
// used. A tenant that names nothing gets a colour derived from its own
// identity, which is what it had before any theme could be named, so an
// upgrade does not repaint every deployment. A name that no longer resolves
// falls back to the default rather than failing, because a configuration file
// that stopped defining a theme must not take the board down; the write path
// is where a typo is refused, since that is the moment someone can fix it.
func (r *ThemeRegistry) Resolve(t *Tenant) Theme {
	if r == nil {
		return builtInThemes[0]
	}
	if t == nil {
		return r.Default()
	}
	if name := strings.TrimSpace(t.Theme); name != "" {
		if found, ok := r.Lookup(name); ok {
			return found
		}
		return r.Default()
	}
	return DerivedTheme(t)
}

// DerivedTheme is the accent an unthemed tenant carries, hashed from its own
// identity so it is stable for the life of the tenant.
func DerivedTheme(t *Tenant) Theme {
	if t == nil || strings.TrimSpace(t.Name) == "" {
		return builtInThemes[0]
	}
	sum := fnv.New32a()
	_, _ = sum.Write([]byte(t.Key + t.ID))
	picked := derivedAccents[int(sum.Sum32())%len(derivedAccents)]
	// Named for the tenant rather than for the palette entry: nobody chose
	// this, so reporting it as "indigo" would imply somebody did.
	return Theme{Name: "", Accent: picked.Accent, AccentSoft: picked.AccentSoft, BuiltIn: true}
}

// ThemeNames lists the built-in theme names, for shell completion.
//
// Built-ins only: completion runs in a shell that has not resolved this
// deployment's configuration, and offering a name that does not exist here is
// worse than offering fewer. `tix theme ls` is the complete answer.
func ThemeNames() []string {
	out := make([]string, 0, len(builtInThemes))
	for _, t := range builtInThemes {
		out = append(out, t.Name)
	}
	sort.Strings(out)
	return out
}
