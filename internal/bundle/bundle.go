// Package bundle encodes and decodes shareable component bundles.
package bundle

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// RecordKind names the two record shapes a bundle document holds.
type RecordKind string

// Record kinds.
const (
	RecordHeader    RecordKind = "header"
	RecordComponent RecordKind = "component"
)

// Record is one line of a bundle document.
type Record struct {
	Record    RecordKind `json:"record"`
	Header    *Header    `json:"header,omitempty"`
	Component *Component `json:"component,omitempty"`
}

// Header is the opening record of every bundle.
type Header struct {
	Name       string    `json:"name,omitempty"`
	Version    int       `json:"version"`
	TixVersion string    `json:"tix_version"`
	ExportedAt time.Time `json:"exported_at"`
	// Tenant is decoded and then ignored. A bundle never chooses where it
	// lands; the importing caller's own tenant always does.
	Tenant string `json:"tenant,omitempty"`
}

// Component is one shareable piece of configuration.
type Component struct {
	Kind     core.ComponentKind `json:"kind"`
	Workflow *Workflow          `json:"workflow,omitempty"`
	Field    *Field             `json:"field,omitempty"`
	Tag      *Tag               `json:"tag,omitempty"`
	Project  *Project           `json:"project,omitempty"`
	Webhook  *Webhook           `json:"webhook,omitempty"`
}

// Workflow is a state machine, with its states, transitions and lease settings.
type Workflow struct {
	Key        string                  `json:"key"`
	Name       string                  `json:"name,omitempty"`
	Definition core.WorkflowDefinition `json:"definition"`
}

// Field is one custom field definition.
type Field struct {
	Key         string         `json:"key"`
	Label       string         `json:"label,omitempty"`
	Type        core.FieldType `json:"type"`
	Required    bool           `json:"required,omitempty"`
	EnumOptions []string       `json:"enum_options,omitempty"`
	Default     any            `json:"default,omitempty"`
	Indexed     bool           `json:"indexed,omitempty"`
	Position    int            `json:"position,omitempty"`
}

// Tag is one entry of a tag vocabulary.
type Tag struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// Project is a project template: the way of working, never the work.
type Project struct {
	Key         string            `json:"key"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Color       core.ProjectColor `json:"color,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Workflow    *Workflow         `json:"workflow,omitempty"`
	Fields      []Field           `json:"fields,omitempty"`
	Tags        []Tag             `json:"tags,omitempty"`
}

// Webhook is a delivery endpoint definition. A signing secret never travels in
// a bundle, so an imported endpoint stays inactive until one is supplied.
type Webhook struct {
	URL        string   `json:"url"`
	EventTypes []string `json:"event_types,omitempty"`
}

// Order returns a component kind's position in the bundle order, which is the
// order that puts a component before anything that can depend on it.
func Order(k core.ComponentKind) (int, bool) {
	for i, known := range core.ComponentKinds {
		if known == k {
			return i, true
		}
	}
	return 0, false
}

// Key returns the key the component is known by in its target tenant.
func (c Component) Key() string {
	switch c.Kind {
	case core.ComponentWorkflow:
		if c.Workflow != nil {
			return c.Workflow.Key
		}
	case core.ComponentFieldDef:
		if c.Field != nil {
			return c.Field.Key
		}
	case core.ComponentTag:
		if c.Tag != nil {
			return c.Tag.Name
		}
	case core.ComponentProject:
		if c.Project != nil {
			return c.Project.Key
		}
	case core.ComponentWebhook:
		if c.Webhook != nil {
			return c.Webhook.URL
		}
	}
	return ""
}

// HasPayload reports whether the component carries the payload its kind names.
func (c Component) HasPayload() bool {
	switch c.Kind {
	case core.ComponentWorkflow:
		return c.Workflow != nil
	case core.ComponentFieldDef:
		return c.Field != nil
	case core.ComponentTag:
		return c.Tag != nil
	case core.ComponentProject:
		return c.Project != nil
	case core.ComponentWebhook:
		return c.Webhook != nil
	default:
		return false
	}
}

// Validate checks one component in full, so a bundle can be refused before
// anything is written.
func (c Component) Validate() error {
	if !c.Kind.Valid() {
		return core.Invalid("component kind %q is not one this build can share", c.Kind)
	}
	if !c.HasPayload() {
		return core.Invalid("%s component carries no payload", c.Kind)
	}
	if strings.TrimSpace(c.Key()) == "" {
		return core.Invalid("%s component has no key", c.Kind)
	}
	switch c.Kind {
	case core.ComponentWorkflow:
		return c.Workflow.validate()
	case core.ComponentFieldDef:
		return c.Field.validate()
	case core.ComponentTag:
		return nil
	case core.ComponentProject:
		return c.Project.validate()
	default:
		return c.Webhook.validate()
	}
}

// validate checks a workflow's state machine.
func (w Workflow) validate() error {
	in := core.WorkflowInput{Key: w.Key, Name: w.Name, Definition: w.Definition}
	if err := in.Validate(); err != nil {
		return core.Invalid("workflow %q is not valid: %s", w.Key, message(err))
	}
	return nil
}

// validate checks a custom field definition.
func (f Field) validate() error {
	in := core.FieldDefInput{
		Key: f.Key, Label: f.Label, Type: f.Type,
		Required: f.Required, EnumOptions: f.EnumOptions,
		Default: f.Default, Indexed: f.Indexed, Position: f.Position,
	}
	if err := in.Validate(); err != nil {
		return core.Invalid("field definition %q is not valid: %s", f.Key, message(err))
	}
	return nil
}

// validate checks a project template and everything it carries.
func (p Project) validate() error {
	if err := core.ValidateProjectKey(p.Key); err != nil {
		return core.Invalid("project template %q is not valid: %s", p.Key, message(err))
	}
	if !p.Color.Valid() {
		return core.Invalid("project template %q carries colour %q, which is not in the palette", p.Key, p.Color)
	}
	if _, err := core.NormalizeProjectIcon(p.Icon); err != nil {
		return core.Invalid("project template %q carries an invalid icon: %s", p.Key, message(err))
	}
	if p.Workflow != nil {
		if err := p.Workflow.validate(); err != nil {
			return core.Invalid("project template %q carries an invalid workflow: %s", p.Key, message(err))
		}
	}
	for _, f := range p.Fields {
		if err := f.validate(); err != nil {
			return core.Invalid("project template %q carries an invalid field: %s", p.Key, message(err))
		}
	}
	for _, t := range p.Tags {
		if strings.TrimSpace(t.Name) == "" {
			return core.Invalid("project template %q carries a tag with no name", p.Key)
		}
	}
	return nil
}

// validate checks a webhook endpoint definition.
func (w Webhook) validate() error {
	in := core.WebhookInput{URL: w.URL}
	if err := in.Validate(); err != nil {
		return core.Invalid("webhook %q is not valid: %s", w.URL, message(err))
	}
	return nil
}

// message returns an error's text without its kind prefix, so a nested failure
// reads as one sentence rather than a stack of "invalid: invalid:".
func message(err error) string {
	var domain *core.Error
	switch {
	case err == nil:
		return ""
	case errors.As(err, &domain):
		return domain.Message
	default:
		return err.Error()
	}
}

// Sort puts components in bundle order: by kind, then by key. Exporting the
// same components twice therefore produces the same bytes.
func Sort(cs []Component) {
	sort.SliceStable(cs, func(a, b int) bool {
		ai, _ := Order(cs[a].Kind)
		bi, _ := Order(cs[b].Kind)
		if ai != bi {
			return ai < bi
		}
		return cs[a].Key() < cs[b].Key()
	})
}
