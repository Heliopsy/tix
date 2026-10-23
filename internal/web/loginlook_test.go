// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"regexp"
	"strings"
	"testing"
)

// The sign-in screen is the one page a tenant's own accent has to survive
// untouched, because it is the first thing anyone sees of an install. Its
// redesign is therefore written in tokens, and these guards read the rules by
// name rather than searching the sheet for a colour that could be anywhere.

func TestSignInCardCarriesTheAccentGlow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("")

	page := body(t, b.get("/login"))
	if !strings.Contains(page, `<div class="card authcard">`) {
		t.Fatalf("the sign-in card carries no authcard class, so none of its rules apply:\n%s", page)
	}

	sheet := body(t, b.get("/assets/app.css"))
	glow := declarations(t, sheet, ".auth .authcard::before")
	if !strings.Contains(glow, "radial-gradient") {
		t.Errorf("the glow is not a radial gradient:\n%s", glow)
	}
	if !strings.Contains(glow, "var(--accent)") {
		t.Errorf("the glow does not resolve the tenant accent, so `tix theme` cannot reach it:\n%s", glow)
	}
	if strings.Contains(glow, "filter:") {
		t.Errorf("the glow softens with a blur filter rather than with its gradient stops:\n%s", glow)
	}
	if !strings.Contains(glow, "z-index: -1") {
		t.Errorf("the glow is not painted behind the card:\n%s", glow)
	}
}

func TestSignInSubmitIsPaintedInTokens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("").get("/assets/app.css"))

	button := declarations(t, sheet, ".authcard form .actions button")
	for _, want := range []string{"background: var(--ink)", "color: var(--surface)", "width: 100%"} {
		if !strings.Contains(button, want) {
			t.Errorf("the submit button is missing %q, so it is not the full-width inverse of the card:\n%s", want, button)
		}
	}
}

// A hard-coded colour in this block would survive `tix theme` and both dark
// schemes while looking correct to whoever wrote it, which is exactly the
// failure the tokens exist to prevent. Shadows are the one exemption: --shadow
// is none in both dark schemes, and the card's lift is not a themed colour.
func TestSignInRedesignHardCodesNoColour(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("").get("/assets/app.css"))

	const marker = "/* ===== Sign-in screen redesign"
	at := strings.Index(sheet, marker)
	if at < 0 {
		t.Fatalf("the sign-in block is not delimited in the stylesheet")
	}
	block := sheet[at:]

	hex := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)
	for _, line := range strings.Split(block, "\n") {
		if strings.Contains(line, "box-shadow") || strings.Contains(line, "rgba(16, 24, 40") {
			continue
		}
		if found := hex.FindString(line); found != "" {
			t.Errorf("the sign-in block hard-codes %s, which no theme can override:\n%s", found, line)
		}
	}
}
