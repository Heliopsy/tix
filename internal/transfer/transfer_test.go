package transfer_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/transfer"
)

func at() time.Time { return time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC) }

func TestKindOrderPutsReferencedRowsFirst(t *testing.T) {
	want := []core.RecordKind{
		core.RecordHeader, core.RecordWorkflow, core.RecordProject, core.RecordFieldDef,
		core.RecordLabel, core.RecordTask, core.RecordDependency, core.RecordComment,
		core.RecordArtifact,
	}
	if len(transfer.Kinds) != len(want) {
		t.Fatalf("Kinds has %d entries, want %d", len(transfer.Kinds), len(want))
	}
	for i, k := range want {
		got, ok := transfer.Order(k)
		if !ok || got != i {
			t.Errorf("Order(%q) = %d, %v; want %d, true", k, got, ok, i)
		}
	}
	if _, ok := transfer.Order("nonsense"); ok {
		t.Error("Order accepted an unknown kind")
	}
}

func TestEncodeRejectsUnknownKindAndMissingPayload(t *testing.T) {
	tests := []struct {
		name string
		rec  core.SnapshotRecord
	}{
		{"unknown kind", core.SnapshotRecord{Kind: "nonsense"}},
		{"missing payload", core.SnapshotRecord{Kind: core.RecordTask}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := transfer.NewEncoder(&buf)
			if err := enc.Encode(tc.rec); !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("Encode = %v, want invalid", err)
			}
			if buf.Len() != 0 {
				t.Errorf("a refused record wrote %q", buf.String())
			}
			if err := enc.Err(); err == nil {
				t.Error("Err returned nil after a failure")
			}
		})
	}
}

func TestEncodeIsDeterministic(t *testing.T) {
	build := func() string {
		var buf bytes.Buffer
		enc := transfer.NewEncoder(&buf)
		if err := enc.Header("acme", at()); err != nil {
			t.Fatalf("header: %v", err)
		}
		task := &core.Task{ID: "t1", Title: "one", CustomFields: map[string]any{
			"zeta": 1, "alpha": "a", "mid": true,
		}}
		if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordTask, Task: task}); err != nil {
			t.Fatalf("task: %v", err)
		}
		return buf.String()
	}
	if first, second := build(), build(); first != second {
		t.Errorf("repeated encoding differed:\n%s\n%s", first, second)
	}
}

func TestEncodeWritesOneRecordPerLine(t *testing.T) {
	var buf bytes.Buffer
	enc := transfer.NewEncoder(&buf)
	if err := enc.Header("acme", at()); err != nil {
		t.Fatalf("header: %v", err)
	}
	for _, id := range []string{"t1", "t2"} {
		if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordTask, Task: &core.Task{ID: id}}); err != nil {
			t.Fatalf("task %s: %v", id, err)
		}
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("wrote %d lines, want 3: %q", len(lines), buf.String())
	}
}

func TestDecodeRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	enc := transfer.NewEncoder(&buf)
	if err := enc.Header("acme", at()); err != nil {
		t.Fatalf("header: %v", err)
	}
	if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordProject, Project: &core.Project{Key: "infra"}}); err != nil {
		t.Fatalf("project: %v", err)
	}
	if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordTask, Task: &core.Task{ID: "t1", Title: "one"}}); err != nil {
		t.Fatalf("task: %v", err)
	}

	dec := transfer.NewDecoder(&buf)
	header, err := dec.Header()
	if err != nil {
		t.Fatalf("Header: %v", err)
	}
	if header.Version != core.SnapshotVersion || header.TenantKey != "acme" {
		t.Errorf("header = %+v", header)
	}
	kinds := []core.RecordKind{}
	for {
		rec, err := dec.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		kinds = append(kinds, rec.Kind)
	}
	if len(kinds) != 2 || kinds[0] != core.RecordProject || kinds[1] != core.RecordTask {
		t.Errorf("kinds = %v", kinds)
	}
}

func TestDecodeHeaderRefusals(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", "empty"},
		{"missing version", `{"kind":"header","header":{"tenant_key":"acme"}}` + "\n", "no format version"},
		{"unsupported version", `{"kind":"header","header":{"version":99}}` + "\n", "version 99"},
		{"no header record", `{"kind":"project","project":{"key":"infra"}}` + "\n", "not a header"},
		{"header without payload", `{"kind":"header"}` + "\n", "carries no header payload"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := transfer.NewDecoder(strings.NewReader(tc.input)).Header()
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("Header = %v, want invalid", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Header error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestDecodeRefusesBadLines(t *testing.T) {
	head := `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n"
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"malformed line names its number", head + "not json\n", "line 2"},
		{"truncated record", head + `{"kind":"task","task":{"id":"t1"`, "line 2"},
		{"unknown kind", head + `{"kind":"nonsense","task":{"id":"t1"}}` + "\n", "unknown record kind"},
		{"missing payload", head + `{"kind":"task"}` + "\n", "carries no task payload"},
		{"repeated header", head + head, "repeats the header"},
		{"out of order", head +
			`{"kind":"task","task":{"id":"t1"}}` + "\n" +
			`{"kind":"project","project":{"key":"infra"}}` + "\n", "breaks the snapshot order"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := transfer.NewDecoder(strings.NewReader(tc.input))
			if _, err := dec.Header(); err != nil {
				t.Fatalf("Header: %v", err)
			}
			var err error
			for err == nil {
				_, err = dec.Next()
			}
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("Next = %v, want invalid", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestDecodeSkipsBlankLines(t *testing.T) {
	input := "\n" + `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n\n" +
		`{"kind":"task","task":{"id":"t1"}}` + "\n\n"
	dec := transfer.NewDecoder(strings.NewReader(input))
	if _, err := dec.Header(); err != nil {
		t.Fatalf("Header: %v", err)
	}
	rec, err := dec.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if rec.Task.ID != "t1" {
		t.Errorf("task = %+v", rec.Task)
	}
	if _, err := dec.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Next at end = %v, want EOF", err)
	}
}

// The decoder must hand back a record while the rest of the stream is still
// unwritten, or a snapshot could not be piped between two processes.
func TestDecodeReturnsARecordBeforeTheRestIsWritten(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pr.Close() }()

	go func() {
		_, _ = pw.Write([]byte(`{"kind":"header","header":{"version":1,"tenant_key":"acme"}}` + "\n"))
		_, _ = pw.Write([]byte(`{"kind":"project","project":{"key":"infra"}}` + "\n"))
	}()

	dec := transfer.NewDecoder(pr)
	if _, err := dec.Header(); err != nil {
		t.Fatalf("Header: %v", err)
	}
	rec, err := dec.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if rec.Project.Key != "infra" {
		t.Errorf("project = %+v", rec.Project)
	}
}

func TestDecodeReportsReaderFailure(t *testing.T) {
	boom := errors.New("read failure")
	dec := transfer.NewDecoder(failingReader{err: boom})
	if _, err := dec.Next(); !errors.Is(err, boom) {
		t.Errorf("Next = %v, want the reader failure", err)
	}
}

func TestEncodeReportsWriterFailure(t *testing.T) {
	boom := errors.New("write failure")
	enc := transfer.NewEncoder(failingWriter{err: boom})
	if err := enc.Header("acme", at()); !errors.Is(err, boom) {
		t.Errorf("Header = %v, want the writer failure", err)
	}
	if err := enc.Encode(core.SnapshotRecord{Kind: core.RecordTask, Task: &core.Task{}}); !errors.Is(err, boom) {
		t.Errorf("Encode after a failure = %v, want the writer failure", err)
	}
}

type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

func TestDecodeAcceptsEveryKindAndTracksTheLineNumber(t *testing.T) {
	lines := []string{
		`{"kind":"header","header":{"version":1,"tenant_key":"acme"}}`,
		`{"kind":"workflow","workflow":{"key":"flow"}}`,
		`{"kind":"project","project":{"key":"infra"}}`,
		`{"kind":"field_def","field_def":{"key":"points"}}`,
		`{"kind":"tag","tag":{"name":"ops"}}`,
		`{"kind":"task","task":{"id":"t1"}}`,
		`{"kind":"dependency","dependency":{"task_id":"t2","depends_on":"t1"}}`,
		`{"kind":"comment","comment":{"id":"c1","task_id":"t1","body":"hi"}}`,
		`{"kind":"artifact","artifact":{"id":"a1","task_id":"t1","kind":"result"}}`,
	}
	dec := transfer.NewDecoder(strings.NewReader(strings.Join(lines, "\n") + "\n"))
	for want := 1; want <= len(lines); want++ {
		rec, err := dec.Next()
		if err != nil {
			t.Fatalf("record %d: %v", want, err)
		}
		if dec.Line() != want {
			t.Errorf("Line after record %d = %d", want, dec.Line())
		}
		if !transfer.HasPayload(*rec) {
			t.Errorf("record %d of kind %q reported no payload", want, rec.Kind)
		}
	}
	if _, err := dec.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Next at end = %v, want EOF", err)
	}
}

func TestHasPayloadRejectsAnUnknownKind(t *testing.T) {
	if transfer.HasPayload(core.SnapshotRecord{Kind: "nonsense"}) {
		t.Error("HasPayload accepted an unknown kind")
	}
}
