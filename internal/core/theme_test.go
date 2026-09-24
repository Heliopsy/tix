// SPDX-License-Identifier: AGPL-3.0-or-later

package core_test

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// TestThemeColoursAreRefusedUnlessTheyAreHex pins the one validation that
// matters. Both values end up inside a <style> block, where template.CSS
// suppresses escaping, so this regexp is the whole distance between a
// configuration file and CSS injection.
func TestThemeColoursAreRefusedUnlessTheyAreHex(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		accent     string
		accentSoft string
		wantOK     bool
	}{
		{"six digits", "#0f766e", "#e6f4f1", true},
		{"uppercase is fine", "#0F766E", "#E6F4F1", true},
		{"no hash", "0f766e", "#e6f4f1", false},
		{"three digits", "#fff", "#e6f4f1", false},
		{"eight digits", "#0f766eff", "#e6f4f1", false},
		{"not hex", "#zzzzzz", "#e6f4f1", false},
		{"empty", "", "#e6f4f1", false},
		{"a colour name", "rebeccapurple", "#e6f4f1", false},
		{"rgb()", "rgb(15,118,110)", "#e6f4f1", false},
		{"soft is checked too", "#0f766e", "red", false},
		{"closes the style block", "#0f766e", "#fff}</style><script>", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := core.NewThemeRegistry([]core.Theme{
				{Name: "custom", Accent: tc.accent, AccentSoft: tc.accentSoft},
			})
			if tc.wantOK && err != nil {
				t.Fatalf("NewThemeRegistry(%q, %q) = %v, want no error", tc.accent, tc.accentSoft, err)
			}
			if !tc.wantOK && err == nil {
				t.Fatalf("NewThemeRegistry(%q, %q) succeeded, want a refusal", tc.accent, tc.accentSoft)
			}
		})
	}
}

func TestThemeRegistryResolvesBuiltInsAndConfiguration(t *testing.T) {
	t.Parallel()
	r, err := core.NewThemeRegistry([]core.Theme{
		{Name: "acme", Accent: "#7c3aed", AccentSoft: "#f3eeff"},
	})
	if err != nil {
		t.Fatalf("NewThemeRegistry: %v", err)
	}

	if _, ok := r.Lookup("indigo"); !ok {
		t.Error("a built-in theme did not resolve")
	}
	acme, ok := r.Lookup("acme")
	if !ok {
		t.Fatal("the configured theme did not resolve")
	}
	if acme.Accent != "#7c3aed" || acme.BuiltIn {
		t.Errorf("configured theme = %+v, want the configured accent and BuiltIn false", acme)
	}
	if _, ok := r.Lookup("nothing-defines-this"); ok {
		t.Error("an undefined name resolved")
	}
}

// TestConfigurationRedefinesABuiltIn is the escape hatch that lets an operator
// change a shipped palette without a patched binary.
func TestConfigurationRedefinesABuiltIn(t *testing.T) {
	t.Parallel()
	r, err := core.NewThemeRegistry([]core.Theme{
		{Name: core.DefaultThemeName, Accent: "#abcdef", AccentSoft: "#fedcba"},
	})
	if err != nil {
		t.Fatalf("NewThemeRegistry: %v", err)
	}
	got := r.Default()
	if got.Accent != "#abcdef" {
		t.Errorf("default accent = %q, want the configured one", got.Accent)
	}
	if got.BuiltIn {
		t.Error("a redefined theme still reports itself as built in")
	}
}

func TestThemeListNamesItsSources(t *testing.T) {
	t.Parallel()
	r, err := core.NewThemeRegistry([]core.Theme{
		{Name: "acme", Accent: "#7c3aed", AccentSoft: "#f3eeff"},
	})
	if err != nil {
		t.Fatalf("NewThemeRegistry: %v", err)
	}
	list := r.List()
	if len(list) < 2 {
		t.Fatalf("List() returned %d themes, want the built-ins and the configured one", len(list))
	}
	var sawBuiltIn, sawCustom bool
	for _, th := range list {
		if th.BuiltIn {
			sawBuiltIn = true
			continue
		}
		sawCustom = true
		if th.Name != "acme" {
			t.Errorf("unexpected configured theme %q", th.Name)
		}
	}
	if !sawBuiltIn || !sawCustom {
		t.Errorf("List() = %v, want both sources represented", list)
	}
	// Built-ins first, then configured, each sorted: map iteration order must
	// not reach a listing someone reads.
	if list[len(list)-1].BuiltIn {
		t.Error("a built-in theme sorted after a configured one")
	}
}

// TestUnnamedTenantKeepsItsDerivedColour is the compatibility guarantee. An
// upgrade must not repaint a deployment, so an empty theme resolves to the
// same hash-derived accent the web interface used before themes existed.
func TestUnnamedTenantKeepsItsDerivedColour(t *testing.T) {
	t.Parallel()
	r, err := core.NewThemeRegistry(nil)
	if err != nil {
		t.Fatalf("NewThemeRegistry: %v", err)
	}
	tenant := &core.Tenant{ID: "01ABC", Key: "acme", Name: "Acme"}

	first := r.Resolve(tenant)
	second := r.Resolve(tenant)
	if first != second {
		t.Errorf("resolving twice gave %+v then %+v, want a stable colour", first, second)
	}
	if first.Accent == "" {
		t.Error("an unthemed tenant resolved to no accent")
	}
	if first.Name != "" {
		t.Errorf("derived theme reported name %q, want empty: nobody chose it", first.Name)
	}

	other := &core.Tenant{ID: "01XYZ", Key: "globex", Name: "Globex"}
	if r.Resolve(other).Accent == first.Accent {
		// Not a correctness failure, but the whole point of deriving is that
		// two tenants usually look different; a palette this small collides
		// eventually, so this only guards the obvious bug of ignoring input.
		t.Log("two tenants derived the same accent; check the hash actually reads the key and id")
	}
}

func TestNamedThemeWins(t *testing.T) {
	t.Parallel()
	r, err := core.NewThemeRegistry(nil)
	if err != nil {
		t.Fatalf("NewThemeRegistry: %v", err)
	}
	tenant := &core.Tenant{ID: "01ABC", Key: "acme", Name: "Acme", Theme: "ocean"}
	ocean, _ := r.Lookup("ocean")
	if got := r.Resolve(tenant); got.Accent != ocean.Accent {
		t.Errorf("resolved accent = %q, want ocean's %q", got.Accent, ocean.Accent)
	}
}

// TestAThemeThatStoppedBeingDefinedDoesNotBreakThePage covers the split the
// design makes on purpose: a write refuses an unknown name, because that is
// when a typo can be fixed, but a read falls back, because a configuration
// file that stopped defining a theme must not take the board down.
func TestAThemeThatStoppedBeingDefinedDoesNotBreakThePage(t *testing.T) {
	t.Parallel()
	r, err := core.NewThemeRegistry(nil)
	if err != nil {
		t.Fatalf("NewThemeRegistry: %v", err)
	}
	tenant := &core.Tenant{ID: "01ABC", Key: "acme", Name: "Acme", Theme: "was-in-the-config-yesterday"}
	got := r.Resolve(tenant)
	if got.Accent != r.Default().Accent {
		t.Errorf("resolved accent = %q, want the default %q", got.Accent, r.Default().Accent)
	}
}

func TestResolveToleratesNoTenant(t *testing.T) {
	t.Parallel()
	r, err := core.NewThemeRegistry(nil)
	if err != nil {
		t.Fatalf("NewThemeRegistry: %v", err)
	}
	if got := r.Resolve(nil); got.Accent != r.Default().Accent {
		t.Errorf("Resolve(nil) = %+v, want the default theme", got)
	}
	// The sign-in screen has no tenant, and it is the one page everybody sees.
	var nilRegistry *core.ThemeRegistry
	if got := nilRegistry.Resolve(nil); got.Accent == "" {
		t.Error("a nil registry resolved to no accent")
	}
}

func TestCustomThemeNeedsAName(t *testing.T) {
	t.Parallel()
	_, err := core.NewThemeRegistry([]core.Theme{
		{Name: "  ", Accent: "#0f766e", AccentSoft: "#e6f4f1"},
	})
	if err == nil {
		t.Fatal("a nameless theme was accepted")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error = %q, want it to mention the missing name", err)
	}
}
