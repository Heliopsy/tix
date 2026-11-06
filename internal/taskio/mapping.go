// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"bytes"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/heliopsy/tix/internal/core"
)

// Mode is how an import treats a row whose task already exists.
type Mode string

// Import modes.
const (
	ModeCreate Mode = "create"
	ModeUpsert Mode = "upsert"
	ModeSkip   Mode = "skip"
)

// UnknownPolicy is what happens to a value that has no mapping and is not valid as is.
type UnknownPolicy string

// Unknown value policies.
const (
	PolicyDefault UnknownPolicy = "default"
	PolicySkipRow UnknownPolicy = "skip_row"
	PolicyError   UnknownPolicy = "error"
	PolicyKeep    UnknownPolicy = "keep"
	PolicyIgnore  UnknownPolicy = "ignore"
)

// MappingVersion is the mapping file version this package reads and writes.
const MappingVersion = 1

// DefaultSource namespaces external ids when neither the mapping nor the file names a source.
const DefaultSource = "import"

// Mapping is the user-editable description of how a file becomes tasks.
type Mapping struct {
	Version       int                          `yaml:"version" json:"version"`
	Source        string                       `yaml:"source,omitempty" json:"source,omitempty"`
	Format        string                       `yaml:"format,omitempty" json:"format,omitempty"`
	Delimiter     string                       `yaml:"delimiter,omitempty" json:"delimiter,omitempty"`
	Mode          Mode                         `yaml:"mode,omitempty" json:"mode,omitempty"`
	DateFormat    string                       `yaml:"date_format,omitempty" json:"date_format,omitempty"`
	Columns       map[string]Target            `yaml:"columns,omitempty" json:"columns,omitempty"`
	Values        map[Target]map[string]string `yaml:"values,omitempty" json:"values,omitempty"`
	Unknown       map[Target]UnknownPolicy     `yaml:"unknown,omitempty" json:"unknown,omitempty"`
	Defaults      map[Target]string            `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	TagsSeparator string                       `yaml:"tags_separator,omitempty" json:"tags_separator,omitempty"`
	Strict        bool                         `yaml:"strict,omitempty" json:"strict,omitempty"`
}

var policies = map[Target][]UnknownPolicy{
	TargetStatus:   {PolicyDefault, PolicySkipRow, PolicyError, PolicyKeep},
	TargetAssignee: {PolicyIgnore, PolicySkipRow, PolicyError, PolicyKeep},
	TargetPriority: {PolicyDefault, PolicySkipRow, PolicyError, PolicyKeep},
}

var defaultPolicy = map[Target]UnknownPolicy{
	TargetStatus:   PolicyDefault,
	TargetAssignee: PolicyIgnore,
	TargetPriority: PolicyDefault,
}

// ParseMapping reads and validates a YAML or JSON mapping file.
func ParseMapping(raw []byte) (*Mapping, error) {
	var m Mapping
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := yaml.Load(raw, &m, yaml.WithKnownFields()); err != nil {
			return nil, core.Invalid("reading the mapping: %v", err)
		}
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks every target, mode, policy and format the mapping names, filling the version.
func (m *Mapping) Validate() error {
	switch m.Version {
	case 0:
		m.Version = MappingVersion
	case MappingVersion:
	default:
		return core.Invalid("mapping version %d is not supported; use version: 1", m.Version)
	}
	switch m.Mode {
	case "", ModeCreate, ModeUpsert, ModeSkip:
	default:
		return core.Invalid("mode %q is not one of create, upsert or skip", m.Mode)
	}
	if _, err := ParseFormat(m.Format); err != nil {
		return err
	}
	if _, err := m.delimiter(); err != nil {
		return err
	}
	if _, err := compileDateFormat(m.DateFormat); err != nil {
		return err
	}
	for h, t := range m.Columns {
		if !t.Valid() {
			return core.Invalid("column %q maps to %q, which is not a target", h, t)
		}
	}
	for _, section := range []struct {
		name    string
		targets []Target
	}{{"values", keys(m.Values)}, {"defaults", keys(m.Defaults)}} {
		for _, t := range section.targets {
			if !t.Valid() || t == TargetIgnore || t == TargetAppendBody {
				return core.Invalid("%s names %q, which is not a target", section.name, t)
			}
		}
	}
	for t, p := range m.Unknown {
		allowed, ok := policies[t]
		if !ok {
			return core.Invalid("unknown names %q; only status, assignee and priority take a policy", t)
		}
		if !slices.Contains(policyNames(allowed), string(p)) {
			return core.Invalid("the %s policy %q is not one of %s", t, p, strings.Join(policyNames(allowed), ", "))
		}
	}
	return nil
}

func keys[V any](m map[Target]V) []Target {
	out := make([]Target, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func policyNames(ps []UnknownPolicy) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = string(p)
	}
	return out
}

func (m *Mapping) delimiter() (rune, error) {
	switch m.Delimiter {
	case "":
		return 0, nil
	case "tab", `\t`:
		return '\t', nil
	}
	r := []rune(m.Delimiter)
	if len(r) != 1 || r[0] == '"' || r[0] == '\n' || r[0] == '\r' {
		return 0, core.Invalid("delimiter %q is not a single character", m.Delimiter)
	}
	return r[0], nil
}

// Marshal writes the mapping as YAML with a stable key order.
func (m *Mapping) Marshal() ([]byte, error) {
	out := *m
	if out.Version == 0 {
		out.Version = MappingVersion
	}
	raw, err := yaml.Dump(&out, yaml.WithIndent(2), yaml.WithLineWidth(-1))
	if err != nil {
		return nil, core.Internal("writing the mapping: %v", err)
	}
	return raw, nil
}

// ReadOptions turns the mapping's format and delimiter into options for Open.
func (m *Mapping) ReadOptions(limits Limits) (ReadOptions, error) {
	format, err := ParseFormat(m.Format)
	if err != nil {
		return ReadOptions{}, err
	}
	delim, err := m.delimiter()
	if err != nil {
		return ReadOptions{}, err
	}
	return ReadOptions{Format: format, Delimiter: delim, Limits: limits}, nil
}

// SourceName is the namespace for external ids: the mapping's source, else the detected one, else "import".
func (m *Mapping) SourceName(detected string) string {
	return firstNonEmpty(m.Source, detected, DefaultSource)
}

// EffectiveMode is the mapping's mode, upsert when unset.
func (m *Mapping) EffectiveMode() Mode {
	if m.Mode == "" {
		return ModeUpsert
	}
	return m.Mode
}

// Policy is the unknown-value policy for a target, with its default when unset.
func (m *Mapping) Policy(t Target) UnknownPolicy {
	if p, ok := m.Unknown[t]; ok && p != "" {
		return p
	}
	if p, ok := defaultPolicy[t]; ok {
		return p
	}
	return PolicyKeep
}

// Resolve settles each header's target: explicit columns first, then the suggestion unless strict.
func (m *Mapping) Resolve(headers []string, source string, fields []FieldInfo) map[string]Target {
	out := make(map[string]Target, len(headers))
	taken := map[Target]bool{}
	var open []string
	for _, h := range headers {
		t, ok := m.Columns[h]
		if !ok && BaseHeader(h) != h {
			t, ok = m.Columns[BaseHeader(h)]
		}
		if ok {
			out[h] = t
			taken[t] = true
			continue
		}
		open = append(open, h)
	}
	suggested := Suggest(headers, source, fields)
	for _, h := range open {
		t := suggested[h]
		switch {
		case m.Strict:
			t = TargetIgnore
		case taken[t] && !t.Multi():
			t = fallback(h)
		}
		out[h] = t
	}
	return out
}
