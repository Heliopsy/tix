// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// placeholderRe finds the {name} segments of a route pattern.
var placeholderRe = regexp.MustCompile(`\{([a-z]+)\}`)

// shotManifest is the part of scripts/screenshots/routes.json the guard reads.
type shotManifest struct {
	Params map[string]json.RawMessage `json:"params"`
	Routes []struct {
		Pattern string            `json:"pattern"`
		Skip    string            `json:"skip"`
		Params  map[string]string `json:"params"`
	} `json:"routes"`
}

// TestScreenshotManifestCoversEveryScreen holds `just screenshots` to the
// route registry: a GET route the manifest does not list would never be
// captured, and one it lists that no longer exists would be captured as an
// error page.
func TestScreenshotManifestCoversEveryScreen(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "scripts", "screenshots", "routes.json"))
	if err != nil {
		t.Fatalf("reading the screenshot manifest: %v", err)
	}
	var m shotManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parsing the screenshot manifest: %v", err)
	}
	listed := map[string]bool{}
	for _, r := range m.Routes {
		if listed[r.Pattern] {
			t.Errorf("manifest lists %s twice", r.Pattern)
		}
		listed[r.Pattern] = true
		for _, ph := range placeholderRe.FindAllStringSubmatch(r.Pattern, -1) {
			if r.Skip == "" && r.Params[ph[1]] == "" {
				t.Errorf("manifest route %s has no resolver for {%s}", r.Pattern, ph[1])
			}
		}
		for placeholder, resolver := range r.Params {
			if _, ok := m.Params[resolver]; !ok {
				t.Errorf("manifest route %s fills {%s} from %q, which params does not define", r.Pattern, placeholder, resolver)
			}
		}
	}
	served := map[string]bool{}
	for _, b := range web.Bindings() {
		if b.Method != http.MethodGet {
			continue
		}
		served[b.Pattern] = true
		if !listed[b.Pattern] {
			t.Errorf("GET %s is missing from scripts/screenshots/routes.json; list it, with a skip reason if it is not a screen", b.Pattern)
		}
	}
	for p := range listed {
		if !served[p] {
			t.Errorf("scripts/screenshots/routes.json lists %s, which is not a GET route", p)
		}
	}
}
