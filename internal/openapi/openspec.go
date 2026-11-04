// SPDX-License-Identifier: AGPL-3.0-or-later

package openapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/wire"
)

// HTTPSpec is the OpenSpec capability that specifies the HTTP API, where a
// route may be written relative to the API prefix.
const HTTPSpec = "http-api"

var mention = regexp.MustCompile("\\b(GET|POST|PUT|PATCH|DELETE) +`?(/[^\\s`'\")\\]]*)")

// Mentions returns every route a spec text names, as "METHOD /path" with the
// query and trailing punctuation dropped. Relative paths are taken as under
// the API prefix when relative is set, and skipped otherwise, because outside
// the HTTP API's own spec a bare path is usually a browser screen.
func Mentions(text string, relative bool) []string {
	seen := map[string]bool{}
	for _, m := range mention.FindAllStringSubmatch(text, -1) {
		path, _, _ := strings.Cut(m[2], "?")
		path = strings.TrimRight(path, ".,;:")
		switch {
		case strings.HasPrefix(path, wire.APIPrefix+"/"), path == wire.RouteHealth, path == wire.RouteReady:
		case relative:
			path = wire.APIPrefix + path
		default:
			continue
		}
		seen[m[1]+" "+path] = true
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// readOpenSpec returns the routes each OpenSpec capability mentions.
func readOpenSpec(root string) (map[string][]string, error) {
	dir := filepath.Join(root, "openspec", "specs")
	caps, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", dir, err)
	}
	out := map[string][]string{}
	for _, c := range caps {
		if !c.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, c.Name(), "spec.md")) // #nosec G304 -- a file of this repository
		if err != nil {
			continue
		}
		out[c.Name()] = Mentions(string(b), c.Name() == HTTPSpec)
	}
	return out, nil
}

// Matcher resolves a concrete or templated "METHOD /path" to the pattern of
// the document that serves it, with the same precedence the server's mux
// applies.
type Matcher struct{ mux *http.ServeMux }

// NewMatcher builds a matcher over "METHOD /pattern" keys.
func NewMatcher(patterns []string) (*Matcher, error) {
	mux := http.NewServeMux()
	nop := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	var err error
	for _, p := range patterns {
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("registering %q: %v", p, r)
				}
			}()
			mux.Handle(p, nop)
		}()
	}
	return &Matcher{mux: mux}, err
}

// Match returns the pattern serving method and path, or "" for none.
func (m *Matcher) Match(method, path string) string {
	req, err := http.NewRequest(method, "http://matcher"+path, nil)
	if err != nil {
		return ""
	}
	_, pattern := m.mux.Handler(req)
	if !strings.HasPrefix(pattern, method+" ") {
		return ""
	}
	return pattern
}
