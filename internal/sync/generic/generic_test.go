package generic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	extsync "github.com/heliopsy/tix/internal/sync"
)

func write(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

func fetch(t *testing.T, i *Importer, opt extsync.Options) extsync.Batch {
	t.Helper()
	batch, err := i.Fetch(context.Background(), opt)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	return batch
}

const csvBody = `id,title,status,updated
1,first,To Do,2026-01-01T00:00:00Z
2,second,Done,2026-01-02T00:00:00Z
3,third,To Do,2026-01-03T00:00:00Z
`

func newCSV(t *testing.T) *Importer {
	t.Helper()
	i, err := New(Options{Path: write(t, "rows.csv", csvBody), UpdatedField: "updated"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return i
}

func TestGenericReadsCSVRows(t *testing.T) {
	i := newCSV(t)
	if i.System() != System {
		t.Errorf("System() = %q", i.System())
	}
	batch := fetch(t, i, extsync.Options{})
	if len(batch.Records) != 3 {
		t.Fatalf("read %d records, want 3", len(batch.Records))
	}
	if batch.Records[0].Fields["title"] != "first" {
		t.Errorf("first row = %v", batch.Records[0].Fields)
	}
	if batch.Cursor != "2026-01-03T00:00:00Z" {
		t.Errorf("cursor = %q, want the newest record's update time", batch.Cursor)
	}
	if batch.Page != "" {
		t.Errorf("page = %q, want the file exhausted", batch.Page)
	}
}

func TestGenericPagesThroughAFile(t *testing.T) {
	i := newCSV(t)
	var seen []string
	opt := extsync.Options{Size: 2}
	for {
		batch := fetch(t, i, opt)
		for _, r := range batch.Records {
			seen = append(seen, extsync.Text(r.Fields["id"]))
		}
		if batch.Page == "" {
			break
		}
		opt.Page = batch.Page
	}
	if strings.Join(seen, ",") != "1,2,3" {
		t.Errorf("paged records = %v, want every row exactly once", seen)
	}
}

func TestGenericHonoursTheCursor(t *testing.T) {
	i := newCSV(t)
	batch := fetch(t, i, extsync.Options{Cursor: "2026-01-03T00:00:00Z"})
	if len(batch.Records) != 1 || batch.Records[0].Fields["id"] != "3" {
		t.Fatalf("cursored fetch returned %d records", len(batch.Records))
	}
	full := fetch(t, i, extsync.Options{Cursor: "2026-01-03T00:00:00Z", Full: true})
	if len(full.Records) != 3 {
		t.Errorf("full refresh returned %d records, want every row", len(full.Records))
	}
	garbage := fetch(t, i, extsync.Options{Cursor: "not a time"})
	if len(garbage.Records) != 3 {
		t.Errorf("an unreadable cursor dropped records")
	}
}

func TestGenericReadsJSONWithNestedFields(t *testing.T) {
	body := `[
		{"id":"a","fields":{"summary":"nested","status":{"name":"Done"}},"updated":"2026-01-01T00:00:00Z"},
		{"id":"b","fields":{"summary":"second"},"updated":"2026-01-02T00:00:00Z"}
	]`
	i, err := New(Options{Path: write(t, "rows.json", body), UpdatedField: "updated"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	batch := fetch(t, i, extsync.Options{})
	if len(batch.Records) != 2 {
		t.Fatalf("read %d records, want 2", len(batch.Records))
	}
	got, ok := extsync.Lookup(batch.Records[0].Fields, "fields.status.name")
	if !ok || extsync.Text(got) != "Done" {
		t.Errorf("nested field = %v %v", got, ok)
	}
}

func TestGenericAcceptsWrapperAndNDJSONDocuments(t *testing.T) {
	wrapper := `{"records":[{"id":"a"},{"id":"b"}]}`
	i, err := New(Options{Path: write(t, "wrap.json", wrapper)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := fetch(t, i, extsync.Options{}); len(got.Records) != 2 {
		t.Errorf("wrapper document produced %d records", len(got.Records))
	}

	single := `{"id":"only"}`
	j, err := New(Options{Path: write(t, "one.json", single)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := fetch(t, j, extsync.Options{}); len(got.Records) != 1 {
		t.Errorf("single object produced %d records", len(got.Records))
	}

	nd := "{\"id\":\"a\"}\n{\"id\":\"b\"}\n"
	k, err := New(Options{Path: write(t, "rows.ndjson", nd)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := fetch(t, k, extsync.Options{}); len(got.Records) != 2 {
		t.Errorf("ndjson produced %d records", len(got.Records))
	}
}

func TestGenericReadsTSV(t *testing.T) {
	body := "id\ttitle\n1\tfirst\n"
	i, err := New(Options{Path: write(t, "rows.tsv", body)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	batch := fetch(t, i, extsync.Options{})
	if len(batch.Records) != 1 || batch.Records[0].Fields["title"] != "first" {
		t.Errorf("tsv rows = %v", batch.Records)
	}
}

func TestGenericRefusesUnusableFiles(t *testing.T) {
	tests := []struct {
		name, file, body, want string
	}{
		{"unknown extension", "rows.txt", "a", "neither csv nor json"},
		{"empty csv", "rows.csv", "", "header row"},
		{"empty json", "rows.json", "", "empty"},
		{"broken json", "rows.json", "{oops", "parsing json"},
		{"ragged csv", "rows.csv", "a,b\n\"unterminated\n", "csv"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i, err := New(Options{Path: write(t, tc.file, tc.body)})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			_, err = i.Fetch(context.Background(), extsync.Options{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Fetch = %v, want an error mentioning %q", err, tc.want)
			}
		})
	}
}

func TestGenericRefusesAMissingPathOrFile(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("New accepted a source with no file")
	}
	i, err := New(Options{Path: filepath.Join(t.TempDir(), "absent.csv")})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := i.Fetch(context.Background(), extsync.Options{}); err == nil {
		t.Fatal("Fetch accepted a missing file")
	}
}

func TestGenericRejectsABadPageToken(t *testing.T) {
	i := newCSV(t)
	if _, err := i.Fetch(context.Background(), extsync.Options{Page: "later"}); err == nil {
		t.Fatal("Fetch accepted a page token that is not an offset")
	}
	batch := fetch(t, i, extsync.Options{Page: "99"})
	if len(batch.Records) != 0 {
		t.Errorf("an offset past the end returned %d records", len(batch.Records))
	}
}
