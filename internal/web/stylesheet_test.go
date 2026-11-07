// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// stylesheetLicence opens every part of the stylesheet.
const stylesheetLicence = "/* SPDX-License-Identifier: AGPL-3.0-or-later */\n"

// stylesheetPart is the name a part of the stylesheet takes: its place in the
// cascade, then the feature it styles.
var stylesheetPart = regexp.MustCompile(`^[0-9]{3}-[a-z0-9]+(-[a-z0-9]+)*\.css$`)

// stylesheetParts reads assets/css in name order, which is cascade order.
func stylesheetParts(t *testing.T) (names, parts []string) {
	t.Helper()
	found, err := filepath.Glob("assets/css/*.css")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) < 2 {
		t.Fatalf("found %d stylesheet parts under assets/css", len(found))
	}
	for _, name := range found {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, filepath.Base(name))
		parts = append(parts, string(raw))
	}
	return names, parts
}

// Each part closes the blocks it opens. A part that leaves a media query open
// swallows every rule in the parts after it, and the bundle as a whole can
// still balance when a later part closes one too many.
func TestEveryStylesheetPartClosesItsOwnBlocks(t *testing.T) {
	t.Parallel()
	names, parts := stylesheetParts(t)
	for i, part := range parts {
		if !stylesheetPart.MatchString(names[i]) {
			t.Errorf("%s is not named NNN-feature.css", names[i])
		}
		if !strings.HasPrefix(part, stylesheetLicence) {
			t.Errorf("%s does not open with its SPDX line", names[i])
		}
		if !strings.HasSuffix(part, "\n") {
			t.Errorf("%s does not end with a newline, so the next part would join its last line", names[i])
		}
		if err := unclosedBlock(part); err != nil {
			t.Errorf("%s: %v", names[i], err)
		}
	}
}

// The served stylesheet is the parts, in name order, joined with only the
// licence lines after the first taken out; no part is served on its own, and
// no stylesheet sits beside them for the bundle to shadow.
func TestTheServedStylesheetIsItsPartsInOrder(t *testing.T) {
	t.Parallel()
	names, parts := stylesheetParts(t)
	want := parts[0]
	for _, part := range parts[1:] {
		want += strings.TrimPrefix(part, stylesheetLicence)
	}
	b := newFixture(t).as("")
	if got := body(t, b.get("/assets/app.css")); got != want {
		t.Fatalf("/assets/app.css is %d bytes, the parts joined in order are %d", len(got), len(want))
	}
	if resp := b.get("/assets/css/" + names[0]); resp.StatusCode != http.StatusNotFound {
		t.Errorf("/assets/css/%s answered %d; parts are served only inside the bundle", names[0], resp.StatusCode)
	}
	loose, err := filepath.Glob("assets/*.css")
	if err != nil {
		t.Fatal(err)
	}
	if len(loose) > 0 {
		t.Errorf("%v sit beside assets/css; styles go in a numbered part", loose)
	}
}
