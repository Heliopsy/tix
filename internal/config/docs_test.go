package config_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/heliopsy/tix/internal/config"
)

// docsKeyRow matches one row of the key table in docs/configuration.md.
var docsKeyRow = regexp.MustCompile("(?m)^\\| `([a-z0-9_.]+)` \\| `(TIX_[A-Z0-9_]+)` \\|")

// TestDocumentedKeysMatchTheRealOnes ties the key table in
// docs/configuration.md to config.Keys(), which is derived by reflection over
// Config. Without it a new field is a silently undocumented setting, which is
// how allow_network_fs, allow_private_targets and tui.keymap went missing.
func TestDocumentedKeysMatchTheRealOnes(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "configuration.md")
	body, err := os.ReadFile(path) // #nosec G304 -- a fixed path inside the repository
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	documented := map[string]string{}
	for _, row := range docsKeyRow.FindAllStringSubmatch(string(body), -1) {
		documented[row[1]] = row[2]
	}
	if len(documented) == 0 {
		t.Fatalf("%s has no key table rows; the parser or the table changed shape", path)
	}

	real := map[string]string{}
	for _, k := range config.Keys() {
		real[k.Path] = k.Env
		switch env, ok := documented[k.Path]; {
		case !ok:
			t.Errorf("key %q is not in the table in %s", k.Path, path)
		case env != k.Env:
			t.Errorf("%s documents %q as %s, want %s", path, k.Path, env, k.Env)
		}
	}
	for path2 := range documented {
		if _, ok := real[path2]; !ok {
			t.Errorf("%s documents %q, which is not a configuration key", path, path2)
		}
	}
}
