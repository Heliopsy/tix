// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

//go:embed overlay.yaml
var builtinOverlay []byte

// Entry is the overlay's decision about one operation. Every field is
// optional, and a pointer tells a field left out from one set to its zero.
type Entry struct {
	// Hide keeps the operation off the tool list. It needs a Reason.
	Hide *bool `yaml:"hide,omitempty"`
	// Reason says why the operation is hidden or why its destructive marking
	// departs from what its method implies.
	Reason string `yaml:"reason,omitempty"`
	// Name replaces the tool name derived from the operationId.
	Name string `yaml:"name,omitempty"`
	// Description replaces the description taken from the document.
	Description string `yaml:"description,omitempty"`
	// Destructive overrides the default, which is true for DELETE alone.
	Destructive *bool `yaml:"destructive,omitempty"`
	// Enable lists a destructive tool without --allow-destructive.
	Enable *bool `yaml:"enable,omitempty"`
	// HideParams keeps optional parameters out of the input schema.
	HideParams []string `yaml:"hide_params,omitempty"`
}

// Overlay is the set of per-operation decisions, keyed by operationId.
type Overlay struct {
	Operations map[string]Entry `yaml:"operations"`
}

// BuiltinOverlay returns the overlay shipped in the binary.
func BuiltinOverlay() (Overlay, error) { return ParseOverlay(builtinOverlay) }

// ParseOverlay reads an overlay, refusing a key it does not know.
func ParseOverlay(raw []byte) (Overlay, error) {
	var ov Overlay
	if len(bytes.TrimSpace(raw)) == 0 {
		return ov, nil
	}
	if err := yaml.Load(raw, &ov, yaml.WithKnownFields()); err != nil {
		return Overlay{}, fmt.Errorf("reading the overlay: %w", err)
	}
	return ov, nil
}

// Merge lays top over o, field by field, so a user file can change one
// decision about an operation without restating the others.
func (o Overlay) Merge(top Overlay) Overlay {
	out := Overlay{Operations: map[string]Entry{}}
	for id, e := range o.Operations {
		out.Operations[id] = e
	}
	for id, t := range top.Operations {
		e := out.Operations[id]
		if t.Hide != nil {
			e.Hide = t.Hide
		}
		if t.Reason != "" {
			e.Reason = t.Reason
		}
		if t.Name != "" {
			e.Name = t.Name
		}
		if t.Description != "" {
			e.Description = t.Description
		}
		if t.Destructive != nil {
			e.Destructive = t.Destructive
		}
		if t.Enable != nil {
			e.Enable = t.Enable
		}
		if t.HideParams != nil {
			e.HideParams = t.HideParams
		}
		out.Operations[id] = e
	}
	return out
}

// Check holds the overlay to the document: every key names an operation,
// every judgement carries its reason and every hidden parameter exists.
func (o Overlay) Check(doc *Document) error {
	ids := make([]string, 0, len(o.Operations))
	for id := range o.Operations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var errs []error
	for _, id := range ids {
		e := o.Operations[id]
		op, ok := doc.Lookup(id)
		if !ok {
			errs = append(errs, fmt.Errorf("overlay names %q, which is not an operation of the api", id))
			continue
		}
		if isSet(e.Hide) && strings.TrimSpace(e.Reason) == "" {
			errs = append(errs, fmt.Errorf("overlay hides %q without a reason", id))
		}
		if e.Destructive != nil && *e.Destructive != (op.Method == http.MethodDelete) && strings.TrimSpace(e.Reason) == "" {
			errs = append(errs, fmt.Errorf("overlay overrides whether %q is destructive without a reason", id))
		}
		for _, name := range e.HideParams {
			if !slices.ContainsFunc(op.Params, func(p Param) bool { return p.Name == name && !p.Required }) {
				errs = append(errs, fmt.Errorf("overlay hides parameter %q of %q, which has no such optional parameter", name, id))
			}
		}
	}
	return errors.Join(errs...)
}

func isSet(b *bool) bool { return b != nil && *b }
