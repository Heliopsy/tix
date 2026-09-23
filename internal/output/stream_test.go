// SPDX-License-Identifier: AGPL-3.0-or-later

package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

func tasks(n int) []core.Task {
	out := make([]core.Task, n)
	for i := range out {
		out[i] = core.Task{ID: string(rune('a' + i)), Title: "t", Status: "todo"}
	}
	return out
}

// ndjson is the format that lets a caller pipe a large listing without either
// side holding it in memory.
func TestNDJSONStreamIsOneObjectPerLine(t *testing.T) {
	var buf bytes.Buffer
	s := output.NewStream(output.FormatNDJSON, &buf)
	for _, task := range tasks(3) {
		if err := s.Write(task); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%s", len(lines), buf.String())
	}
	for i, line := range lines {
		var got core.Task
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Errorf("line %d is not a standalone json object: %v", i, err)
		}
	}
}

// Each record must be written as it arrives, or the format is not streaming.
func TestNDJSONWritesIncrementally(t *testing.T) {
	var buf bytes.Buffer
	s := output.NewStream(output.FormatNDJSON, &buf)

	if err := s.Write(core.Task{ID: "a"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("nothing was written before Close; the stream is buffering")
	}
	first := buf.Len()

	if err := s.Write(core.Task{ID: "b"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if buf.Len() <= first {
		t.Error("the second record did not reach the writer")
	}
	_ = s.Close()
}

func TestJSONStreamProducesAValidArray(t *testing.T) {
	var buf bytes.Buffer
	s := output.NewStream(output.FormatJSON, &buf)
	for _, task := range tasks(3) {
		if err := s.Write(task); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var got []core.Task
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not a valid json array: %v\n%s", err, buf.String())
	}
	if len(got) != 3 {
		t.Errorf("decoded %d records, want 3", len(got))
	}
}

func TestJSONStreamWithNoRecordsIsAnEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := output.NewStream(output.FormatJSON, &buf).Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	var got []core.Task
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("empty stream is not valid json: %v (%q)", err, buf.String())
	}
	if len(got) != 0 {
		t.Errorf("got %d records, want 0", len(got))
	}
}

func TestYAMLStreamSeparatesDocuments(t *testing.T) {
	var buf bytes.Buffer
	s := output.NewStream(output.FormatYAML, &buf)
	for _, task := range tasks(2) {
		if err := s.Write(task); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if strings.Count(buf.String(), "---") != 2 {
		t.Errorf("expected two yaml documents:\n%s", buf.String())
	}
}

// Table cannot stream because it needs every row to size its columns. It must
// still produce correct output through the same interface.
func TestTableStreamRendersOnClose(t *testing.T) {
	var buf bytes.Buffer
	s := output.NewStream(output.FormatTable, &buf)
	for _, task := range tasks(2) {
		if err := s.Write(task); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if buf.Len() != 0 {
		t.Error("table wrote before Close; it should render once at the end")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("table produced no output")
	}
}

func TestUnknownStreamFormatFallsBackToTable(t *testing.T) {
	var buf bytes.Buffer
	s := output.NewStream("nonsense", &buf)
	if err := s.Write(core.Task{ID: "a"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("fallback produced no output")
	}
}

// The non-streaming path must agree with the streaming one, so a caller can
// switch between them without the output changing shape.
func TestNDJSONFormatterMatchesTheStream(t *testing.T) {
	list := tasks(3)

	var direct bytes.Buffer
	if err := output.New(output.FormatNDJSON).Format(&direct, list); err != nil {
		t.Fatalf("Format: %v", err)
	}

	var streamed bytes.Buffer
	s := output.NewStream(output.FormatNDJSON, &streamed)
	for _, task := range list {
		if err := s.Write(task); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if direct.String() != streamed.String() {
		t.Errorf("formatter and stream disagree:\n%q\nvs\n%q", direct.String(), streamed.String())
	}
}

func TestNDJSONFormatterOnASingleValue(t *testing.T) {
	var buf bytes.Buffer
	if err := output.New(output.FormatNDJSON).Format(&buf, core.Task{ID: "a"}); err != nil {
		t.Fatalf("Format: %v", err)
	}
	if strings.Count(strings.TrimRight(buf.String(), "\n"), "\n") != 0 {
		t.Errorf("a single value should be one line:\n%s", buf.String())
	}
}

func TestNDJSONFormatterOnNil(t *testing.T) {
	var buf bytes.Buffer
	if err := output.New(output.FormatNDJSON).Format(&buf, nil); err != nil {
		t.Fatalf("Format(nil): %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("nil should produce nothing, got %q", buf.String())
	}
}

// A secret must not leak through the streaming path either.
func TestStreamDoesNotLeakSecrets(t *testing.T) {
	const secret = "STREAM-SECRET"
	for _, format := range output.Formats {
		var buf bytes.Buffer
		s := output.NewStream(format, &buf)
		if err := s.Write(core.WebhookEndpoint{ID: "w1", URL: "https://x", Secret: secret}); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if strings.Contains(buf.String(), secret) {
			t.Errorf("%s stream leaked the secret:\n%s", format, buf.String())
		}
	}
}
