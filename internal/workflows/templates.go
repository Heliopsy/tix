// SPDX-License-Identifier: AGPL-3.0-or-later

// Package workflows holds the built-in workflow templates: ready-made state
// machines, each a plain YAML file under templates/ in the workflow definition
// format plus a name, a one-line purpose and the custom fields a project using
// it usually wants.
package workflows

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"go.yaml.in/yaml/v4"

	"github.com/heliopsy/tix/internal/core"
)

//go:embed templates/*.yaml
var files embed.FS

// order is the sequence templates are offered in: the simplest first, then
// software work, then everything else.
var order = []string{
	"tix-kanban", "tix-gtd", "tix-dev-review", "tix-bugs", "tix-releases", "tix-agent-review",
	"tix-incident", "tix-support", "tix-approval", "tix-sales", "tix-hiring", "tix-content",
}

var (
	loadOnce sync.Once
	loaded   []core.WorkflowTemplate
	loadErr  error
)

// Templates returns every built-in template in offering order. The files are
// part of the binary, so an error here is a build defect the tests catch.
func Templates() ([]core.WorkflowTemplate, error) {
	loadOnce.Do(func() { loaded, loadErr = load(files) })
	if loadErr != nil {
		return nil, loadErr
	}
	out := make([]core.WorkflowTemplate, len(loaded))
	copy(out, loaded)
	return out, nil
}

// Lookup returns the template with the given key, ignoring case.
func Lookup(key string) (core.WorkflowTemplate, bool, error) {
	all, err := Templates()
	if err != nil {
		return core.WorkflowTemplate{}, false, err
	}
	key = strings.ToLower(strings.TrimSpace(key))
	for _, t := range all {
		if t.Key == key {
			return t, true, nil
		}
	}
	return core.WorkflowTemplate{}, false, nil
}

// load parses and checks every template file.
func load(fsys fs.FS) ([]core.WorkflowTemplate, error) {
	names, err := fs.Glob(fsys, "templates/*.yaml")
	if err != nil {
		return nil, err
	}
	out := make([]core.WorkflowTemplate, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		t, err := Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("workflow template %s: %w", path.Base(name), err)
		}
		if seen[t.Key] {
			return nil, fmt.Errorf("workflow template %s: key %q is used twice", path.Base(name), t.Key)
		}
		seen[t.Key] = true
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i].Key) < rank(out[j].Key) })
	return out, nil
}

// rank is a template's place in the offering order; one not listed sorts last.
func rank(key string) int {
	for i, k := range order {
		if k == key {
			return i
		}
	}
	return len(order)
}

// Parse reads one template document and checks it: the key carries the
// reserved prefix, name and purpose are present, the definition passes the
// workflow validator and every field passes the field validator.
func Parse(raw []byte) (core.WorkflowTemplate, error) {
	var t core.WorkflowTemplate
	if err := yaml.Load(raw, &t, yaml.WithKnownFields()); err != nil {
		return t, err
	}
	t.Key = strings.ToLower(strings.TrimSpace(t.Key))
	switch {
	case !core.ReservedWorkflowKey(t.Key):
		return t, fmt.Errorf("key %q does not start with %q", t.Key, core.WorkflowTemplatePrefix)
	case strings.TrimSpace(t.Name) == "":
		return t, fmt.Errorf("template %q has no name", t.Key)
	case strings.TrimSpace(t.Purpose) == "":
		return t, fmt.Errorf("template %q has no purpose", t.Key)
	case strings.Contains(strings.TrimSpace(t.Purpose), "\n"):
		return t, fmt.Errorf("template %q has a purpose longer than one line", t.Key)
	}
	in := core.WorkflowInput{Key: t.Key, Name: t.Name, Definition: t.Definition}
	if err := in.Validate(); err != nil {
		return t, err
	}
	for _, f := range t.Fields {
		if err := f.Validate(); err != nil {
			return t, fmt.Errorf("field %q: %w", f.Key, err)
		}
	}
	t.Purpose = strings.TrimSpace(t.Purpose)
	t.States = t.Definition.StatesSummary()
	return t, nil
}
