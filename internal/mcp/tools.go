// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// State is what became of one operation.
type State string

// The states an operation can end in.
const (
	StateExposed State = "exposed"
	// StateDestructive is a destructive tool left off the list because
	// neither --allow-destructive nor the overlay enabled it.
	StateDestructive State = "destructive"
	// StateOutOfScope is a tool the caller's token lacks the scope for.
	StateOutOfScope State = "out of scope"
	StateHidden     State = "hidden"
)

// bodyArgument is the argument a request body travels in when its properties
// cannot sit beside the parameters.
const bodyArgument = "body"

// validName is the tool name every MCP client accepts.
var validName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Options are the run-time decisions a tool plan depends on.
type Options struct {
	AllowDestructive bool
	// HasScope reports whether the caller holds a scope. Nil holds them all,
	// which is how a listing describes the plan without asking a server.
	HasScope func(scope string) bool
}

// Tool is one operation together with the tool it became, or why it did not.
type Tool struct {
	Name        string
	Op          *Operation
	Description string
	Destructive bool
	// ReadOnly marks a tool that only reads whatever its method.
	ReadOnly bool
	State    State
	Reason   string
	Schema   map[string]any

	// nested is set when the body travels whole under bodyArgument.
	nested bool
	// args maps each argument name to the parameter it fills, nil for a
	// property of the request body.
	args map[string]*Param
}

// Plan decides every operation of the document under the overlay.
func Plan(doc *Document, ov Overlay, opts Options) ([]Tool, error) {
	if err := ov.Check(doc); err != nil {
		return nil, err
	}
	tools := make([]Tool, 0, len(doc.Operations))
	names := map[string]string{}
	var errs []error
	for i := range doc.Operations {
		op := &doc.Operations[i]
		e := ov.Operations[op.ID]
		t := Tool{Op: op, Name: toolName(op.ID), Description: describe(op)}
		if e.Name != "" {
			t.Name = e.Name
		}
		if e.Description != "" {
			t.Description = e.Description
		}
		t.ReadOnly = isSet(e.ReadOnly)
		t.Destructive = op.Method == http.MethodDelete
		if e.Destructive != nil {
			t.Destructive = *e.Destructive
		}
		switch {
		case op.Unsupported != "":
			t.State, t.Reason = StateHidden, op.Unsupported
		case isSet(e.Hide):
			t.State, t.Reason = StateHidden, e.Reason
		case t.Destructive && !opts.AllowDestructive && !isSet(e.Enable):
			t.State, t.Reason = StateDestructive, "pass --allow-destructive or enable it in an overlay"
		case opts.HasScope != nil && op.Scope != "" && !opts.HasScope(op.Scope):
			t.State, t.Reason = StateOutOfScope, "the token lacks "+op.Scope
		default:
			t.State = StateExposed
		}
		if !validName.MatchString(t.Name) {
			errs = append(errs, fmt.Errorf("tool name %q of %q is not a valid MCP tool name", t.Name, op.ID))
		}
		if prev, dup := names[t.Name]; dup {
			errs = append(errs, fmt.Errorf("tool name %q is taken by both %q and %q", t.Name, prev, op.ID))
		}
		names[t.Name] = op.ID
		if op.Unsupported == "" {
			t.Schema, t.nested, t.args = inputSchema(op, e.HideParams)
		}
		tools = append(tools, t)
	}
	return tools, errors.Join(errs...)
}

// Exposed returns the tools a client is offered.
func Exposed(tools []Tool) []Tool {
	return slices.DeleteFunc(slices.Clone(tools), func(t Tool) bool { return t.State != StateExposed })
}

// toolName turns an operationId into a tool name: dots are not in the
// character set every client accepts.
func toolName(id string) string { return strings.ReplaceAll(id, ".", "_") }

// describe joins the summary and description, dropping the lines that point
// into the repository, which mean nothing to a model.
func describe(op *Operation) string {
	text := op.Description
	if text == "" {
		text = op.Summary
	}
	var keep []string
	for _, para := range strings.Split(text, "\n\n") {
		trimmed := strings.TrimSpace(para)
		if trimmed == "" || strings.HasPrefix(trimmed, "Specified in ") {
			continue
		}
		keep = append(keep, trimmed)
	}
	out := strings.Join(keep, "\n\n")
	return fmt.Sprintf("%s\n\n%s %s", out, op.Method, op.Path)
}

// inputSchema merges the parameters and the request body into one object.
// Body properties sit at the top level when none collides with a parameter
// and the body is an object; otherwise the body travels whole as "body".
func inputSchema(op *Operation, hidden []string) (map[string]any, bool, map[string]*Param) {
	props := map[string]any{}
	args := map[string]*Param{}
	var required []string
	for i := range op.Params {
		p := &op.Params[i]
		if slices.Contains(hidden, p.Name) {
			continue
		}
		schema := map[string]any{}
		for k, v := range p.Schema {
			schema[k] = v
		}
		if p.Description != "" {
			schema["description"] = p.Description
		}
		props[p.Name] = schema
		args[p.Name] = p
		if p.Required {
			required = append(required, p.Name)
		}
	}
	nested := false
	if op.Body != nil {
		bodyProps, isObject := op.Body["properties"].(map[string]any)
		collides := !isObject || op.Body["type"] != "object"
		for name := range bodyProps {
			if _, taken := props[name]; taken {
				collides = true
			}
		}
		if collides {
			nested = true
			props[bodyArgument] = op.Body
			if op.BodyRequired {
				required = append(required, bodyArgument)
			}
		} else {
			for name, s := range bodyProps {
				props[name] = s
			}
			if req, ok := op.Body["required"].([]any); ok && op.BodyRequired {
				for _, r := range req {
					if s, ok := r.(string); ok {
						required = append(required, s)
					}
				}
			}
		}
	}
	schema := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		slices.Sort(required)
		schema["required"] = required
	}
	return schema, nested, args
}
