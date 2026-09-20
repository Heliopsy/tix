package bundle

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

var fixedTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func goodWorkflow(key string) Component {
	return Component{Kind: core.ComponentWorkflow, Workflow: &Workflow{
		Key:  key,
		Name: strings.ToUpper(key),
		Definition: core.WorkflowDefinition{
			Initial:     "todo",
			States:      []core.State{{Key: "todo"}, {Key: "done", Terminal: true}},
			Transitions: []core.Transition{{From: "todo", To: "done"}},
		},
	}}
}

func encodeAll(t *testing.T, name string, comps ...Component) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Header(name, fixedTime); err != nil {
		t.Fatalf("header: %v", err)
	}
	for _, c := range comps {
		if err := enc.Component(c); err != nil {
			t.Fatalf("component %s %q: %v", c.Kind, c.Key(), err)
		}
	}
	return buf.Bytes()
}

func TestComponentKeyAndPayload(t *testing.T) {
	cases := []struct {
		name    string
		comp    Component
		key     string
		payload bool
	}{
		{"workflow", goodWorkflow("dev"), "dev", true},
		{"field", Component{Kind: core.ComponentFieldDef, Field: &Field{Key: "points", Type: core.FieldInt}}, "points", true},
		{"tag", Component{Kind: core.ComponentTag, Tag: &Tag{Name: "backend"}}, "backend", true},
		{"project", Component{Kind: core.ComponentProject, Project: &Project{Key: "acme"}}, "acme", true},
		{"webhook", Component{Kind: core.ComponentWebhook, Webhook: &Webhook{URL: "https://x/y"}}, "https://x/y", true},
		{"missing payload", Component{Kind: core.ComponentWorkflow}, "", false},
		{"unknown kind", Component{Kind: "task"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.comp.Key(); got != tc.key {
				t.Errorf("Key() = %q, want %q", got, tc.key)
			}
			if got := tc.comp.HasPayload(); got != tc.payload {
				t.Errorf("HasPayload() = %v, want %v", got, tc.payload)
			}
		})
	}
}

func TestComponentValidate(t *testing.T) {
	undefined := goodWorkflow("dev")
	undefined.Workflow.Definition.Transitions = []core.Transition{{From: "todo", To: "nowhere"}}

	cases := []struct {
		name string
		comp Component
		want string
	}{
		{"valid workflow", goodWorkflow("dev"), ""},
		{"unknown kind", Component{Kind: "task", Tag: &Tag{Name: "x"}}, "not one this build can share"},
		{"no payload", Component{Kind: core.ComponentTag}, "carries no payload"},
		{"no key", Component{Kind: core.ComponentTag, Tag: &Tag{Name: "  "}}, "has no key"},
		{"transition to undefined state", undefined, `workflow "dev" is not valid`},
		{"bad field", Component{Kind: core.ComponentFieldDef, Field: &Field{Key: "k", Type: "nope"}}, `field definition "k" is not valid`},
		{"enum without options", Component{Kind: core.ComponentFieldDef, Field: &Field{Key: "k", Type: core.FieldEnum}}, "is not valid"},
		{"valid field", Component{Kind: core.ComponentFieldDef, Field: &Field{Key: "k", Type: core.FieldString}}, ""},
		{"valid tag", Component{Kind: core.ComponentTag, Tag: &Tag{Name: "x", Color: "#fff"}}, ""},
		{"bad project key", Component{Kind: core.ComponentProject, Project: &Project{Key: "9bad"}}, "is not valid"},
		{"project with bad workflow", Component{Kind: core.ComponentProject, Project: &Project{
			Key: "acme", Workflow: &Workflow{Key: "w"},
		}}, "carries an invalid workflow"},
		{"project with bad field", Component{Kind: core.ComponentProject, Project: &Project{
			Key: "acme", Fields: []Field{{Key: "k", Type: "nope"}},
		}}, "carries an invalid field"},
		{"project with nameless tag", Component{Kind: core.ComponentProject, Project: &Project{
			Key: "acme", Tags: []Tag{{Name: " "}},
		}}, "tag with no name"},
		{"valid project", Component{Kind: core.ComponentProject, Project: &Project{
			Key: "acme", Workflow: goodWorkflow("dev").Workflow,
			Fields: []Field{{Key: "k", Type: core.FieldString}}, Tags: []Tag{{Name: "x"}},
		}}, ""},
		{"webhook without url", Component{Kind: core.ComponentWebhook, Webhook: &Webhook{URL: " "}}, "has no key"},
		{"valid webhook", Component{Kind: core.ComponentWebhook, Webhook: &Webhook{URL: "https://x/y"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.comp.Validate()
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("Validate() = %v, want nil", err)
			case tc.want == "":
			case err == nil:
				t.Fatalf("Validate() = nil, want an error mentioning %q", tc.want)
			case !strings.Contains(err.Error(), tc.want):
				t.Fatalf("Validate() = %v, want it to mention %q", err, tc.want)
			case !core.IsKind(err, core.KindInvalid):
				t.Fatalf("Validate() = %v, want an invalid-input error", err)
			}
		})
	}
}

func TestWebhookValidateRejectsEmptyURL(t *testing.T) {
	if err := (Webhook{}).validate(); err == nil {
		t.Error("a webhook with no url must be refused")
	}
}

func TestSortIsDeterministic(t *testing.T) {
	comps := []Component{
		{Kind: core.ComponentWebhook, Webhook: &Webhook{URL: "https://b"}},
		{Kind: core.ComponentTag, Tag: &Tag{Name: "zeta"}},
		goodWorkflow("zulu"),
		{Kind: core.ComponentTag, Tag: &Tag{Name: "alpha"}},
		goodWorkflow("alpha"),
	}
	Sort(comps)
	want := []string{"alpha", "zulu", "alpha", "zeta", "https://b"}
	for i, c := range comps {
		if c.Key() != want[i] {
			t.Fatalf("component %d = %q, want %q (order %v)", i, c.Key(), want[i], keys(comps))
		}
	}
}

func keys(cs []Component) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, string(c.Kind)+":"+c.Key())
	}
	return out
}

func TestOrderRejectsUnknownKind(t *testing.T) {
	if _, ok := Order("task"); ok {
		t.Error("a work item must have no place in the bundle order")
	}
	if _, ok := Order(core.ComponentWorkflow); !ok {
		t.Error("workflow must have a place in the bundle order")
	}
}

func TestRoundTrip(t *testing.T) {
	comps := []Component{
		goodWorkflow("dev"),
		{Kind: core.ComponentFieldDef, Field: &Field{Key: "points", Type: core.FieldInt, Position: 2}},
		{Kind: core.ComponentTag, Tag: &Tag{Name: "backend", Color: "#abc"}},
		{Kind: core.ComponentProject, Project: &Project{Key: "acme", Name: "Acme", Workflow: goodWorkflow("dev").Workflow}},
		{Kind: core.ComponentWebhook, Webhook: &Webhook{URL: "https://x/y", EventTypes: []string{"task.created"}}},
	}
	raw := encodeAll(t, "team", comps...)

	header, got, err := Read(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if header.Name != "team" || header.Version != core.BundleVersion {
		t.Fatalf("header = %+v", header)
	}
	if header.TixVersion == "" {
		t.Error("the header must identify the tix version that produced the bundle")
	}
	if !header.ExportedAt.Equal(fixedTime) {
		t.Errorf("exported at %v, want %v", header.ExportedAt, fixedTime)
	}
	if len(got) != len(comps) {
		t.Fatalf("read %d components, want %d", len(got), len(comps))
	}
	for i := range got {
		if got[i].Kind != comps[i].Kind || got[i].Key() != comps[i].Key() {
			t.Errorf("component %d = %s %q, want %s %q", i, got[i].Kind, got[i].Key(), comps[i].Kind, comps[i].Key())
		}
	}
}

func TestEncodeIsByteIdentical(t *testing.T) {
	comps := []Component{goodWorkflow("dev"), {Kind: core.ComponentTag, Tag: &Tag{Name: "backend"}}}
	first := encodeAll(t, "team", comps...)
	second := encodeAll(t, "team", comps...)
	if !bytes.Equal(first, second) {
		t.Errorf("re-encoding the same components changed the bytes:\n%s\n%s", first, second)
	}
}

func TestEncoderRefusals(t *testing.T) {
	t.Run("component before header", func(t *testing.T) {
		enc := NewEncoder(io.Discard)
		if err := enc.Component(goodWorkflow("dev")); err == nil {
			t.Error("a component before the header must be refused")
		}
		if enc.Err() == nil {
			t.Error("Err must report the failure that stopped the encoder")
		}
		if err := enc.Component(goodWorkflow("dev")); err == nil {
			t.Error("a stopped encoder must stay stopped")
		}
		if err := enc.Header("x", fixedTime); err == nil {
			t.Error("a stopped encoder must refuse a header too")
		}
	})

	t.Run("second header", func(t *testing.T) {
		enc := NewEncoder(io.Discard)
		if err := enc.Header("x", fixedTime); err != nil {
			t.Fatalf("header: %v", err)
		}
		if err := enc.Header("x", fixedTime); err == nil {
			t.Error("a bundle carries exactly one header")
		}
	})

	t.Run("invalid component", func(t *testing.T) {
		enc := NewEncoder(io.Discard)
		_ = enc.Header("x", fixedTime)
		if err := enc.Component(Component{Kind: "task"}); err == nil {
			t.Error("a work item must never be encodable")
		}
	})

	t.Run("out of order", func(t *testing.T) {
		enc := NewEncoder(io.Discard)
		_ = enc.Header("x", fixedTime)
		if err := enc.Component(Component{Kind: core.ComponentTag, Tag: &Tag{Name: "x"}}); err != nil {
			t.Fatalf("tag: %v", err)
		}
		if err := enc.Component(goodWorkflow("dev")); err == nil {
			t.Error("a workflow after a tag breaks the bundle order and must be refused")
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		enc := NewEncoder(io.Discard)
		_ = enc.Header("x", fixedTime)
		if err := enc.Component(goodWorkflow("dev")); err != nil {
			t.Fatalf("workflow: %v", err)
		}
		if err := enc.Component(goodWorkflow("dev")); err == nil {
			t.Error("the same component twice must be refused")
		}
	})

	t.Run("writer failure", func(t *testing.T) {
		enc := NewEncoder(failingWriter{})
		if err := enc.Header("x", fixedTime); err == nil {
			t.Error("a failing writer must surface")
		}
	})
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk is on fire") }

func TestDecoderRefusals(t *testing.T) {
	head := `{"record":"header","header":{"version":1,"tix_version":"dev","exported_at":"2026-01-02T03:04:05Z"}}` + "\n"
	cases := []struct {
		name string
		doc  string
		want string
	}{
		{"empty", "", "bundle is empty"},
		{"not a header first", `{"record":"component","component":{"kind":"tag","tag":{"name":"x"}}}` + "\n", "line 1"},
		{"unsupported version", `{"record":"header","header":{"version":99}}` + "\n", "version 99 is not supported"},
		{"missing version", `{"record":"header","header":{}}` + "\n", "no schema version"},
		{"not json", "{\n", "not a valid record"},
		{"unknown record type", head + `{"record":"task"}` + "\n", `unknown record type "task"`},
		{"no record type", head + `{}` + "\n", "names no record type"},
		{"repeated header", head + head, "repeats the header record"},
		{"header with no header", `{"record":"header"}` + "\n", "header record with no header"},
		{"component with no component", head + `{"record":"component"}` + "\n", "component record with no component"},
		{"invalid component", head + `{"record":"component","component":{"kind":"workflow","workflow":{"key":"w"}}}` + "\n", "line 2"},
		{"out of order", head +
			`{"record":"component","component":{"kind":"tag","tag":{"name":"x"}}}` + "\n" +
			`{"record":"component","component":{"kind":"workflow","workflow":{"key":"w","definition":{"initial":"a","states":[{"key":"a"},{"key":"b","terminal":true}]}}}}` + "\n",
			"breaks the bundle order"},
		{"repeated component", head +
			`{"record":"component","component":{"kind":"tag","tag":{"name":"x"}}}` + "\n" +
			`{"record":"component","component":{"kind":"tag","tag":{"name":"x"}}}` + "\n",
			`repeats tag "x"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Read(strings.NewReader(tc.doc))
			if err == nil {
				t.Fatalf("Read() = nil, want an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Read() = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// A component record before the header is refused even though the header check
// happens first, because Next is public and callers may drive it themselves.
func TestDecoderRefusesComponentBeforeHeader(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`{"record":"component","component":{"kind":"tag","tag":{"name":"x"}}}` + "\n"))
	if _, err := dec.Next(); err == nil || !strings.Contains(err.Error(), "before the header") {
		t.Errorf("Next() = %v, want a complaint about the missing header", err)
	}
}

func TestDecoderSkipsBlankLinesAndReportsLines(t *testing.T) {
	doc := "\n" + `{"record":"header","header":{"version":1}}` + "\n\n" +
		`{"record":"component","component":{"kind":"tag","tag":{"name":"x"}}}` + "\n"
	dec := NewDecoder(strings.NewReader(doc))
	if _, err := dec.Header(); err != nil {
		t.Fatalf("Header: %v", err)
	}
	if _, err := dec.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if dec.Line() != 4 {
		t.Errorf("Line() = %d, want 4", dec.Line())
	}
	if _, err := dec.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Next() at the end = %v, want io.EOF", err)
	}
}

func TestDecoderReadsFinalLineWithoutNewline(t *testing.T) {
	doc := `{"record":"header","header":{"version":1}}`
	if _, _, err := Read(strings.NewReader(doc)); err != nil {
		t.Errorf("Read of a header with no trailing newline = %v", err)
	}
}

func TestDecoderSurfacesReadFailure(t *testing.T) {
	dec := NewDecoder(failingReader{})
	if _, err := dec.Next(); err == nil || !core.IsKind(err, core.KindInternal) {
		t.Errorf("Next() = %v, want an internal error", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("cable unplugged") }

// A bundle names its source tenant only so an importer can ignore it.
func TestHeaderTenantIsDecodedAndIgnored(t *testing.T) {
	doc := `{"record":"header","header":{"version":1,"tenant":"somewhere-else"}}` + "\n"
	header, _, err := Read(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if header.Tenant != "somewhere-else" {
		t.Errorf("header tenant = %q, want it decoded so an importer can ignore it", header.Tenant)
	}
}

func TestMessageOfNilError(t *testing.T) {
	if got := message(nil); got != "" {
		t.Errorf("message(nil) = %q, want empty", got)
	}
}
