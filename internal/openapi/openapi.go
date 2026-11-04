// SPDX-License-Identifier: AGPL-3.0-or-later

// Package openapi generates the OpenAPI 3.1 document of the HTTP API from the
// capability registry, so the document cannot describe a route the server
// does not register or miss one it does. Schemas are reflected from the Go
// types the handlers read and write, and descriptions come from their doc
// comments.
package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/swaggest/jsonschema-go"
	oapi "github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// Security scheme names.
const (
	schemeBearer  = "bearer"
	schemeSession = "session"
)

// closedTypes are the core types whose values are exactly their declared
// constants, so their schema carries those constants as an enum. A type whose
// set is open, such as a workflow status or a colour, stays out of this list.
var closedTypes = []any{
	core.Kind(""), core.Scope(""), core.ActorKind(""), core.Role(""),
	core.StateCategory(""), core.SortDirection(""), core.ArtifactKind(""),
	core.InboxKind(""), core.DeliveryStatus(""), core.BulkAction(""),
	core.FieldType(""), core.LeaseSource(""), core.RecurrenceUnit(""),
	core.ProjectIconKind(""), core.ChannelFormat(""), core.HousekeepingRule(""),
	core.CollisionPolicy(""), core.ImportMode(""), core.MatchMode(""),
	core.TOTPChallengeKind(""), core.ComponentKind(""), core.ComponentAction(""),
	core.ConnectionSurface(""), core.EventType(""), core.Source(""), core.Surface(""),
}

// entry is one operation of the document before it is reflected.
type entry struct {
	name     string
	route    capability.Route
	method   string
	cli      string
	summary  string
	scope    core.Scope
	contract capability.Contract
	// also are further registry operations sharing this method and path,
	// picked by a query discriminator.
	also []entry
}

// Build returns the OpenAPI document as a generic JSON value with every map
// sorted, reading doc comments and constants from the module under root.
func Build(root string) (map[string]any, error) {
	src, err := readSource(root)
	if err != nil {
		return nil, err
	}
	specs, err := readOpenSpec(root)
	if err != nil {
		return nil, err
	}
	g := &generator{src: src, specs: specs, r: openapi31.NewReflector()}
	g.configure()
	entries, err := collect()
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if err := g.add(e); err != nil {
			return nil, fmt.Errorf("%s %s (%s): %w", e.route.Method, e.route.Pattern, e.name, err)
		}
	}
	if len(g.clash) > 0 {
		return nil, fmt.Errorf("schema names collide: %s", strings.Join(g.clash, "; "))
	}
	raw, err := g.r.Spec.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	g.finish(doc, entries)
	return doc, nil
}

// collect gathers every HTTP binding of the registry and every endpoint,
// folding operations that share one method and path into the one a request
// without the discriminating query reaches.
func collect() ([]entry, error) {
	var out []entry
	for _, op := range capability.Operations() {
		if op.HTTP.Zero() {
			continue
		}
		out = append(out, entry{
			name: op.Name, route: op.HTTP, method: op.Method, cli: op.CLI,
			scope: op.Scope, contract: op.Contract,
		})
	}
	for _, ep := range capability.Endpoints() {
		out = append(out, entry{
			name: ep.Name, route: ep.HTTP, method: ep.Method, summary: ep.Summary,
			scope: ep.Scope(), contract: ep.Contract,
		})
	}
	byKey := map[string]int{}
	var folded []entry
	sort.SliceStable(out, func(i, j int) bool { return out[i].route.Query < out[j].route.Query })
	for _, e := range out {
		key := e.route.Method + " " + e.route.Pattern
		if i, ok := byKey[key]; ok {
			if e.route.Query == "" {
				return nil, fmt.Errorf("%s and %s both claim %s", folded[i].name, e.name, key)
			}
			folded[i].also = append(folded[i].also, e)
			continue
		}
		byKey[key] = len(folded)
		folded = append(folded, e)
	}
	sort.Slice(folded, func(i, j int) bool { return folded[i].name < folded[j].name })
	return folded, nil
}

// generator reflects entries into one document.
type generator struct {
	src   *source
	specs map[string][]string
	r     *openapi31.Reflector
	names map[string]reflect.Type
	clash []string
}

// configure sets the document's header and the reflection rules.
func (g *generator) configure() {
	g.r.Spec.Info.
		WithTitle("tix").
		WithVersion("1").
		WithDescription("The tix HTTP API. Generated from the capability registry by `just openapi`; " +
			"do not edit by hand.")
	g.r.Spec.Info.WithLicense(openapi31.License{Name: "AGPL-3.0-or-later", Identifier: ptr("AGPL-3.0-or-later")})
	js := g.r.JSONSchemaReflector()
	js.AddTypeMapping(core.Duration(0), "")
	js.DefaultOptions = append(js.DefaultOptions,
		jsonschema.InterceptSchema(g.intercept),
		jsonschema.InterceptDefName(g.defName))
}

// defName names a schema after its type alone, dropping the package prefix
// the reflector adds, and records a name two types would share.
func (g *generator) defName(t reflect.Type, name string) string {
	for _, prefix := range []string{"Core", "Httpapi", "Wire"} {
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			name = rest
			break
		}
	}
	if g.names == nil {
		g.names = map[string]reflect.Type{}
	}
	if prev, ok := g.names[name]; ok && prev != t {
		g.clash = append(g.clash, fmt.Sprintf("%s and %s are both named %s", prev, t, name))
	}
	g.names[name] = t
	return name
}

// intercept adds what reflection cannot see: doc comments and closed enums.
func (g *generator) intercept(p jsonschema.InterceptSchemaParams) (bool, error) {
	if !p.Processed {
		return false, nil
	}
	t := p.Value.Type()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	key := typeKey(t)
	if key == "" {
		return false, nil
	}
	if p.Schema.Description == nil {
		if d := g.src.types[key]; d != "" {
			p.Schema.WithDescription(d)
		}
	}
	if t.Kind() == reflect.String && isClosed(t) {
		vals := g.src.consts[key]
		enum := make([]any, 0, len(vals))
		for _, v := range vals {
			enum = append(enum, v)
		}
		p.Schema.Enum = enum
	}
	if t.Kind() == reflect.Struct {
		g.describeFields(t, p.Schema)
	}
	return false, nil
}

// describeFields sets each property's description from its field comment.
func (g *generator) describeFields(t reflect.Type, s *jsonschema.Schema) {
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.Anonymous && name == "" {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				g.describeFields(ft, s)
			}
			continue
		}
		if name == "" || name == "-" {
			continue
		}
		prop, ok := s.Properties[name]
		if !ok || prop.TypeObject == nil || prop.TypeObject.Description != nil {
			continue
		}
		if d := g.src.fields[typeKey(t)][f.Name]; d != "" {
			prop.TypeObject.WithDescription(d)
			s.Properties[name] = prop
		}
	}
}

// typeKey names a type as the source index does, "pkg.Type".
func typeKey(t reflect.Type) string {
	if t.Name() == "" || t.PkgPath() == "" {
		return ""
	}
	pkg := t.PkgPath()[strings.LastIndex(t.PkgPath(), "/")+1:]
	name, _, _ := strings.Cut(t.Name(), "[")
	return pkg + "." + name
}

// isClosed reports whether a type's values are exactly its constants.
func isClosed(t reflect.Type) bool {
	return slices.ContainsFunc(closedTypes, func(v any) bool { return reflect.TypeOf(v) == t })
}

// serviceMethod returns the core.Service method an entry reaches.
func serviceMethod(name string) (reflect.Method, bool) {
	if name == "" {
		return reflect.Method{}, false
	}
	return reflect.TypeOf((*core.Service)(nil)).Elem().MethodByName(name)
}

var (
	typeTaskRef = reflect.TypeOf(core.TaskRef{})
	typePage    = reflect.TypeOf(core.Page{})
	typeContext = reflect.TypeOf((*context.Context)(nil)).Elem()
	typeError   = reflect.TypeOf((*error)(nil)).Elem()
	typeNone    = reflect.TypeOf(capability.None{})
)

// body is the request body an entry reads: the contract's, or the method's
// last struct parameter on a method that carries one.
func body(e entry, m reflect.Method, ok bool) (reflect.Type, error) {
	if e.contract.Body != nil {
		t := reflect.TypeOf(e.contract.Body)
		if t == typeNone {
			return nil, nil
		}
		return t, nil
	}
	in := structParam(m, ok)
	switch e.route.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return in, nil
	}
	if in != nil && in != typePage && e.contract.Query == nil {
		return nil, fmt.Errorf("%s takes %s, which a %s carries in no body: declare Query or Body",
			e.method, in, e.route.Method)
	}
	return nil, nil
}

// structParam is the method's last struct parameter that is not a path
// reference or the context.
func structParam(m reflect.Method, ok bool) reflect.Type {
	if !ok {
		return nil
	}
	for i := m.Type.NumIn() - 1; i >= 0; i-- {
		t := m.Type.In(i)
		if t == typeContext || t == typeTaskRef {
			continue
		}
		if t.Kind() == reflect.Struct {
			return t
		}
	}
	return nil
}

// query is the struct of query parameters an entry reads.
func query(e entry, m reflect.Method, ok bool) any {
	if e.contract.Query != nil {
		return e.contract.Query
	}
	if e.route.Method == http.MethodGet && structParam(m, ok) == typePage {
		return httpapi.PageQuery{}
	}
	return nil
}

// response is the success body an entry answers with and its status. A nil
// type means no body.
func response(e entry, m reflect.Method, ok bool) (reflect.Type, int, error) {
	status := e.contract.Status
	if e.contract.Resp != nil {
		t := reflect.TypeOf(e.contract.Resp)
		if t == typeNone {
			t = nil
		} else {
			t = asPage(t)
		}
		if status == 0 {
			status = http.StatusOK
			if t == nil {
				status = http.StatusNoContent
			}
		}
		return t, status, nil
	}
	if !ok {
		return nil, 0, fmt.Errorf("an endpoint reaching no service method declares no Resp")
	}
	if m.Type.NumOut() == 0 || m.Type.Out(0) == typeError {
		if status == 0 {
			status = http.StatusNoContent
		}
		return nil, status, nil
	}
	t := m.Type.Out(0)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if status == 0 {
		status = http.StatusOK
	}
	switch t.Kind() {
	case reflect.Slice:
		return pageOf(t.Elem()), status, nil
	case reflect.Struct, reflect.Map:
		return t, status, nil
	default:
		return nil, 0, fmt.Errorf("%s answers %s, which is not a JSON object: declare Resp", e.method, t)
	}
}

// add reflects one entry into the document.
func (g *generator) add(e entry) error {
	m, ok := serviceMethod(e.method)
	if e.method != "" && !ok {
		return fmt.Errorf("names service method %q, which does not exist", e.method)
	}
	oc, err := g.r.NewOperationContext(e.route.Method, e.route.Pattern)
	if err != nil {
		return err
	}
	oc.SetID(e.name)
	oc.SetTags(tagOf(e.name))
	if params := pathParams(e.route.Pattern); params != nil {
		oc.AddReqStructure(params)
	}
	if q := query(e, m, ok); q != nil {
		oc.AddReqStructure(q)
	}
	for _, a := range e.also {
		am, aok := serviceMethod(a.method)
		if q := query(a, am, aok); q != nil && reflect.TypeOf(q) != reflect.TypeOf(query(e, m, ok)) {
			oc.AddReqStructure(q)
		}
	}
	bt, err := body(e, m, ok)
	if err != nil {
		return err
	}
	switch {
	case e.contract.BodyContent != "":
		oc.AddReqStructure(nil, oapi.WithContentType(e.contract.BodyContent))
	case bt != nil:
		oc.AddReqStructure(sample(bt), oapi.WithContentType(wire.ContentJSON))
	}
	if err := g.addResponses(oc, e, m, ok); err != nil {
		return err
	}
	for _, a := range e.also {
		am, aok := serviceMethod(a.method)
		at, status, err := response(a, am, aok)
		if err != nil {
			return err
		}
		desc := fmt.Sprintf("%s, when %s", a.name, a.route.Query)
		if at == nil {
			oc.AddRespStructure(nil, oapi.WithHTTPStatus(status), descOpt(desc))
		} else {
			oc.AddRespStructure(reflect.New(at).Elem().Interface(), oapi.WithHTTPStatus(status), descOpt(desc))
		}
	}
	if !e.contract.Public {
		scopes := []string{}
		if e.scope != "" {
			scopes = []string{string(e.scope)}
		}
		oc.AddSecurity(schemeBearer, scopes...)
		oc.AddSecurity(schemeSession, scopes...)
	}
	return g.r.AddOperation(oc)
}

// addResponses adds the success response, the further ones and the errors.
func (g *generator) addResponses(oc oapi.OperationContext, e entry, m reflect.Method, ok bool) error {
	rt, status, err := response(e, m, ok)
	if err != nil {
		return err
	}
	switch {
	case e.contract.Content != "":
		oc.AddRespStructure(nil, oapi.WithHTTPStatus(status), oapi.WithContentType(e.contract.Content))
	case rt != nil:
		oc.AddRespStructure(reflect.New(rt).Elem().Interface(), oapi.WithHTTPStatus(status))
	default:
		oc.AddRespStructure(nil, oapi.WithHTTPStatus(status))
	}
	for _, code := range sortedKeys(e.contract.Also) {
		v := e.contract.Also[code]
		if reflect.TypeOf(v) == typeNone {
			oc.AddRespStructure(nil, oapi.WithHTTPStatus(code))
			continue
		}
		oc.AddRespStructure(v, oapi.WithHTTPStatus(code))
	}
	for _, code := range errorStatuses(e) {
		oc.AddRespStructure(wire.ErrorEnvelope{}, oapi.WithHTTPStatus(code), descOpt(errorDescription(code)))
	}
	return nil
}

// errorStatuses are the statuses an entry may fail with: every status the
// error taxonomy maps to, and the two a JSON body adds when it is refused
// before it is decoded.
func errorStatuses(e entry) []int {
	set := map[int]bool{}
	for _, k := range core.Kinds {
		set[k.HTTPStatus()] = true
	}
	if e.route.Pattern != wire.RouteEvents {
		set[http.StatusGatewayTimeout] = true
	}
	if e.route.Method != http.MethodGet && e.route.Method != http.MethodDelete {
		set[http.StatusRequestEntityTooLarge] = true
		set[http.StatusUnsupportedMediaType] = true
	}
	return sortedKeys(set)
}

// errorDescription names the error kinds answered with a status.
func errorDescription(code int) string {
	var kinds []string
	for _, k := range core.Kinds {
		if k.HTTPStatus() == code {
			kinds = append(kinds, "`"+string(k)+"`")
		}
	}
	switch {
	case code == http.StatusGatewayTimeout:
		return http.StatusText(code) + ": `internal`, the request ran past the server's timeout."
	case len(kinds) == 0:
		return http.StatusText(code) + ": `invalid`, the body was refused before it was read."
	}
	return http.StatusText(code) + ": " + strings.Join(kinds, ", ") + "."
}

// descOpt sets a response description.
func descOpt(d string) oapi.ContentOption {
	return func(cu *oapi.ContentUnit) { cu.Description = d }
}

// tagOf is the area an operation belongs to, the first segment of its name.
func tagOf(name string) string {
	tag, _, _ := strings.Cut(name, ".")
	return tag
}

var wildcard = regexp.MustCompile(`\{([^}]+)\}`)

// pathParams builds a struct holding one path parameter per wildcard.
func pathParams(pattern string) any {
	names := wildcard.FindAllStringSubmatch(pattern, -1)
	if len(names) == 0 {
		return nil
	}
	fields := make([]reflect.StructField, 0, len(names))
	for i, n := range names {
		fields = append(fields, reflect.StructField{
			Name: "P" + strconv.Itoa(i),
			Type: reflect.TypeOf(""),
			Tag:  reflect.StructTag(`path:"` + n[1] + `" required:"true"`),
		})
	}
	return reflect.New(reflect.StructOf(fields)).Elem().Interface()
}

var typeWirePage = reflect.TypeOf(wire.Page[any]{})

// pageOf is the shape of a wire.Page of the element type. Generics cannot be
// instantiated by reflection, so the envelope is rebuilt field for field from
// wire.Page itself, and a test holds the two to one JSON shape.
func pageOf(elem reflect.Type) reflect.Type {
	fields := make([]reflect.StructField, 0, typeWirePage.NumField())
	for i := range typeWirePage.NumField() {
		f := typeWirePage.Field(i)
		if f.Type.Kind() == reflect.Slice {
			f.Type = reflect.SliceOf(elem)
		}
		fields = append(fields, reflect.StructField{Name: f.Name, Type: f.Type, Tag: f.Tag})
	}
	return reflect.StructOf(fields)
}

// asPage rewrites an instantiated wire.Page into the shape pageOf builds, so
// every listing is described alike whether its Resp was declared or inferred.
func asPage(t reflect.Type) reflect.Type {
	if t.PkgPath() != typeWirePage.PkgPath() || !strings.HasPrefix(t.Name(), "Page[") {
		return t
	}
	items, _ := t.FieldByName("Items")
	return pageOf(items.Type.Elem())
}

// sortedKeys returns a map's integer keys in order.
func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// sample returns a zero value of t with every pointer-to-pointer field set to
// a pointer to nil. swaggest/refl walks a nil **T into an invalid value and
// panics, and a field such as due_at, which distinguishes absent from null,
// is exactly that shape.
func sample(t reflect.Type) any {
	v := reflect.New(t).Elem()
	fillDoublePointers(v)
	return v.Interface()
}

// fillDoublePointers sets the nil pointer-to-pointer fields of a struct.
func fillDoublePointers(v reflect.Value) {
	if v.Kind() != reflect.Struct {
		return
	}
	for i := range v.NumField() {
		f := v.Field(i)
		if !f.CanSet() {
			continue
		}
		switch {
		case f.Kind() == reflect.Pointer && f.Type().Elem().Kind() == reflect.Pointer && f.IsNil():
			f.Set(reflect.New(f.Type().Elem()))
		case f.Kind() == reflect.Struct:
			fillDoublePointers(f)
		}
	}
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }
