// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/heliopsy/tix/api"
	"github.com/heliopsy/tix/internal/openapi"
	"github.com/heliopsy/tix/internal/wire"
)

// contract is the committed OpenAPI document, compiled for validation. Every
// request the fixture server answers is checked against it, so a handler and
// the document cannot disagree without one of the API tests failing.
type contract struct {
	doc     map[string]any
	matcher *openapi.Matcher
	// lax checks requests against the document as published. strict checks
	// responses against a copy whose objects admit no property they do not
	// describe, since a handler writing a type other than the one declared
	// otherwise passes wherever the two types share no field by the same name.
	lax, strict *jsonschema.Compiler
	mu          sync.Mutex
	cache       map[string]*jsonschema.Schema
}

const contractURL = "file:///openapi.json"

// strayQuery are parameters a test sends on purpose to prove a handler
// ignores them, which the document rightly does not declare.
var strayQuery = map[string]string{
	"tenant": "TestBundleImportTargetsTheCallersOwnTenant: an import lands in the caller's tenant whatever the URL says",
}

// loadContract reads the embedded document once per process.
var loadContract = sync.OnceValues(func() (*contract, error) {
	raw, err := api.JSON()
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	lax, err := compiler(raw, false)
	if err != nil {
		return nil, err
	}
	strict, err := compiler(raw, true)
	if err != nil {
		return nil, err
	}
	var patterns []string
	for path, item := range doc["paths"].(map[string]any) {
		for method := range item.(map[string]any) {
			patterns = append(patterns, strings.ToUpper(method)+" "+path)
		}
	}
	m, err := openapi.NewMatcher(patterns)
	if err != nil {
		return nil, err
	}
	return &contract{doc: doc, matcher: m, lax: lax, strict: strict, cache: map[string]*jsonschema.Schema{}}, nil
})

// compiler loads the document, closing every described object when strict.
func compiler(raw []byte, strict bool) (*jsonschema.Compiler, error) {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if strict {
		closeObjects(inst)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(contractURL, inst); err != nil {
		return nil, err
	}
	return c, nil
}

// closeObjects forbids undescribed properties on every schema that lists
// properties and says nothing about others.
func closeObjects(v any) {
	switch v := v.(type) {
	case map[string]any:
		if _, ok := v["properties"].(map[string]any); ok {
			if _, set := v["additionalProperties"]; !set {
				v["additionalProperties"] = false
			}
		}
		for _, child := range v {
			closeObjects(child)
		}
	case []any:
		for _, child := range v {
			closeObjects(child)
		}
	}
}

// checkContract wraps the fixture's handler. The response is held until it
// has been checked, so a failure is reported while the test is still running.
// A WebSocket upgrade passes straight through: the stream is not JSON over
// HTTP and the document only names its 101.
func checkContract(t *testing.T, next http.Handler) http.Handler {
	t.Helper()
	ct, err := loadContract()
	if err != nil {
		t.Fatalf("loading the openapi document: %v", err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			next.ServeHTTP(w, r)
			return
		}
		reqBody, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		for _, problem := range ct.check(r, reqBody, rec) {
			t.Errorf("openapi contract: %s %s: %s", r.Method, r.URL.Path, problem)
		}
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
}

// check lists every way one exchange departs from the document.
func (ct *contract) check(r *http.Request, reqBody []byte, rec *httptest.ResponseRecorder) []string {
	method := r.Method
	if method == http.MethodHead {
		method = http.MethodGet
	}
	if !strings.HasPrefix(r.URL.Path, wire.APIPrefix+"/") && r.URL.Path != wire.RouteHealth &&
		r.URL.Path != wire.RouteReady {
		return nil
	}
	pattern := ct.matcher.Match(method, r.URL.Path)
	if pattern == "" {
		if rec.Code < 300 {
			return []string{fmt.Sprintf("answered %d and the document has no operation for it", rec.Code)}
		}
		return nil
	}
	_, path, _ := strings.Cut(pattern, " ")
	op := ct.doc["paths"].(map[string]any)[path].(map[string]any)[strings.ToLower(method)].(map[string]any)
	ptr := "#/paths/" + escape(path) + "/" + strings.ToLower(method)
	name := fmt.Sprint(op["operationId"])

	var problems []string
	problems = append(problems, ct.checkQuery(op, r)...)
	if rec.Code < 300 {
		problems = append(problems, ct.checkRequest(op, ptr, r, reqBody)...)
	}
	if r.Method != http.MethodHead {
		problems = append(problems, ct.checkResponse(op, ptr, rec)...)
	}
	for i, p := range problems {
		problems[i] = name + ": " + p
	}
	return problems
}

// checkQuery fails a query parameter the operation does not declare.
func (ct *contract) checkQuery(op map[string]any, r *http.Request) []string {
	declared := map[string]bool{}
	params, _ := op["parameters"].([]any)
	for _, p := range params {
		p := p.(map[string]any)
		if p["in"] == "query" {
			declared[p["name"].(string)] = true
		}
	}
	prefixes, _ := op["x-tix-query-prefixes"].([]any)
	var problems []string
	for name := range r.URL.Query() {
		if _, stray := strayQuery[name]; stray {
			continue
		}
		if declared[name] || slices.ContainsFunc(prefixes, func(p any) bool {
			return strings.HasPrefix(name, p.(string))
		}) {
			continue
		}
		problems = append(problems, fmt.Sprintf("query parameter %q is not declared; set Query on the registry entry", name))
	}
	slices.Sort(problems)
	return problems
}

// checkRequest validates an accepted request body against the document.
func (ct *contract) checkRequest(op map[string]any, ptr string, r *http.Request, body []byte) []string {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	rb, ok := op["requestBody"].(map[string]any)
	if !ok {
		return []string{"the request carried a body the document does not declare; set Body on the registry entry"}
	}
	media := mediaType(r.Header.Get(wire.HeaderContentType))
	content := rb["content"].(map[string]any)
	if media != "" && media != wire.ContentJSON {
		if _, ok := content[media]; !ok {
			return []string{fmt.Sprintf("request body is %s, which the document does not declare", media)}
		}
		return nil
	}
	if _, ok := content[wire.ContentJSON]; !ok {
		return []string{"the request carried JSON and the document declares no JSON body"}
	}
	return ct.validate(ct.lax, ptr+"/requestBody/content/"+escape(wire.ContentJSON)+"/schema", body, "request body")
}

// checkResponse validates the status, the media type and the body.
func (ct *contract) checkResponse(op map[string]any, ptr string, rec *httptest.ResponseRecorder) []string {
	code := strconv.Itoa(rec.Code)
	responses := op["responses"].(map[string]any)
	resp, ok := responses[code].(map[string]any)
	if !ok {
		return []string{fmt.Sprintf("answered %d, which the document does not list", rec.Code)}
	}
	ptr += "/responses/" + code
	if ref, ok := resp["$ref"].(string); ok {
		ptr = ref
		resp = ct.doc["components"].(map[string]any)["responses"].(map[string]any)[strings.TrimPrefix(ref, "#/components/responses/")].(map[string]any)
	}
	content, _ := resp["content"].(map[string]any)
	body := rec.Body.Bytes()
	if len(content) == 0 {
		if len(bytes.TrimSpace(body)) > 0 {
			return []string{fmt.Sprintf("answered %d with a body the document does not declare", rec.Code)}
		}
		return nil
	}
	media := mediaType(rec.Header().Get(wire.HeaderContentType))
	key := media
	if _, ok := content[key]; !ok {
		key = ""
		for k := range content {
			if wild, ok := strings.CutSuffix(k, "/*"); ok && strings.HasPrefix(media, wild+"/") {
				key = k
			}
		}
	}
	if key == "" {
		return []string{fmt.Sprintf("answered %d as %q, which the document does not list", rec.Code, media)}
	}
	schemaPtr := ptr + "/content/" + escape(key) + "/schema"
	switch key {
	case wire.ContentJSON:
		return ct.validate(ct.strict, schemaPtr, body, "response "+code)
	case wire.ContentNDJSON:
		if _, ok := content[key].(map[string]any)["schema"]; !ok {
			return nil
		}
		var problems []string
		sc := bufio.NewScanner(bytes.NewReader(body))
		sc.Buffer(nil, 1<<24)
		for line := 1; sc.Scan(); line++ {
			problems = append(problems, ct.validate(ct.strict, schemaPtr, sc.Bytes(), fmt.Sprintf("response %s line %d", code, line))...)
		}
		return problems
	}
	return nil
}

// validate checks one JSON document against the schema at a pointer.
func (ct *contract) validate(c *jsonschema.Compiler, ptr string, body []byte, what string) []string {
	sch, err := ct.schema(c, ptr)
	if err != nil {
		return []string{fmt.Sprintf("compiling %s: %v", ptr, err)}
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return []string{fmt.Sprintf("%s is not JSON: %v", what, err)}
	}
	if err := sch.Validate(inst); err != nil {
		return []string{fmt.Sprintf("%s does not match the document: %#v", what, err)}
	}
	return nil
}

// schema compiles the schema at a pointer once.
func (ct *contract) schema(c *jsonschema.Compiler, ptr string) (*jsonschema.Schema, error) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	key := fmt.Sprintf("%p%s", c, ptr)
	if s, ok := ct.cache[key]; ok {
		return s, nil
	}
	s, err := c.Compile(contractURL + ptr)
	if err != nil {
		return nil, err
	}
	ct.cache[key] = s
	return s, nil
}

// escape renders a key as a JSON pointer token.
func escape(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}

// mediaType drops a Content-Type's parameters.
func mediaType(ct string) string {
	m, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return ""
	}
	return m
}
