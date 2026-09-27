// SPDX-License-Identifier: AGPL-3.0-or-later

// Package docsmd reads the enumerable parts of the Markdown under docs/, so a
// test can assert one named table rather than grepping a whole page. A guard
// that searches the whole file passes on a key named in a paragraph, which is
// the drift these tests exist to catch.
package docsmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Row is one table row, cell by cell, with the pipes and padding removed.
type Row []string

// Root locates the module root by walking up from the working directory, so a
// test names a path from the repository root rather than counting how deep its
// own package sits.
func Root() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locating the working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %q", dir)
		}
		dir = parent
	}
}

// Read returns a file named relative to the module root.
func Read(rel string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	b, err := os.ReadFile(path) // #nosec G304 -- the path is a literal in a test
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", rel, err)
	}
	return string(b), nil
}

// Heading returns the first heading line at the given level whose text matches
// match, which reports the heading as written so a test can assert the count a
// heading names.
func Heading(md string, level int, match func(text string) bool) (string, error) {
	prefix := strings.Repeat("#", level) + " "
	for _, line := range strings.Split(md, "\n") {
		text, ok := strings.CutPrefix(strings.TrimSpace(line), prefix)
		if ok && match(text) {
			return text, nil
		}
	}
	return "", fmt.Errorf("no level %d heading matched", level)
}

// Section returns the body under the heading whose text is exactly text,
// stopping at the next heading of the same or a higher level. Scoping to a
// section is what lets a table assertion read the table it names and not a
// similar one further down the page.
func Section(md, heading string) (string, error) {
	level := len(heading) - len(strings.TrimLeft(heading, "#"))
	if level == 0 {
		return "", fmt.Errorf("%q is not a heading", heading)
	}
	lines := strings.Split(md, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("docs page has no heading %q", heading)
	}
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		if depth := len(trimmed) - len(strings.TrimLeft(trimmed, "#")); depth <= level {
			return strings.Join(lines[start:i], "\n"), nil
		}
	}
	return strings.Join(lines[start:], "\n"), nil
}

// Table returns the rows of the one table in md whose header cells are exactly
// header. Naming the header is how a test says which table it means: a page
// carrying two tables with a View column has two different headers.
func Table(md string, header ...string) ([]Row, error) {
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		if !equalCells(cells(line), header) {
			continue
		}
		if i+1 >= len(lines) || !isDivider(lines[i+1]) {
			return nil, fmt.Errorf("table %q is not followed by a divider row", strings.Join(header, " | "))
		}
		var rows []Row
		for _, body := range lines[i+2:] {
			row := cells(body)
			if row == nil {
				break
			}
			rows = append(rows, row)
		}
		if len(rows) == 0 {
			return nil, fmt.Errorf("table %q has no rows", strings.Join(header, " | "))
		}
		return rows, nil
	}
	return nil, fmt.Errorf("no table with the header %q", "| "+strings.Join(header, " | ")+" |")
}

// Codes returns the backticked spans of a cell, which is how every table in
// docs/ writes a key, a marker or a command.
func Codes(cell string) []string {
	var out []string
	for i, part := range strings.Split(cell, "`") {
		if i%2 == 1 && part != "" {
			out = append(out, part)
		}
	}
	return out
}

// Column returns one column of a set of rows, skipping rows too short to have
// it.
func Column(rows []Row, n int) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if n < len(r) {
			out = append(out, r[n])
		}
	}
	return out
}

// cells splits a Markdown table row, returning nil for a line that is not one.
func cells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") || len(trimmed) < 2 {
		return nil
	}
	parts := strings.Split(strings.Trim(trimmed, "|"), "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// isDivider reports whether a line is the dashed row under a table header.
func isDivider(line string) bool {
	c := cells(line)
	if c == nil {
		return false
	}
	for _, cell := range c {
		if strings.Trim(cell, ":- ") != "" || !strings.Contains(cell, "-") {
			return false
		}
	}
	return true
}

// equalCells reports whether two header rows name the same columns.
func equalCells(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if !strings.EqualFold(got[i], want[i]) {
			return false
		}
	}
	return true
}
