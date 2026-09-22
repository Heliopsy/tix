package httpapi_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// minimalBundle is a bundle body carrying nothing but its header, so a
// validation test need not depend on an export first.
const minimalBundle = `{"record":"header","header":{"name":"starter","version":1}}` + "\n"

// exportBundle asks the API for a bundle and returns its body.
func (f *apiFixture) exportBundle(in core.BundleExportInput) []byte {
	f.t.Helper()
	resp := f.call(http.MethodPost, wire.RouteBundleExport, in)
	mustStatus(f.t, resp, http.StatusOK)
	return []byte(readBody(f.t, resp))
}

// importBundle posts a bundle body with the given query and returns the response.
func (f *apiFixture) importBundle(query string, body []byte, host, token string) *http.Response {
	f.t.Helper()
	req, err := http.NewRequest(http.MethodPost,
		f.server.URL+wire.RouteBundleImport+query, bytes.NewReader(body))
	if err != nil {
		f.t.Fatalf("building request: %v", err)
	}
	req.Host = host
	req.Header.Set(wire.HeaderAuth, "Bearer "+token)
	req.Header.Set(wire.HeaderContentType, wire.ContentBundle)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatalf("sending import: %v", err)
	}
	return resp
}

func TestBundleExportReturnsABundleBody(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPost, wire.RouteBundleExport,
		core.BundleExportInput{Name: "starter", Kinds: []core.ComponentKind{core.ComponentWorkflow}})
	mustStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get(wire.HeaderContentType); got != wire.ContentBundle {
		t.Errorf("content type = %q, want %q", got, wire.ContentBundle)
	}
	if got := resp.Header.Get(wire.HeaderContentDisposition); !strings.Contains(got, "filename=") {
		t.Errorf("content disposition = %q, want a filename", got)
	}
	if body := readBody(t, resp); strings.TrimSpace(body) == "" {
		t.Error("export returned an empty bundle")
	}
}

func TestBundleExportRefusesAnUnknownKind(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPost, wire.RouteBundleExport,
		core.BundleExportInput{Kinds: []core.ComponentKind{"saved_filter"}})
	mustStatus(t, resp, http.StatusBadRequest)
	var env wire.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Code != core.KindInvalid {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindInvalid)
	}
}

func TestBundleImportAppliesABundle(t *testing.T) {
	f := newFixture(t)
	bundle := f.exportBundle(core.BundleExportInput{Name: "starter",
		Kinds: []core.ComponentKind{core.ComponentWorkflow}})

	resp := f.importBundle("?on_collision=rename", bundle, f.hostA, f.tokenA)
	mustStatus(t, resp, http.StatusOK)
	var result core.BundleResult
	decodeBody(t, resp, &result)
	if result.Preview {
		t.Error("a real import reported itself as a preview")
	}
	if len(result.Outcomes) == 0 {
		t.Error("import reported no outcomes")
	}
}

func TestBundleImportRequiresACollisionPolicy(t *testing.T) {
	f := newFixture(t)

	resp := f.importBundle("", []byte(minimalBundle), f.hostA, f.tokenA)
	mustStatus(t, resp, http.StatusBadRequest)
	var env wire.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Code != core.KindInvalid {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindInvalid)
	}
	if !strings.Contains(env.Error.Message, "collision policy") {
		t.Errorf("message = %q, want it to name the collision policy", env.Error.Message)
	}
}

func TestBundleImportRefusesAnUnknownCollisionPolicy(t *testing.T) {
	f := newFixture(t)

	resp := f.importBundle("?on_collision=clobber", []byte(minimalBundle), f.hostA, f.tokenA)
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()
}

func TestBundleImportPreviewWritesNothing(t *testing.T) {
	f := newFixture(t)
	bundle := f.exportBundle(core.BundleExportInput{Name: "starter",
		Kinds: []core.ComponentKind{core.ComponentWorkflow}})
	before := f.workflowCount()

	resp := f.importBundle("?on_collision=rename&preview=true", bundle, f.hostA, f.tokenA)
	mustStatus(t, resp, http.StatusOK)
	var result core.BundleResult
	decodeBody(t, resp, &result)
	if !result.Preview {
		t.Error("preview was not carried through")
	}
	if after := f.workflowCount(); after != before {
		t.Errorf("workflow count = %d, want %d unchanged by a preview", after, before)
	}
}

func TestBundleImportTargetsTheCallersOwnTenant(t *testing.T) {
	f := newFixture(t)
	bundle := f.exportBundle(core.BundleExportInput{Name: "starter",
		Kinds: []core.ComponentKind{core.ComponentWorkflow}})
	before := f.workflowCount()

	resp := f.importBundle("?on_collision=rename&tenant="+f.tenantA.Key, bundle, f.hostB, f.tokenB)
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	if after := f.workflowCount(); after != before {
		t.Errorf("tenant a workflow count = %d, want %d; another tenant's import reached it", after, before)
	}
}

// workflowCount reports how many workflows tenant A can see.
func (f *apiFixture) workflowCount() int {
	f.t.Helper()
	resp := f.call(http.MethodGet, wire.RouteWorkflows, nil)
	mustStatus(f.t, resp, http.StatusOK)
	var page wire.Page[core.Workflow]
	decodeBody(f.t, resp, &page)
	return len(page.Items)
}

func TestBundleExportRefusesAMalformedBody(t *testing.T) {
	f := newFixture(t)

	req := f.newRequest(http.MethodPost, wire.RouteBundleExport, strings.NewReader("{"))
	req.Header.Set(wire.HeaderContentType, wire.ContentJSON)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()
}

func TestUnnamedBundleDownloadsUnderADefaultName(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPost, wire.RouteBundleExport, core.BundleExportInput{})
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	if got := resp.Header.Get(wire.HeaderContentDisposition); got != `attachment; filename="tix-bundle.ndjson"` {
		t.Errorf("content disposition = %q, want the default bundle filename", got)
	}
}

func TestBundleNameIsReducedToASafeFilename(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPost, wire.RouteBundleExport,
		core.BundleExportInput{Name: `Ops "kit"; rm -rf /`})
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	got := resp.Header.Get(wire.HeaderContentDisposition)
	if strings.ContainsAny(got[len("attachment; filename="):], ";") {
		t.Errorf("content disposition = %q, want no punctuation from the bundle name", got)
	}
	if !strings.HasSuffix(got, `.ndjson"`) {
		t.Errorf("content disposition = %q, want a bundle filename", got)
	}
}

func TestBundleImportRefusesAMalformedBundle(t *testing.T) {
	f := newFixture(t)

	resp := f.importBundle("?on_collision=skip", []byte("this is not a bundle\n"), f.hostA, f.tokenA)
	if resp.StatusCode < 400 {
		t.Fatalf("status = %d, want a failure: %s", resp.StatusCode, readBody(t, resp))
	}
	var env wire.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Message == "" {
		t.Error("a malformed bundle carried no error envelope")
	}
}
