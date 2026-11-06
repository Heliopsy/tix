// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		in   string
		want Format
		bad  bool
	}{
		{"", "", false}, {"csv", FormatCSV, false}, {"TSV", FormatCSV, false}, {"json", FormatJSON, false},
		{"ndjson", FormatNDJSON, false}, {"jsonl", FormatNDJSON, false}, {"xlsx", "", true},
	}
	for _, tt := range tests {
		got, err := ParseFormat(tt.in)
		if got != tt.want || (err != nil) != tt.bad {
			t.Errorf("ParseFormat(%q) = %q, %v", tt.in, got, err)
		}
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name, head string
		want       Format
	}{
		{"tasks.csv", "[", FormatCSV},
		{"tasks.JSON", "", FormatJSON},
		{"tasks.jsonl", "", FormatNDJSON},
		{"", "\ufeff  [{\"a\":1}]", FormatJSON},
		{"", "{\"a\":1}\n{\"a\":2}\n", FormatNDJSON},
		{"", "{\"a\":1}\n", FormatNDJSON},
		{"", "{\"tasks\":[]}", FormatJSON},
		{"", "{\n  \"cards\": [", FormatJSON},
		{"", "title,status\n", FormatCSV},
		{"", "", FormatCSV},
	}
	for _, tt := range tests {
		if got := DetectFormat(tt.name, []byte(tt.head)); got != tt.want {
			t.Errorf("DetectFormat(%q, %q) = %q, want %q", tt.name, tt.head, got, tt.want)
		}
	}
}

func TestDetectDelimiter(t *testing.T) {
	tests := []struct {
		head string
		want rune
	}{
		{"a,b,c\n1;2;3;4;5", ','},
		{"a;b;c\n", ';'},
		{"a\tb\tc", '\t'},
		{"a|b|c", '|'},
		{`"x;y;z",b` + "\n", ','},
		{"single", ','},
	}
	for _, tt := range tests {
		if got := DetectDelimiter([]byte(tt.head)); got != tt.want {
			t.Errorf("DetectDelimiter(%q) = %q, want %q", tt.head, got, tt.want)
		}
	}
}

func TestCSVReading(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		opts    ReadOptions
		headers []string
		rows    []map[string]string
		lines   []int
	}{
		{
			name: "bom and semicolons", in: "\ufefftitle;status\nA;todo\n",
			headers: []string{"title", "status"}, rows: []map[string]string{{"title": "A", "status": "todo"}}, lines: []int{2},
		},
		{
			name: "duplicate and empty headers", in: "Labels,Labels,,Labels\na,b,c,d\n",
			headers: []string{"Labels", "Labels (2)", "Column 3", "Labels (3)"},
			rows:    []map[string]string{{"Labels": "a", "Labels (2)": "b", "Column 3": "c", "Labels (3)": "d"}},
			lines:   []int{2},
		},
		{
			name: "blank rows skipped and short rows padded", in: "a,b\n\n,\n1\n , \n2,3\n",
			headers: []string{"a", "b"}, rows: []map[string]string{{"a": "1"}, {"a": "2", "b": "3"}}, lines: []int{4, 6},
		},
		{
			name: "formula guard undone", in: "title,note\n'=SUM(A1),'+1\n'-x,'@y\n'plain,''=z\n",
			headers: []string{"title", "note"},
			rows: []map[string]string{
				{"title": "=SUM(A1)", "note": "+1"},
				{"title": "-x", "note": "@y"},
				{"title": "'plain", "note": "''=z"},
			},
			lines: []int{2, 3, 4},
		},
		{
			name: "explicit tab delimiter", in: "a\tb\n1\t2\n", opts: ReadOptions{Format: FormatCSV, Delimiter: '\t'},
			headers: []string{"a", "b"}, rows: []map[string]string{{"a": "1", "b": "2"}}, lines: []int{2},
		},
		{
			name: "multiline cell keeps its record line", in: "a,b\n\"x\ny\",1\nz,2\n",
			headers: []string{"a", "b"}, rows: []map[string]string{{"a": "x\ny", "b": "1"}, {"a": "z", "b": "2"}}, lines: []int{2, 4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd, rows := openAll(t, strings.NewReader(tt.in), tt.opts)
			if !reflect.DeepEqual(rd.Headers(), tt.headers) {
				t.Errorf("headers = %q, want %q", rd.Headers(), tt.headers)
			}
			if len(rows) != len(tt.rows) {
				t.Fatalf("rows = %d, want %d", len(rows), len(tt.rows))
			}
			for i, want := range tt.rows {
				for k, v := range want {
					if rows[i].Values[k] != v {
						t.Errorf("row %d %q = %q, want %q", i, k, rows[i].Values[k], v)
					}
				}
				if rows[i].Line != tt.lines[i] {
					t.Errorf("row %d line = %d, want %d", i, rows[i].Line, tt.lines[i])
				}
			}
		})
	}
}

func TestBaseHeader(t *testing.T) {
	for in, want := range map[string]string{
		"Labels": "Labels", "Labels (2)": "Labels", "Labels (12)": "Labels", "Size (1)": "Size (1)", "a (b)": "a (b)",
	} {
		if got := BaseHeader(in); got != want {
			t.Errorf("BaseHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJSONReading(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		opts    ReadOptions
		headers []string
		source  string
		rows    []map[string]string
		lines   []int
	}{
		{
			name:    "array with nesting and nulls",
			in:      `[{"title":"A","fields":{"summary":"S","n":{"x":1.5}},"tags":["a","b"],"who":[{"login":"x"},{"id":2},{"displayName":"Y"}],"gone":null,"ok":true}]`,
			headers: []string{"title", "fields.summary", "fields.n.x", "tags", "who", "gone", "ok"},
			rows:    []map[string]string{{"title": "A", "fields.summary": "S", "fields.n.x": "1.5", "tags": "a, b", "who": "x, Y", "gone": "", "ok": "true"}},
			lines:   []int{1},
		},
		{
			name:    "headers are the union in first-seen order",
			in:      `[{"a":1},{"b":2,"a":3},{"c":[[1]],"d":false}]`,
			headers: []string{"a", "b", "c", "d"},
			rows:    []map[string]string{{"a": "1"}, {"a": "3", "b": "2"}, {"c": "", "d": "false"}},
			lines:   []int{1, 2, 3},
		},
		{
			name: "tix export wrapper streams tasks", in: `{"next":"x","tasks":[{"ref":"P-1","title":"T","status":"todo"}],"after":1}`,
			headers: []string{"ref", "title", "status"}, source: SourceTix,
			rows: []map[string]string{{"ref": "P-1", "title": "T", "status": "todo"}}, lines: []int{1},
		},
		{
			name: "issues wrapper after a non-array tasks member", in: `{"tasks":{"n":1},"issues":[{"Issue key":"K-1","Summary":"S"}]}`,
			headers: []string{"Issue key", "Summary"}, source: SourceJira,
			rows: []map[string]string{{"Issue key": "K-1"}}, lines: []int{1},
		},
		{
			name: "cards without lists read as plain rows", in: `{"cards":[{"name":"c1"},{"name":"c2"}]}`,
			headers: []string{"name"}, rows: []map[string]string{{"name": "c1"}, {"name": "c2"}}, lines: []int{1, 2},
		},
		{
			name: "ndjson with blank lines", in: "{\"a\":\"1\"}\n\n{\"a\":\"2\",\"b\":\"3\"}\n",
			opts: ReadOptions{Format: FormatNDJSON}, headers: []string{"a", "b"},
			rows: []map[string]string{{"a": "1"}, {"a": "2", "b": "3"}}, lines: []int{1, 3},
		},
		{
			name: "ndjson without a final newline", in: "{\"a\":\"1\"}",
			opts: ReadOptions{Format: FormatNDJSON}, headers: []string{"a"}, rows: []map[string]string{{"a": "1"}}, lines: []int{1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd, rows := openAll(t, strings.NewReader(tt.in), tt.opts)
			if !reflect.DeepEqual(rd.Headers(), tt.headers) {
				t.Errorf("headers = %q, want %q", rd.Headers(), tt.headers)
			}
			if rd.Source() != tt.source {
				t.Errorf("source = %q, want %q", rd.Source(), tt.source)
			}
			if len(rows) != len(tt.rows) {
				t.Fatalf("rows = %d, want %d", len(rows), len(tt.rows))
			}
			for i, want := range tt.rows {
				for k, v := range want {
					if rows[i].Values[k] != v {
						t.Errorf("row %d %q = %q, want %q", i, k, rows[i].Values[k], v)
					}
				}
				if rows[i].Line != tt.lines[i] {
					t.Errorf("row %d line = %d, want %d", i, rows[i].Line, tt.lines[i])
				}
			}
		})
	}
}

func TestJSONStreamsPastTheHeaderSample(t *testing.T) {
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < headerSample+50; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"title":"t"}`)
	}
	b.WriteString(`,{"title":"late","extra":"x"}]`)
	rd, rows := openAll(t, strings.NewReader(b.String()), ReadOptions{})
	if len(rows) != headerSample+51 {
		t.Fatalf("rows = %d", len(rows))
	}
	if !reflect.DeepEqual(rd.Headers(), []string{"title"}) {
		t.Errorf("headers = %q", rd.Headers())
	}
	if last := rows[len(rows)-1]; last.Values["title"] != "late" || last.Line != headerSample+51 {
		t.Errorf("last row = %+v", last)
	}
}

func TestOpenRefusesMalformedInput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		opts ReadOptions
		want string
	}{
		{"empty csv", "", ReadOptions{Format: FormatCSV}, "the file is empty"},
		{"empty json", "  ", ReadOptions{Format: FormatJSON}, "the file is empty"},
		{"json scalar", `"x"`, ReadOptions{Format: FormatJSON}, "neither an array"},
		{"array of scalars", `[1]`, ReadOptions{Format: FormatJSON}, "record 1 is not a JSON object"},
		{"broken json", `[{"a":}]`, ReadOptions{Format: FormatJSON}, "reading JSON"},
		{"no array member", `{"x":[]}`, ReadOptions{Format: FormatJSON}, "no tasks, issues or cards"},
		{"cards not an array", `{"cards":{}}`, ReadOptions{Format: FormatJSON}, "cards member is not an array"},
		{"empty array", `[]`, ReadOptions{Format: FormatJSON}, "holds no tasks"},
		{"ndjson array line", "[1]\n", ReadOptions{Format: FormatNDJSON}, "line 1 is not a single JSON object"},
		{"ndjson two values", "{\"a\":1} {\"a\":2}\n", ReadOptions{Format: FormatNDJSON}, "line 1 is not a single JSON object"},
		{"ndjson broken", "{\"a\":1}\n{oops\n", ReadOptions{Format: FormatNDJSON}, "line 2 is not valid JSON"},
		{"bad trello", `{"cards":[{"idList":1}],"lists":[]}`, ReadOptions{Format: FormatJSON}, "reading the Trello cards"},
		{"unknown format", "a", ReadOptions{Format: "xml"}, "not one of csv"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := drain(strings.NewReader(tt.in), tt.opts)
			assertInvalid(t, err, tt.want)
		})
	}
}

func drain(r io.Reader, opts ReadOptions) error {
	rd, err := Open(r, opts)
	if err != nil {
		return err
	}
	for {
		if _, err := rd.Next(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func assertInvalid(t *testing.T, err error, want string) {
	t.Helper()
	var domain *core.Error
	if !errors.As(err, &domain) || domain.Kind != core.KindInvalid {
		t.Fatalf("error = %v, want an invalid core error", err)
	}
	if !strings.Contains(domain.Message, want) {
		t.Errorf("message = %q, want it to contain %q", domain.Message, want)
	}
}

func TestLimits(t *testing.T) {
	big := strings.Repeat("x", 40)
	tests := []struct {
		name   string
		in     string
		format Format
		limits Limits
		want   string
	}{
		{"csv rows", "a\n1\n2\n3\n", FormatCSV, Limits{MaxRows: 2}, "the file has more than 2 rows; split it and import the parts"},
		{"json rows", `[{"a":1},{"a":2},{"a":3}]`, FormatJSON, Limits{MaxRows: 2}, "more than 2 rows"},
		{"ndjson rows", "{\"a\":1}\n{\"a\":2}\n{\"a\":3}\n", FormatNDJSON, Limits{MaxRows: 2}, "more than 2 rows"},
		{"trello rows", `{"cards":[{},{},{}],"lists":[]}`, FormatJSON, Limits{MaxRows: 2}, "more than 2 rows"},
		{"csv header columns", "a,b,c\n1,2\n", FormatCSV, Limits{MaxColumns: 2}, "the file has more than 2 columns"},
		{"csv row columns", "a,b\n1,2,3\n", FormatCSV, Limits{MaxColumns: 2}, "more than 2 columns"},
		{"json columns", `[{"a":1,"b":{"c":2,"d":3}}]`, FormatJSON, Limits{MaxColumns: 2}, "more than 2 columns"},
		{"json union columns", `[{"a":1,"b":2},{"c":3}]`, FormatJSON, Limits{MaxColumns: 2}, "more than 2 columns"},
		{"csv cell", "title,body\nt," + big + "\n", FormatCSV, Limits{MaxCell: 10}, `row 2: the "body" cell is longer than the 10 bytes cell limit`},
		{"json cell", `[{"body":"` + big + `"}]`, FormatJSON, Limits{MaxCell: 10}, `row 1: the "body" cell is longer`},
		{"trello cell", `{"cards":[{"name":"` + big + `"}],"lists":[]}`, FormatJSON, Limits{MaxCell: 10}, `the "name" cell is longer`},
		{"csv bytes", "a\n" + strings.Repeat("row\n", 600), FormatCSV, Limits{MaxBytes: 1024}, "the file is larger than 1 KiB; split it and import the parts"},
		{"csv bytes in head", "a\n" + strings.Repeat("row\n", 600), "", Limits{MaxBytes: 1 << 20 / 512}, "larger than 2 KiB"},
		{"json bytes", "[" + strings.Repeat(`{"a":"`+big+big+`"},`, 20000) + `{"a":1}]`, FormatJSON, Limits{MaxBytes: 1 << 20}, "larger than 1 MiB"},
		{"ndjson bytes", strings.Repeat("{\"a\":1}\n", 200000), FormatNDJSON, Limits{MaxBytes: 100000}, "larger than 100000 bytes"},
		{"wrapper bytes", `{"tasks":{"x":"` + strings.Repeat("y", 100000) + `"}}`, FormatJSON, Limits{MaxBytes: 70000}, "larger than"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := drain(strings.NewReader(tt.in), ReadOptions{Format: tt.format, Limits: tt.limits})
			assertInvalid(t, err, tt.want)
		})
	}
}

func TestLimitsAllowTheBoundary(t *testing.T) {
	in := "a,b\n1,2\n3,4\n"
	limits := Limits{MaxRows: 2, MaxColumns: 2, MaxCell: 1, MaxBytes: int64(len(in))}
	if err := drain(strings.NewReader(in), ReadOptions{Limits: limits}); err != nil {
		t.Fatalf("a file exactly at every limit was refused: %v", err)
	}
}
