// SPDX-License-Identifier: AGPL-3.0-or-later

package openapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/heliopsy/tix/api"
	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/docsmd"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/openapi"
	"github.com/heliopsy/tix/internal/wire"
)

// built generates the document once for the whole package.
var built = sync.OnceValues(func() (map[string]any, error) {
	root, err := docsmd.Root()
	if err != nil {
		return nil, err
	}
	return openapi.Build(root)
})

// document returns the generated document, failing the test without one.
func document(t *testing.T) map[string]any {
	t.Helper()
	doc, err := built()
	if err != nil {
		t.Fatalf("generating the openapi document: %v", err)
	}
	return doc
}

// operations lists the document's operations as "METHOD /path".
func operations(doc map[string]any) []string {
	var out []string
	for path, item := range doc["paths"].(map[string]any) {
		for method := range item.(map[string]any) {
			out = append(out, strings.ToUpper(method)+" "+path)
		}
	}
	sort.Strings(out)
	return out
}

// TestTheCommittedDocumentIsGenerated fails when api/openapi.yaml or the
// routes table of docs/api.md is not what `just openapi` writes now, which is
// what a route changed without regenerating looks like.
func TestTheCommittedDocumentIsGenerated(t *testing.T) {
	t.Parallel()
	root, err := docsmd.Root()
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t)
	want, err := openapi.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, openapi.SpecFile))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is not what the registry generates; run `just openapi` and commit the result.\n%s",
			openapi.SpecFile, firstDifference(got, want))
	}
	md, err := os.ReadFile(filepath.Join(root, openapi.DocsFile))
	if err != nil {
		t.Fatal(err)
	}
	spliced, err := openapi.Splice(md, openapi.RoutesTable(doc))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(md, spliced) {
		t.Errorf("the routes table of %s is not what the registry generates; run `just openapi`.\n%s",
			openapi.DocsFile, firstDifference(md, spliced))
	}
}

// firstDifference names the first line two texts disagree on.
func firstDifference(got, want []byte) string {
	g, w := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	for i := range max(len(g), len(w)) {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			return "line " + strconv.Itoa(i+1) + ":\n  committed: " + gl + "\n  generated: " + wl
		}
	}
	return "they differ only in length"
}

// TestTheEmbeddedDocumentIsTheCommittedOne fails when the JSON the server
// serves is not the YAML the repository holds.
func TestTheEmbeddedDocumentIsTheCommittedOne(t *testing.T) {
	t.Parallel()
	served, err := api.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var got any
	if err := json.Unmarshal(served, &got); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(document(t))
	if err != nil {
		t.Fatal(err)
	}
	var want any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatal("the served document differs from the generated one; run `just openapi`")
	}
}

// muxRoutes returns every route the real router registers, as "METHOD /path".
// The event stream is registered without a method because the WebSocket
// handler answers the upgrade itself, and an upgrade is a GET by protocol.
func muxRoutes(t *testing.T) []string {
	t.Helper()
	actor := &core.Actor{ID: "u1", TenantID: "t1", Kind: core.ActorUser, Scopes: core.AllScopes}
	nop := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	router, err := httpapi.New(httpapi.Config{
		Service:       stubService{},
		Authenticator: auth.NewStaticAuthenticator(actor),
		Resolver:      stubResolver{},
		EventHandler:  nop,
	})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range router.Patterns() {
		if !strings.Contains(p, " ") {
			p = http.MethodGet + " " + p
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// stubService satisfies the contract without implementing it.
type stubService struct{ core.Service }

// stubResolver serves no host.
type stubResolver struct{}

func (stubResolver) ResolveDomain(context.Context, string) (*core.Tenant, error) {
	return nil, core.NotFound("no tenant serves this host")
}

// TestTheDocumentHoldsExactlyTheRoutesTheServerRegisters walks the router's
// own registrations, not the registry, so a handler bound to the mux with no
// registry entry or endpoint is caught as surely as an entry with no handler.
func TestTheDocumentHoldsExactlyTheRoutesTheServerRegisters(t *testing.T) {
	t.Parallel()
	served := muxRoutes(t)
	documented := operations(document(t))
	if len(served) < 100 {
		t.Fatalf("the router registered %d routes, which is not the whole API", len(served))
	}
	for _, r := range served {
		if !slices.Contains(documented, r) {
			t.Errorf("%s is served and missing from the openapi document; add a registry entry or an endpoint", r)
		}
	}
	for _, r := range documented {
		if !slices.Contains(served, r) {
			t.Errorf("%s is in the openapi document and the server registers no such route", r)
		}
	}
}

// TestPublicOperationsAreTheOnesTheServerLetsThrough holds the document's
// unauthenticated operations to the middleware that decides it.
func TestPublicOperationsAreTheOnesTheServerLetsThrough(t *testing.T) {
	t.Parallel()
	fill := regexp.MustCompile(`\{[^}]+\}`)
	public := 0
	for path, item := range document(t)["paths"].(map[string]any) {
		for method, op := range item.(map[string]any) {
			sec, declared := op.(map[string]any)["security"].([]any)
			documented := declared && len(sec) == 0
			served := httpapi.IsPublicPath(fill.ReplaceAllString(path, "x.ics"))
			if documented != served {
				t.Errorf("%s %s: the document says public=%v and the server lets it through=%v",
					strings.ToUpper(method), path, documented, served)
			}
			if documented {
				public++
			}
		}
	}
	if public == 0 {
		t.Fatal("no operation is public, so the comparison saw nothing")
	}
}

// TestEveryRouteTheHTTPSpecNamesIsDocumented reads the routes the http-api
// OpenSpec capability mentions, relative ones included, and fails when the
// OpenAPI document has no operation that serves one.
func TestEveryRouteTheHTTPSpecNamesIsDocumented(t *testing.T) {
	t.Parallel()
	md, err := docsmd.Read("openspec/specs/" + openapi.HTTPSpec + "/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	mentioned := openapi.Mentions(md, true)
	if len(mentioned) < 20 {
		t.Fatalf("read %d routes from the http-api spec, which is not all of them", len(mentioned))
	}
	m, err := openapi.NewMatcher(operations(document(t)))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range mentioned {
		method, path, _ := strings.Cut(r, " ")
		if m.Match(method, path) == "" {
			t.Errorf("the http-api spec names %s and the openapi document has no operation for it", r)
		}
	}
}

// TestAListingIsDescribedAsItIsWritten holds the listing envelope the
// generator describes to the one wire.Page marshals.
func TestAListingIsDescribedAsItIsWritten(t *testing.T) {
	t.Parallel()
	schema := document(t)["paths"].(map[string]any)[wire.RouteTasks].(map[string]any)["get"].(map[string]any)
	content := schema["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)
	props := content[wire.ContentJSON].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)
	raw, _ := json.Marshal(wire.Page[core.Task]{Items: []core.Task{}, NextCursor: "c"})
	var page map[string]any
	_ = json.Unmarshal(raw, &page)
	for k := range page {
		if _, ok := props[k]; !ok {
			t.Errorf("a page marshals %q and the document does not describe it", k)
		}
	}
	for k := range props {
		if _, ok := page[k]; !ok {
			t.Errorf("the document describes %q and a page does not marshal it", k)
		}
	}
	if _, ok := content[wire.ContentNDJSON]; !ok {
		t.Error("a listing is not offered as ndjson")
	}
}

// fallback matches the sentence describe writes for a folded operation whose
// method has no doc comment: the operation's name, repeated.
var fallback = regexp.MustCompile("this route is `([^`]+)`: ([^ ]+)\\.$")

// TestEveryOperationIsDescribed fails when an operation falls back to its own
// name because the core.Service method behind it has no doc comment. The
// summary and description are all a reader of the document, and all a model
// choosing among the MCP tools built from it, has to go on.
func TestEveryOperationIsDescribed(t *testing.T) {
	t.Parallel()
	for path, item := range document(t)["paths"].(map[string]any) {
		for method, raw := range item.(map[string]any) {
			op, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id, _ := op["operationId"].(string)
			summary, _ := op["summary"].(string)
			if summary == "" || summary == id {
				t.Errorf("%s %s (%s) has no summary; give its core.Service method a doc comment",
					strings.ToUpper(method), path, id)
			}
			desc, _ := op["description"].(string)
			for _, para := range strings.Split(desc, "\n\n") {
				if m := fallback.FindStringSubmatch(para); m != nil && m[1] == m[2] {
					t.Errorf("%s %s?%s has no description; give its core.Service method a doc comment",
						strings.ToUpper(method), path, m[1])
				}
			}
		}
	}
}

// TestEveryParameterIsDescribed fails on a path or query parameter, or a
// parameter family read by prefix, that the document leaves undescribed. The
// MCP tools take their arguments from these parameters, so a bare name is all
// a model would have to go on.
func TestEveryParameterIsDescribed(t *testing.T) {
	t.Parallel()
	seen := 0
	for path, item := range document(t)["paths"].(map[string]any) {
		for method, raw := range item.(map[string]any) {
			op, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			params, _ := op["parameters"].([]any)
			for _, rp := range params {
				p := rp.(map[string]any)
				seen++
				if d, _ := p["description"].(string); strings.TrimSpace(d) == "" {
					where := "give its Query struct field a doc comment"
					if p["in"] == "path" {
						where = "add it to pathParamDocs in internal/openapi/params.go"
					}
					t.Errorf("%s %s: %s parameter %q has no description; %s",
						strings.ToUpper(method), path, p["in"], p["name"], where)
				}
			}
			prefixes, _ := op["x-tix-query-prefixes"].([]any)
			docs, _ := op["x-tix-query-prefix-descriptions"].(map[string]any)
			for _, x := range prefixes {
				if d, _ := docs[x.(string)].(string); strings.TrimSpace(d) == "" {
					t.Errorf("%s %s: query parameters starting %q have no description; "+
						"give the constant spelling the prefix a doc comment", strings.ToUpper(method), path, x)
				}
			}
		}
	}
	if seen < 150 {
		t.Fatalf("read %d parameters, which is not all of them", seen)
	}
}
