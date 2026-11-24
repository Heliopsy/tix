// SPDX-License-Identifier: AGPL-3.0-or-later

package ical

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// MaxObjectBytes bounds an iCalendar object Parse reads.
const MaxObjectBytes = 256 << 10

// Property is one content line: its name, its parameters and its raw value,
// still escaped.
type Property struct {
	Name   string
	Params map[string][]string
	Value  string
}

// Param returns the first value of a parameter, by name in any case.
func (p Property) Param(name string) string {
	if v := p.Params[strings.ToUpper(name)]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// Component is a BEGIN and END pair with the properties and components
// between them.
type Component struct {
	Name       string
	Properties []Property
	Children   []*Component
}

// Props returns every property of a name, in any case.
func (c *Component) Props(name string) []Property {
	var out []Property
	for _, p := range c.Properties {
		if strings.EqualFold(p.Name, name) {
			out = append(out, p)
		}
	}
	return out
}

// Prop returns the first property of a name.
func (c *Component) Prop(name string) (Property, bool) {
	for _, p := range c.Properties {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Property{}, false
}

// Components returns the child components of a name, in any case.
func (c *Component) Components(name string) []*Component {
	var out []*Component
	for _, k := range c.Children {
		if strings.EqualFold(k.Name, name) {
			out = append(out, k)
		}
	}
	return out
}

// Parse reads one iCalendar object: content lines unfolded, parameters split
// and components nested. Values are left escaped; Unescape reads a TEXT one.
func Parse(data []byte) (*Component, error) {
	if len(data) > MaxObjectBytes {
		return nil, core.Invalid("an iCalendar object is at most %d bytes", MaxObjectBytes)
	}
	var (
		root  *Component
		stack []*Component
	)
	for _, line := range unfold(string(data)) {
		if line == "" {
			continue
		}
		p, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		switch strings.ToUpper(p.Name) {
		case "BEGIN":
			c := &Component{Name: strings.ToUpper(p.Value)}
			if len(stack) == 0 {
				if root != nil {
					return nil, core.Invalid("an iCalendar object holds one top-level component")
				}
				root = c
			} else {
				top := stack[len(stack)-1]
				top.Children = append(top.Children, c)
			}
			stack = append(stack, c)
		case "END":
			if len(stack) == 0 || !strings.EqualFold(stack[len(stack)-1].Name, p.Value) {
				return nil, core.Invalid("END:%s does not close the open component", p.Value)
			}
			stack = stack[:len(stack)-1]
		default:
			if len(stack) == 0 {
				return nil, core.Invalid("a property outside any component")
			}
			top := stack[len(stack)-1]
			top.Properties = append(top.Properties, p)
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, core.Invalid("an iCalendar object is one complete component")
	}
	return root, nil
}

// unfold joins continuation lines, which start with a space or a tab, onto
// the line before.
func unfold(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if len(out) > 0 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			out[len(out)-1] += line[1:]
			continue
		}
		out = append(out, strings.TrimSuffix(line, "\r"))
	}
	return out
}

// parseLine splits name;param=value;param="quoted":value.
func parseLine(line string) (Property, error) {
	p := Property{}
	i := strings.IndexAny(line, ";:")
	if i <= 0 {
		return p, core.Invalid("a content line has no name")
	}
	p.Name = strings.ToUpper(line[:i])
	rest := line[i:]
	for strings.HasPrefix(rest, ";") {
		rest = rest[1:]
		eq := strings.IndexByte(rest, '=')
		if eq <= 0 {
			return p, core.Invalid("a parameter of %s has no value", p.Name)
		}
		name := strings.ToUpper(rest[:eq])
		rest = rest[eq+1:]
		for {
			var v string
			if strings.HasPrefix(rest, `"`) {
				end := strings.IndexByte(rest[1:], '"')
				if end < 0 {
					return p, core.Invalid("an unclosed quote in a parameter of %s", p.Name)
				}
				v, rest = rest[1:end+1], rest[end+2:]
			} else {
				end := strings.IndexAny(rest, ",;:")
				if end < 0 {
					return p, core.Invalid("a content line of %s has no value", p.Name)
				}
				v, rest = rest[:end], rest[end:]
			}
			if p.Params == nil {
				p.Params = map[string][]string{}
			}
			p.Params[name] = append(p.Params[name], v)
			if !strings.HasPrefix(rest, ",") {
				break
			}
			rest = rest[1:]
		}
	}
	if !strings.HasPrefix(rest, ":") {
		return p, core.Invalid("a content line of %s has no value", p.Name)
	}
	p.Value = rest[1:]
	return p, nil
}

// Unescape reads a TEXT value: the escapes Text writes, undone.
func Unescape(v string) string {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		if v[i] != '\\' || i+1 == len(v) {
			b.WriteByte(v[i])
			continue
		}
		i++
		switch v[i] {
		case 'n', 'N':
			b.WriteByte('\n')
		default:
			b.WriteByte(v[i])
		}
	}
	return b.String()
}

// SplitText splits a multi-valued TEXT value, such as CATEGORIES, on the
// commas that are not escaped, and unescapes each part.
func SplitText(v string) []string {
	var (
		out  []string
		part strings.Builder
	)
	for i := 0; i < len(v); i++ {
		switch {
		case v[i] == '\\' && i+1 < len(v):
			part.WriteByte(v[i])
			part.WriteByte(v[i+1])
			i++
		case v[i] == ',':
			out = append(out, Unescape(part.String()))
			part.Reset()
		default:
			part.WriteByte(v[i])
		}
	}
	return append(out, Unescape(part.String()))
}
