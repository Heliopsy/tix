// SPDX-License-Identifier: AGPL-3.0-or-later

// Package taskio turns a foreign file of tasks into normalised draft rows
// under a user-editable column and value mapping.
package taskio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// Format is the encoding of an import file.
type Format string

// Formats an import file can be read in.
const (
	FormatCSV    Format = "csv"
	FormatJSON   Format = "json"
	FormatNDJSON Format = "ndjson"
)

// ParseFormat reads a format name, empty meaning detect it from the file.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return "", nil
	case "csv", "tsv":
		return FormatCSV, nil
	case "json":
		return FormatJSON, nil
	case "ndjson", "jsonl":
		return FormatNDJSON, nil
	}
	return "", core.Invalid("format %q is not one of csv, json or ndjson", s)
}

// DetectFormat names a file's format from its extension, else from its first bytes.
func DetectFormat(name string, head []byte) Format {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv", ".tsv", ".txt":
		return FormatCSV
	case ".json":
		return FormatJSON
	case ".ndjson", ".jsonl":
		return FormatNDJSON
	}
	head = bytes.TrimLeft(bytes.TrimPrefix(head, bom), " \t\r\n")
	switch {
	case len(head) == 0:
		return FormatCSV
	case head[0] == '[':
		return FormatJSON
	case head[0] == '{':
		return sniffObject(head)
	}
	return FormatCSV
}

func sniffObject(head []byte) Format {
	dec := json.NewDecoder(bytes.NewReader(head))
	var first map[string]json.RawMessage
	if err := dec.Decode(&first); err != nil {
		return FormatJSON
	}
	rest := bytes.TrimLeft(head[dec.InputOffset():], " \t\r\n")
	if len(rest) > 0 && rest[0] == '{' {
		return FormatNDJSON
	}
	for _, key := range wrapperKeys {
		if _, ok := first[key]; ok {
			return FormatJSON
		}
	}
	return FormatNDJSON
}

var wrapperKeys = []string{"tasks", "issues", "cards"}

var bom = []byte{0xEF, 0xBB, 0xBF}

// Limits bounds what an import reads, so a hostile or mistaken file fails early.
type Limits struct {
	MaxBytes   int64
	MaxRows    int
	MaxColumns int
	MaxCell    int
}

// DefaultLimits are the limits applied where a field of Limits is zero.
var DefaultLimits = Limits{MaxBytes: 64 << 20, MaxRows: 50000, MaxColumns: 500, MaxCell: 1 << 20}

func (l Limits) withDefaults() Limits {
	if l.MaxBytes <= 0 {
		l.MaxBytes = DefaultLimits.MaxBytes
	}
	if l.MaxRows <= 0 {
		l.MaxRows = DefaultLimits.MaxRows
	}
	if l.MaxColumns <= 0 {
		l.MaxColumns = DefaultLimits.MaxColumns
	}
	if l.MaxCell <= 0 {
		l.MaxCell = DefaultLimits.MaxCell
	}
	return l
}

func (l Limits) rowsError() error {
	return core.Invalid("the file has more than %d rows; split it and import the parts", l.MaxRows)
}

func (l Limits) columnsError() error {
	return core.Invalid("the file has more than %d columns; remove the ones you do not need and import again", l.MaxColumns)
}

func (l Limits) cellError(line int, header string) error {
	return core.Invalid("row %d: the %q cell is longer than the %s cell limit", line, header, byteSize(int64(l.MaxCell)))
}

func (l Limits) bytesError() error {
	return core.Invalid("the file is larger than %s; split it and import the parts", byteSize(l.MaxBytes))
}

func byteSize(n int64) string {
	switch {
	case n >= 1<<20 && n%(1<<20) == 0:
		return fmt.Sprintf("%d MiB", n>>20)
	case n >= 1<<10 && n%(1<<10) == 0:
		return fmt.Sprintf("%d KiB", n>>10)
	}
	return fmt.Sprintf("%d bytes", n)
}

type cappedReader struct {
	r    io.Reader
	left int64
	err  error
}

func newCappedReader(r io.Reader, l Limits) *cappedReader {
	return &cappedReader{r: r, left: l.MaxBytes, err: l.bytesError()}
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.left < 0 {
		return 0, c.err
	}
	if int64(len(p)) > c.left+1 {
		p = p[:c.left+1]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	if c.left < 0 {
		return n, c.err
	}
	return n, err
}

func (c *cappedReader) over() bool { return c.left < 0 }
