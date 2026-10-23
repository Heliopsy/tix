// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"
	"testing"
)

// The encoder itself is internal/qr, and its own tests pin it against an
// independent encoder across every version this can produce. What is left
// here is the markup: the part internal/qr does not draw.
func TestQRRefusesWhatItCannotHold(t *testing.T) {
	t.Parallel()
	if _, err := qrSVG(strings.Repeat("x", 214), "too much"); err == nil {
		t.Fatal("a payload past version ten was encoded anyway")
	}
	if _, err := qrSVG(strings.Repeat("x", 213), "the largest"); err != nil {
		t.Fatalf("the largest payload version ten holds was refused: %v", err)
	}
}

func TestQRSVGIsSelfContainedMarkup(t *testing.T) {
	t.Parallel()
	svg, err := qrSVG("otpauth://totp/tix:alice@example.test?secret=JBSWY3DPEHPK3PXP", `Scan "me"`)
	if err != nil {
		t.Fatalf("drawing: %v", err)
	}
	markup := string(svg)
	for _, want := range []string{`<svg class="qr"`, `viewBox="0 0 41 41"`, `role="img"`, `<path fill="#000000"`} {
		if !strings.Contains(markup, want) {
			t.Errorf("the markup carries no %s:\n%s", want, markup)
		}
	}
	if strings.Contains(markup, `aria-label="Scan "me""`) {
		t.Error("the label was not escaped into the attribute")
	}
	if strings.Contains(markup, "<script") || strings.Contains(markup, "<image") ||
		strings.Contains(markup, "href=") {
		t.Error("the drawing reaches outside the page")
	}
}
