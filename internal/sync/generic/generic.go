// SPDX-License-Identifier: AGPL-3.0-or-later

// Package generic imports records from a CSV or JSON file, so a system with no
// adapter of its own is imported by shaping a file and writing a mapping.
package generic

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// System is the name recorded on every external reference this adapter writes.
const System = core.SystemGeneric

// Options configures one file import.
type Options struct {
	// Path is the CSV or JSON file to read.
	Path string
	// Format overrides detection by extension: "csv" or "json".
	Format string
	// UpdatedField is the dotted path holding each record's update time, which
	// is what makes an incremental refresh possible.
	UpdatedField string
	// PageSize bounds how many records one Fetch returns.
	PageSize int
}

// Importer reads records from a local file.
type Importer struct {
	opt     Options
	records []extsync.Record
	loaded  bool
}

// New builds a file importer.
func New(opt Options) (*Importer, error) {
	if strings.TrimSpace(opt.Path) == "" {
		return nil, core.Invalid("the generic adapter needs a file to read; set %s",
			extsync.EnvPrefix+"<SOURCE>_"+extsync.EnvFile)
	}
	return &Importer{opt: opt}, nil
}

// System names the external system these records came from.
func (i *Importer) System() string { return System }

// Fetch returns one page of records, oldest first, so a cursor committed after
// a page never skips a record the next run still needs.
func (i *Importer) Fetch(_ context.Context, opt extsync.Options) (extsync.Batch, error) {
	if err := i.load(); err != nil {
		return extsync.Batch{}, err
	}
	eligible := i.eligible(opt)

	start := 0
	if opt.Page != "" {
		n, err := strconv.Atoi(opt.Page)
		if err != nil || n < 0 {
			return extsync.Batch{}, core.Invalid("page token %q is not a file offset", opt.Page)
		}
		start = n
	}
	if start > len(eligible) {
		start = len(eligible)
	}
	size := extsync.PageSize(max(i.opt.PageSize, opt.Size))
	end := start + size
	if end > len(eligible) {
		end = len(eligible)
	}

	batch := extsync.Batch{Records: eligible[start:end]}
	if end < len(eligible) {
		batch.Page = strconv.Itoa(end)
	}
	if n := len(batch.Records); n > 0 {
		if at := batch.Records[n-1].UpdatedAt; !at.IsZero() {
			batch.Cursor = at.UTC().Format(time.RFC3339Nano)
		}
	}
	return batch, nil
}

// eligible drops records the cursor has already covered.
func (i *Importer) eligible(opt extsync.Options) []extsync.Record {
	if opt.Full || opt.Cursor == "" {
		return i.records
	}
	since, ok := extsync.ParseTime(opt.Cursor)
	if !ok {
		return i.records
	}
	out := make([]extsync.Record, 0, len(i.records))
	for _, r := range i.records {
		if r.UpdatedAt.IsZero() || !r.UpdatedAt.Before(since) {
			out = append(out, r)
		}
	}
	return out
}

func (i *Importer) load() error {
	if i.loaded {
		return nil
	}
	raw, err := os.ReadFile(i.opt.Path)
	if err != nil {
		return core.Invalid("reading import file %q: %v", i.opt.Path, err)
	}
	format := strings.ToLower(strings.TrimSpace(i.opt.Format))
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(filepath.Ext(i.opt.Path)), ".")
	}
	var records []extsync.Record
	switch format {
	case "csv", "tsv":
		records, err = readCSV(raw, format)
	case "json", "ndjson", "jsonl":
		records, err = readJSON(raw)
	default:
		return core.Invalid("import file %q is neither csv nor json; name it .csv or .json, or set the format",
			i.opt.Path)
	}
	if err != nil {
		return err
	}
	for idx := range records {
		if at, ok := extsync.Lookup(records[idx].Fields, i.opt.UpdatedField); ok {
			if parsed, ok := extsync.ParseTime(at); ok {
				records[idx].UpdatedAt = parsed
			}
		}
	}
	sort.SliceStable(records, func(a, b int) bool {
		return records[a].UpdatedAt.Before(records[b].UpdatedAt)
	})
	i.records = records
	i.loaded = true
	return nil
}

func readCSV(raw []byte, format string) ([]extsync.Record, error) {
	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	if format == "tsv" {
		r.Comma = '\t'
	}
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return nil, core.Invalid("import file is empty; a csv import needs a header row")
	}
	if err != nil {
		return nil, core.Invalid("reading csv header: %v", err)
	}
	for idx := range header {
		header[idx] = strings.TrimSpace(header[idx])
	}

	var out []extsync.Record
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, core.Invalid("reading csv row %d: %v", len(out)+2, err)
		}
		fields := make(map[string]any, len(header))
		for idx, name := range header {
			if name == "" || idx >= len(row) {
				continue
			}
			fields[name] = row[idx]
		}
		out = append(out, extsync.Record{Fields: fields})
	}
	return out, nil
}

func readJSON(raw []byte) ([]extsync.Record, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, core.Invalid("import file is empty")
	}
	if trimmed[0] == '[' {
		var items []map[string]any
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, core.Invalid("parsing json array: %v", err)
		}
		return wrap(items), nil
	}
	if trimmed[0] == '{' {
		var doc map[string]any
		if err := json.Unmarshal(trimmed, &doc); err == nil {
			if items, ok := arrayField(doc); ok {
				return wrap(items), nil
			}
			return wrap([]map[string]any{doc}), nil
		}
	}
	return readNDJSON(trimmed)
}

// arrayField finds the single list of objects a wrapper document carries.
func arrayField(doc map[string]any) ([]map[string]any, bool) {
	for _, key := range []string{"records", "items", "issues", "results", "data"} {
		list, ok := doc[key].([]any)
		if !ok {
			continue
		}
		out := make([]map[string]any, 0, len(list))
		for _, item := range list {
			obj, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			out = append(out, obj)
		}
		return out, true
	}
	return nil, false
}

func readNDJSON(raw []byte) ([]extsync.Record, error) {
	var items []map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	for {
		var obj map[string]any
		err := dec.Decode(&obj)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, core.Invalid("parsing json: %v", err)
		}
		items = append(items, obj)
	}
	if len(items) == 0 {
		return nil, core.Invalid("import file holds no json records")
	}
	return wrap(items), nil
}

func wrap(items []map[string]any) []extsync.Record {
	out := make([]extsync.Record, 0, len(items))
	for _, item := range items {
		out = append(out, extsync.Record{Fields: item})
	}
	return out
}
