package sync

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thereisnotime/tix/internal/core"
	"gopkg.in/yaml.v3"
)

// MappingVersion is the mapping file format this build understands.
const MappingVersion = 1

// Built-in mapping targets, the tix fields an external field may be assigned to.
const (
	TargetTitle    = "title"
	TargetBody     = "body"
	TargetPriority = "priority"
	TargetTags     = "tags"
	TargetAssignee = "assignee"
	TargetDueAt    = "due_at"
	TargetParent   = "parent"
)

// builtinTargets is the accepted right-hand side of a fields entry.
var builtinTargets = map[string]bool{
	TargetTitle: true, TargetBody: true, TargetPriority: true,
	TargetTags: true, TargetAssignee: true, TargetDueAt: true, TargetParent: true,
}

// priorityNames maps the words a mapping may use onto tix priorities.
var priorityNames = map[string]core.Priority{
	"highest": core.PriorityHighest, "high": core.PriorityHigh,
	"normal": core.PriorityNormal, "medium": core.PriorityNormal,
	"low": core.PriorityLow, "lowest": core.PriorityLowest,
}

// keySanitizer reduces an external field path to a custom field key.
var keySanitizer = regexp.MustCompile(`[^a-z0-9_]+`)

// Identity names the external fields that make a record re-importable.
type Identity struct {
	ID        string `yaml:"id" json:"id"`
	URL       string `yaml:"url,omitempty" json:"url,omitempty"`
	Version   string `yaml:"version,omitempty" json:"version,omitempty"`
	UpdatedAt string `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// CustomField assigns an external field to a tix custom field definition.
type CustomField struct {
	Key   string         `yaml:"key" json:"key"`
	Label string         `yaml:"label,omitempty" json:"label,omitempty"`
	Type  core.FieldType `yaml:"type,omitempty" json:"type,omitempty"`
}

// TypeRule is what an external issue type contributes to an imported task.
type TypeRule struct {
	Tags     []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Priority string   `yaml:"priority,omitempty" json:"priority,omitempty"`
	Status   string   `yaml:"status,omitempty" json:"status,omitempty"`
}

// TypeMapping maps external issue types onto tix constructs.
type TypeMapping struct {
	Field string              `yaml:"field,omitempty" json:"field,omitempty"`
	Map   map[string]TypeRule `yaml:"map,omitempty" json:"map,omitempty"`
}

// Unmapped controls what happens to external fields the mapping does not name.
type Unmapped struct {
	Preserve *bool  `yaml:"preserve,omitempty" json:"preserve,omitempty"`
	Prefix   string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
}

// Lossy declares a source concept the mapping deliberately flattens.
type Lossy struct {
	Field  string `yaml:"field" json:"field"`
	Reason string `yaml:"reason,omitempty" json:"reason,omitempty"`
}

// Mapping is the declarative translation from an external system to tix.
type Mapping struct {
	Version       int                    `yaml:"version" json:"version"`
	System        string                 `yaml:"system,omitempty" json:"system,omitempty"`
	Project       string                 `yaml:"project" json:"project"`
	Workflow      string                 `yaml:"workflow,omitempty" json:"workflow,omitempty"`
	Identity      Identity               `yaml:"identity" json:"identity"`
	Fields        map[string]string      `yaml:"fields,omitempty" json:"fields,omitempty"`
	StatusField   string                 `yaml:"status_field,omitempty" json:"status_field,omitempty"`
	Statuses      map[string]string      `yaml:"statuses,omitempty" json:"statuses,omitempty"`
	DefaultStatus string                 `yaml:"default_status,omitempty" json:"default_status,omitempty"`
	Priorities    map[string]string      `yaml:"priorities,omitempty" json:"priorities,omitempty"`
	Types         TypeMapping            `yaml:"types,omitempty" json:"types,omitempty"`
	Custom        map[string]CustomField `yaml:"custom,omitempty" json:"custom,omitempty"`
	Unmapped      Unmapped               `yaml:"unmapped,omitempty" json:"unmapped,omitempty"`
	Lossy         []Lossy                `yaml:"lossy,omitempty" json:"lossy,omitempty"`
}

// LoadMapping reads a mapping file from disk.
func LoadMapping(path string) (*Mapping, error) {
	if strings.TrimSpace(path) == "" {
		return nil, core.Invalid("a mapping file is required; none is configured for this source")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, core.Invalid("reading mapping file %q: %v", path, err)
	}
	return ParseMapping(raw)
}

// ParseMapping reads a mapping from YAML.
func ParseMapping(raw []byte) (*Mapping, error) {
	var m Mapping
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, core.Invalid("parsing mapping: %v", err)
	}
	if m.Version == 0 {
		m.Version = MappingVersion
	}
	if m.Version != MappingVersion {
		return nil, core.Invalid("mapping version %d is not supported; this build reads version %d",
			m.Version, MappingVersion)
	}
	if err := m.check(); err != nil {
		return nil, err
	}
	return &m, nil
}

// check validates everything a mapping can be judged on without a workflow.
func (m *Mapping) check() error {
	if strings.TrimSpace(m.Project) == "" {
		return core.Invalid("mapping must name the tix project to import into")
	}
	if strings.TrimSpace(m.Identity.ID) == "" {
		return core.Invalid("mapping must name the external field holding the external identifier")
	}
	for ext, target := range m.Fields {
		if !builtinTargets[target] {
			return core.Invalid("mapping entry fields.%s targets unknown tix field %q", ext, target)
		}
	}
	for ext, cf := range m.Custom {
		if strings.TrimSpace(cf.Key) == "" {
			return core.Invalid("mapping entry custom.%s must name a tix custom field key", ext)
		}
		if cf.Type != "" && !cf.Type.Valid() {
			return core.Invalid("mapping entry custom.%s uses unknown field type %q", ext, cf.Type)
		}
	}
	for name, p := range m.Priorities {
		if _, err := parsePriority(p); err != nil {
			return core.Invalid("mapping entry priorities.%s: %v", name, err)
		}
	}
	if len(m.Statuses) > 0 && strings.TrimSpace(m.StatusField) == "" {
		return core.Invalid("mapping defines statuses but no status_field to read them from")
	}
	return nil
}

// ValidateAgainst refuses a mapping that names a state the workflow does not
// define, before any record is read.
func (m *Mapping) ValidateAgainst(def core.WorkflowDefinition) error {
	for _, name := range sortedKeys(m.Statuses) {
		if state := m.Statuses[name]; !def.HasState(state) {
			return core.Invalid("mapping entry statuses.%s names workflow state %q, which the workflow does not define",
				name, state)
		}
	}
	if m.DefaultStatus != "" && !def.HasState(m.DefaultStatus) {
		return core.Invalid("mapping entry default_status names workflow state %q, which the workflow does not define",
			m.DefaultStatus)
	}
	for _, name := range sortedKeys(m.Types.Map) {
		rule := m.Types.Map[name]
		if rule.Status != "" && !def.HasState(rule.Status) {
			return core.Invalid("mapping entry types.map.%s names workflow state %q, which the workflow does not define",
				name, rule.Status)
		}
	}
	return nil
}

// RequiredPaths lists the external fields a record must carry to be importable.
func (m *Mapping) RequiredPaths() []string {
	out := []string{m.Identity.ID}
	for ext, target := range m.Fields {
		if target == TargetTitle {
			out = append(out, ext)
		}
	}
	sort.Strings(out)
	return out
}

// preserveUnmapped reports whether unnamed external fields become custom fields.
func (m *Mapping) preserveUnmapped() bool {
	return m.Unmapped.Preserve == nil || *m.Unmapped.Preserve
}

func (m *Mapping) prefix() string {
	if m.Unmapped.Prefix != "" {
		return m.Unmapped.Prefix
	}
	return "ext_"
}

// Mapped is one external record translated into tix terms.
type Mapped struct {
	ExternalID       string
	ExternalURL      string
	ExternalVersion  string
	ExternalParentID string
	UpdatedAt        time.Time

	Title        string
	Body         string
	Status       string
	Priority     core.Priority
	Tags         []string
	Assignee     string
	DueAt        *time.Time
	CustomFields map[string]any

	Warnings []string
}

// Apply translates one external record through the mapping. A record that
// cannot be translated returns an invalid error naming what went wrong, which
// the caller reports as a skip rather than aborting the run.
func (m *Mapping) Apply(r Record) (Mapped, error) {
	out := Mapped{CustomFields: map[string]any{}, UpdatedAt: r.UpdatedAt}
	consumed := map[string]bool{}

	id := strings.TrimSpace(Text(value(r, m.Identity.ID, consumed)))
	if id == "" {
		return out, core.Invalid("record carries no value at identity field %q", m.Identity.ID)
	}
	out.ExternalID = id
	out.ExternalURL = strings.TrimSpace(Text(value(r, m.Identity.URL, consumed)))
	out.ExternalVersion = strings.TrimSpace(Text(value(r, m.Identity.Version, consumed)))
	if raw := value(r, m.Identity.UpdatedAt, consumed); raw != nil {
		if parsed, ok := ParseTime(raw); ok {
			out.UpdatedAt = parsed
		}
	}

	if err := m.applyFields(r, &out, consumed); err != nil {
		return out, err
	}
	if err := m.applyStatus(r, &out, consumed); err != nil {
		return out, err
	}
	m.applyTypes(r, &out, consumed)
	if err := m.applyCustom(r, &out, consumed); err != nil {
		return out, err
	}
	m.applyUnmapped(r, &out, consumed)
	m.applyLossy(r, &out)

	if strings.TrimSpace(out.Title) == "" {
		return out, core.Invalid("record %q maps to no title", id)
	}
	if out.ExternalVersion == "" && !out.UpdatedAt.IsZero() {
		out.ExternalVersion = out.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	return out, nil
}

func value(r Record, path string, consumed map[string]bool) any {
	if path == "" {
		return nil
	}
	consumed[path] = true
	v, ok := Lookup(r.Fields, path)
	if !ok {
		return nil
	}
	return v
}

func (m *Mapping) applyFields(r Record, out *Mapped, consumed map[string]bool) error {
	for _, ext := range sortedKeys(m.Fields) {
		raw := value(r, ext, consumed)
		if raw == nil {
			continue
		}
		switch m.Fields[ext] {
		case TargetTitle:
			out.Title = strings.TrimSpace(Text(raw))
		case TargetBody:
			out.Body = Text(raw)
		case TargetAssignee:
			out.Assignee = strings.TrimSpace(Text(raw))
		case TargetParent:
			out.ExternalParentID = strings.TrimSpace(Text(raw))
		case TargetTags:
			out.Tags = append(out.Tags, Strings(raw)...)
		case TargetDueAt:
			parsed, ok := ParseTime(raw)
			if !ok {
				return core.Invalid("field %q holds %q, which is not a due date", ext, Text(raw))
			}
			due := parsed
			out.DueAt = &due
		case TargetPriority:
			p, err := m.priorityFor(Text(raw))
			if err != nil {
				return core.Invalid("field %q: %v", ext, err)
			}
			out.Priority = p
		}
	}
	return nil
}

func (m *Mapping) applyStatus(r Record, out *Mapped, consumed map[string]bool) error {
	if m.StatusField == "" {
		out.Status = m.DefaultStatus
		return nil
	}
	raw := strings.TrimSpace(Text(value(r, m.StatusField, consumed)))
	if raw == "" {
		out.Status = m.DefaultStatus
		return nil
	}
	if state, ok := m.Statuses[raw]; ok {
		out.Status = state
		return nil
	}
	if m.DefaultStatus == "" {
		return core.Invalid("external status %q has no mapped tix state and the mapping defines no default_status", raw)
	}
	out.Status = m.DefaultStatus
	out.Warnings = append(out.Warnings,
		"external status "+strconv.Quote(raw)+" fell back to "+strconv.Quote(m.DefaultStatus))
	return nil
}

func (m *Mapping) applyTypes(r Record, out *Mapped, consumed map[string]bool) {
	if m.Types.Field == "" {
		return
	}
	name := strings.TrimSpace(Text(value(r, m.Types.Field, consumed)))
	rule, ok := m.Types.Map[name]
	if !ok {
		if name != "" {
			out.CustomFields[m.prefix()+"type"] = name
		}
		return
	}
	out.Tags = append(out.Tags, rule.Tags...)
	if rule.Status != "" {
		out.Status = rule.Status
	}
	if rule.Priority != "" {
		if p, err := parsePriority(rule.Priority); err == nil {
			out.Priority = p
		}
	}
}

func (m *Mapping) applyCustom(r Record, out *Mapped, consumed map[string]bool) error {
	for _, ext := range sortedKeys(m.Custom) {
		cf := m.Custom[ext]
		raw := value(r, ext, consumed)
		if raw == nil {
			continue
		}
		v, err := Coerce(cf.FieldType(), raw)
		if err != nil {
			return core.Invalid("field %q into custom field %q: %v", ext, cf.Key, err)
		}
		out.CustomFields[cf.Key] = v
	}
	return nil
}

func (m *Mapping) applyUnmapped(r Record, out *Mapped, consumed map[string]bool) {
	if !m.preserveUnmapped() {
		return
	}
	var kept []string
	for _, path := range sortedStrings(Paths(r.Fields)) {
		if consumed[path] || coveredByConsumed(path, consumed) {
			continue
		}
		v, ok := Lookup(r.Fields, path)
		if !ok || v == nil || Text(v) == "" {
			continue
		}
		out.CustomFields[m.prefix()+CustomKey(path)] = Text(v)
		kept = append(kept, path)
	}
	if len(kept) > 0 {
		out.Warnings = append(out.Warnings,
			"preserved unmapped fields in custom fields: "+strings.Join(kept, ", "))
	}
}

// coveredByConsumed reports whether a leaf sits under a path the mapping read
// whole, so reading fields.labels does not also preserve fields.labels.0.
func coveredByConsumed(path string, consumed map[string]bool) bool {
	for prefix := range consumed {
		if prefix != "" && strings.HasPrefix(path, prefix+".") {
			return true
		}
	}
	return false
}

func (m *Mapping) applyLossy(r Record, out *Mapped) {
	for _, l := range m.Lossy {
		if _, ok := Lookup(r.Fields, l.Field); !ok {
			continue
		}
		reason := l.Reason
		if reason == "" {
			reason = "has no tix equivalent and was flattened"
		}
		out.Warnings = append(out.Warnings,
			"lossy mapping for "+out.ExternalID+": "+l.Field+" "+reason)
	}
}

func (m *Mapping) priorityFor(raw string) (core.Priority, error) {
	name := strings.TrimSpace(raw)
	if mapped, ok := m.Priorities[name]; ok {
		return parsePriority(mapped)
	}
	return parsePriority(name)
}

// parsePriority reads a priority written as a name or as a number.
func parsePriority(raw string) (core.Priority, error) {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return 0, nil
	}
	if p, ok := priorityNames[name]; ok {
		return p, nil
	}
	n, err := strconv.Atoi(name)
	if err != nil {
		return 0, core.Invalid("priority %q is not a name or a number", raw)
	}
	p := core.Priority(n)
	if !p.Valid() {
		return 0, core.Invalid("priority %d is out of range", n)
	}
	return p, nil
}

// FieldType returns the custom field's declared type, defaulting to string.
func (c CustomField) FieldType() core.FieldType {
	if c.Type == "" {
		return core.FieldString
	}
	return c.Type
}

// CustomKey reduces an external field path to a usable custom field key.
func CustomKey(path string) string {
	key := keySanitizer.ReplaceAllString(strings.ToLower(path), "_")
	return strings.Trim(key, "_")
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedStrings(in []string) []string {
	sort.Strings(in)
	return in
}
