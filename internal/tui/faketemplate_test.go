// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
)

// fakeTemplates is what the fake holds and records about task templates.
type fakeTemplates struct {
	list    []core.TaskTemplate
	listErr error
	filters []core.TemplateFilter
	read    []string
	saved   []core.SaveTemplateInput
	renamed [][2]string
	deleted []string
	used    []templateUseCall
}

// templateUseCall is one UseTemplate the fake was asked for.
type templateUseCall struct {
	ref string
	in  core.UseTemplateInput
}

func (f *fakeService) SaveTemplate(_ context.Context, in core.SaveTemplateInput) (*core.TaskTemplate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tpl.saved = append(f.tpl.saved, in)
	return &core.TaskTemplate{ID: "tpl", Name: in.Name}, nil
}

func (f *fakeService) GetTemplate(_ context.Context, ref string) (*core.TaskTemplate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tpl.read = append(f.tpl.read, ref)
	for _, t := range f.tpl.list {
		if t.Name == ref {
			return &t, nil
		}
	}
	return nil, core.NotFound("template %q", ref)
}

func (f *fakeService) ListTemplates(_ context.Context, filter core.TemplateFilter) ([]core.TaskTemplate, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tpl.filters = append(f.tpl.filters, filter)
	return append([]core.TaskTemplate(nil), f.tpl.list...), "", f.tpl.listErr
}

func (f *fakeService) UpdateTemplate(_ context.Context, ref string, in core.UpdateTemplateInput) (*core.TaskTemplate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name := ""
	if in.Name != nil {
		name = *in.Name
	}
	f.tpl.renamed = append(f.tpl.renamed, [2]string{ref, name})
	return &core.TaskTemplate{Name: name}, nil
}

func (f *fakeService) DeleteTemplate(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tpl.deleted = append(f.tpl.deleted, ref)
	return nil
}

func (f *fakeService) UseTemplate(_ context.Context, ref string, in core.UseTemplateInput) (*core.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tpl.used = append(f.tpl.used, templateUseCall{ref: ref, in: in})
	return &core.Task{ID: "made", Ref: in.ProjectRef + "-77"}, nil
}
