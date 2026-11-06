// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func write(t *testing.T, format core.TaskFormat, columns []string, opts WriteOptions, rows ...[]any) string {
	t.Helper()
	var buf bytes.Buffer
	w, err := NewWriter(&buf, format, columns, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if err := w.Write(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestWritersShapeEachFormat(t *testing.T) {
	at := time.Date(2026, 11, 6, 9, 30, 0, 0, time.UTC)
	row := []any{"=1", []string{"a", "b"}, &at, map[string]any{"k": 1}, nil}
	cols := []string{"title", "tags", "due", "custom:meta", "parent"}
	for _, tc := range []struct {
		name   string
		format core.TaskFormat
		opts   WriteOptions
		rows   [][]any
		want   string
	}{
		{"csv", core.TaskFormatCSV, WriteOptions{Delimiter: ';'}, [][]any{row},
			"title;tags;due;custom:meta;parent\n'=1;a, b;2026-11-06T09:30:00Z;\"{\"\"k\"\":1}\";\n"},
		{"json", core.TaskFormatJSON, WriteOptions{}, [][]any{row, row},
			"[\n" + `{"title":"=1","tags":["a","b"],"due":"2026-11-06T09:30:00Z","custom:meta":{"k":1},"parent":null}` +
				",\n" + `{"title":"=1","tags":["a","b"],"due":"2026-11-06T09:30:00Z","custom:meta":{"k":1},"parent":null}` + "\n]\n"},
		{"empty json", core.TaskFormatJSON, WriteOptions{}, nil, "[]\n"},
		{"ndjson", core.TaskFormatNDJSON, WriteOptions{Date: func(t time.Time) string { return t.Format(time.DateOnly) }},
			[][]any{row}, `{"title":"=1","tags":["a","b"],"due":"2026-11-06","custom:meta":{"k":1},"parent":null}` + "\n"},
		{"markdown", core.TaskFormatMarkdown, WriteOptions{}, [][]any{{"a|b\nc", nil, nil, nil, nil}},
			"| title | tags | due | custom:meta | parent |\n| --- | --- | --- | --- | --- |\n| a\\|b<br>c |  |  |  |  |\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := write(t, tc.format, cols, tc.opts, tc.rows...); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestTextRendersCommentsAndHistory(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	date := func(t time.Time) string { return t.Format(time.DateOnly) }
	got := Text([]Comment{{Author: "ana", At: at, Body: "one"}, {Author: "bo", At: at, Body: "two"}}, date)
	if got != "ana (2026-01-02): one\n\nbo (2026-01-02): two" {
		t.Errorf("comments = %q", got)
	}
	if got := Text([]Change{{Actor: "ana", At: at, Action: "task.create"}}, date); got != "2026-01-02 ana task.create" {
		t.Errorf("history = %q", got)
	}
	if got := Text(true, date); got != "true" {
		t.Errorf("bool = %q", got)
	}
	if !strings.Contains(write(t, core.TaskFormatJSON, []string{"comments"}, WriteOptions{},
		[]any{[]Comment{{Author: "ana", At: at, Body: "x"}}}), `"author":"ana"`) {
		t.Error("json comments lost their author")
	}
}

func TestNewWriterRefusesAnUnknownFormat(t *testing.T) {
	if _, err := NewWriter(&bytes.Buffer{}, "xml", nil, WriteOptions{}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("err = %v", err)
	}
}
