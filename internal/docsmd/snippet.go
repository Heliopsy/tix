// SPDX-License-Identifier: AGPL-3.0-or-later

package docsmd

import (
	"fmt"
	"strings"
)

// Snippet is one fenced block a page says was copied from a region of a file,
// by an `<!-- example: PATH#REGION -->` comment above the fence, with
// nothing but blank lines between them.
type Snippet struct {
	// Path is the source file, relative to the module root.
	Path string
	// Region names the marked region of that file.
	Region string
	// Lang is the fence's info string, such as go or python.
	Lang string
	// Body is the code between the fences.
	Body string
	// Line is the 1-based line of the comment, for a failure to point at.
	Line int
}

const snippetMarker = "<!-- example: "

// Snippets returns every marked fenced block of md. A marker that is not
// followed by a fence is an error rather than a snippet nobody compares.
func Snippets(md string) ([]Snippet, error) {
	lines := strings.Split(md, "\n")
	var out []Snippet
	for i := 0; i < len(lines); i++ {
		ref, ok := strings.CutPrefix(strings.TrimSpace(lines[i]), snippetMarker)
		if !ok {
			continue
		}
		ref, ok = strings.CutSuffix(ref, " -->")
		path, region, hasRegion := strings.Cut(ref, "#")
		if !ok || !hasRegion || path == "" || region == "" {
			return nil, fmt.Errorf("line %d: marker %q is not <!-- example: PATH#REGION -->", i+1, lines[i])
		}
		fence := i + 1
		for fence < len(lines) && strings.TrimSpace(lines[fence]) == "" {
			fence++
		}
		if fence >= len(lines) || !strings.HasPrefix(lines[fence], "```") {
			return nil, fmt.Errorf("line %d: marker for %s#%s is not followed by a fence", i+1, path, region)
		}
		end := fence + 1
		for end < len(lines) && lines[end] != "```" {
			end++
		}
		if end == len(lines) {
			return nil, fmt.Errorf("line %d: the fence after %s#%s is never closed", fence+1, path, region)
		}
		out = append(out, Snippet{
			Path:   path,
			Region: region,
			Lang:   strings.TrimPrefix(lines[fence], "```"),
			Body:   strings.Join(lines[fence+1:end], "\n"),
			Line:   i + 1,
		})
		i = end
	}
	return out, nil
}

// Regions returns the regions of a source file, by name, in the order they
// open. A region opens on a line reading `# region NAME` or `// region NAME`
// and closes on the next `# endregion` or `// endregion`. Each body is
// dedented and its leading tabs are written as four spaces, which is how the
// Markdown under docs/ indents code.
func Regions(src string) (map[string]string, []string, error) {
	bodies := map[string]string{}
	var order []string
	var name string
	var body []string
	for i, line := range strings.Split(src, "\n") {
		marker := strings.TrimSpace(line)
		marker = strings.TrimPrefix(strings.TrimPrefix(marker, "//"), "#")
		marker = strings.TrimSpace(marker)
		switch {
		case strings.HasPrefix(marker, "region "):
			if name != "" {
				return nil, nil, fmt.Errorf("line %d: region opens inside region %q", i+1, name)
			}
			name = strings.TrimSpace(strings.TrimPrefix(marker, "region "))
			if _, dup := bodies[name]; dup {
				return nil, nil, fmt.Errorf("line %d: region %q is marked twice", i+1, name)
			}
			body = nil
		case marker == "endregion":
			if name == "" {
				return nil, nil, fmt.Errorf("line %d: endregion with no region open", i+1)
			}
			bodies[name] = dedent(body)
			order = append(order, name)
			name = ""
		case name != "":
			body = append(body, line)
		}
	}
	if name != "" {
		return nil, nil, fmt.Errorf("region %q is never closed", name)
	}
	return bodies, order, nil
}

// dedent removes the indentation every non-blank line shares, expands the
// remaining leading tabs and trims blank lines from both ends.
func dedent(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	common := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if common < 0 || n < common {
			common = n
		}
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		l = l[common:]
		indent := len(l) - len(strings.TrimLeft(l, "\t"))
		out[i] = strings.Repeat("    ", indent) + l[indent:]
	}
	return strings.Join(out, "\n")
}
