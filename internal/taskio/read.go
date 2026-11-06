// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// ReadOptions selects how Open reads a file.
type ReadOptions struct {
	Format    Format
	Delimiter rune
	Limits    Limits
}

// Row is one data record of an import file.
type Row struct {
	Line    int
	Values  map[string]string
	Headers []string
}

// Reader yields the rows of an import file in source order.
type Reader interface {
	Headers() []string
	Next() (Row, error)
	Source() string
}

const sniffSize = 64 << 10

// Open starts reading an import file, detecting its format and delimiter where unset.
func Open(r io.Reader, opts ReadOptions) (Reader, error) {
	limits := opts.Limits.withDefaults()
	capped := newCappedReader(r, limits)
	br := bufio.NewReaderSize(capped, sniffSize)
	head, _ := br.Peek(sniffSize)
	if capped.over() {
		return nil, capped.err
	}
	if bytes.HasPrefix(head, bom) {
		_, _ = br.Discard(len(bom))
		head = head[len(bom):]
	}
	format := opts.Format
	if format == "" {
		format = DetectFormat("", head)
	}
	switch format {
	case FormatCSV:
		delim := opts.Delimiter
		if delim == 0 {
			delim = DetectDelimiter(head)
		}
		return openCSV(br, capped, delim, limits)
	case FormatJSON:
		return openJSON(br, capped, limits)
	case FormatNDJSON:
		return openNDJSON(br, capped, limits)
	}
	return nil, core.Invalid("format %q is not one of csv, json or ndjson", format)
}

var delimiters = []rune{',', ';', '\t', '|'}

// DetectDelimiter picks the CSV delimiter that splits the header line most often.
func DetectDelimiter(head []byte) rune {
	head = bytes.TrimPrefix(head, bom)
	counts := map[rune]int{}
	quoted := false
	for _, c := range string(head) {
		if c == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if c == '\n' || c == '\r' {
			break
		}
		counts[c]++
	}
	best := ','
	for _, d := range delimiters {
		if counts[d] > counts[best] {
			best = d
		}
	}
	return best
}

var dupSuffix = regexp.MustCompile(` \((\d+)\)$`)

// BaseHeader strips the " (N)" a repeated header was given, so same-named columns can be joined.
func BaseHeader(h string) string {
	if m := dupSuffix.FindStringSubmatch(h); m != nil {
		if n, _ := strconv.Atoi(m[1]); n >= 2 {
			return strings.TrimSuffix(h, m[0])
		}
	}
	return h
}

func dedupeHeaders(raw []string) []string {
	out := make([]string, len(raw))
	seen := map[string]bool{}
	for i, h := range raw {
		h = strings.TrimSpace(h)
		if h == "" {
			h = "Column " + strconv.Itoa(i+1)
		}
		name := h
		for n := 2; seen[name]; n++ {
			name = h + " (" + strconv.Itoa(n) + ")"
		}
		seen[name] = true
		out[i] = name
	}
	return out
}

// UnguardCell undoes the quote a spreadsheet-safe export put before a formula-like cell.
func UnguardCell(s string) string {
	if len(s) > 1 && s[0] == '\'' {
		switch s[1] {
		case '=', '+', '-', '@', '\t', '\r':
			return s[1:]
		}
	}
	return s
}

type csvReader struct {
	r       *csv.Reader
	capped  *cappedReader
	headers []string
	limits  Limits
	rows    int
	source  string
}

func openCSV(br *bufio.Reader, capped *cappedReader, delim rune, limits Limits) (Reader, error) {
	r := csv.NewReader(br)
	r.Comma = delim
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	c := &csvReader{r: r, capped: capped, limits: limits}
	rec, err := r.Read()
	if errors.Is(err, io.EOF) {
		return nil, core.Invalid("the file is empty")
	}
	if err != nil {
		return nil, c.wrap(err)
	}
	if len(rec) > limits.MaxColumns {
		return nil, limits.columnsError()
	}
	for i := range rec {
		rec[i] = UnguardCell(rec[i])
	}
	c.headers = dedupeHeaders(rec)
	c.source = DetectSource(c.headers)
	return c, nil
}

func (c *csvReader) wrap(err error) error {
	if c.capped.over() {
		return c.capped.err
	}
	var domain *core.Error
	if errors.As(err, &domain) {
		return domain
	}
	return core.Invalid("reading CSV: %v", err)
}

func (c *csvReader) Headers() []string { return c.headers }

func (c *csvReader) Source() string { return c.source }

func (c *csvReader) Next() (Row, error) {
	for {
		rec, err := c.r.Read()
		if errors.Is(err, io.EOF) && !c.capped.over() {
			return Row{}, io.EOF
		}
		if err != nil {
			return Row{}, c.wrap(err)
		}
		if blank(rec) {
			continue
		}
		c.rows++
		if c.rows > c.limits.MaxRows {
			return Row{}, c.limits.rowsError()
		}
		line, _ := c.r.FieldPos(0)
		if len(rec) > c.limits.MaxColumns {
			return Row{}, c.limits.columnsError()
		}
		row := Row{Line: line, Values: make(map[string]string, len(c.headers)), Headers: c.headers}
		for i, h := range c.headers {
			if i >= len(rec) {
				break
			}
			if len(rec[i]) > c.limits.MaxCell {
				return Row{}, c.limits.cellError(line, h)
			}
			row.Values[h] = UnguardCell(rec[i])
		}
		return row, nil
	}
}

func blank(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}
