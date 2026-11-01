// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// taskForm is what Add with details was submitted with, so a refused add is
// drawn again holding every value the reader typed.
type taskForm struct {
	Title     string
	Status    string
	Body      string
	Project   string
	Priority  string
	Due       string
	Assignee  string
	Tags      string
	Repeat    string
	Parent    string
	DependsOn string

	// Quick marks the one-line box, whose title is a quick-add line.
	Quick bool
}

// More reports whether the folded fields hold anything, which is when the
// More options disclosure is drawn open.
func (f taskForm) More() bool {
	return f.Repeat != "" || f.Parent != "" || f.DependsOn != ""
}

// readTaskForm reads both add forms, the one-line box and Add with details.
func readTaskForm(r *http.Request) taskForm {
	return taskForm{
		Title: field(r, "title"), Status: field(r, "status"), Body: prose(r, "body"), Project: field(r, "project_ref"),
		Priority: field(r, "priority"), Due: field(r, "due_at"), Assignee: field(r, "assignee"),
		Tags: field(r, "tags"), Repeat: field(r, "repeat"), Parent: field(r, "parent"),
		DependsOn: field(r, "depends_on"), Quick: field(r, "quick") != "",
	}
}

// input turns the form into what the service creates a task from.
func (f taskForm) input(r *http.Request) (core.CreateTaskInput, error) {
	priority, err := priorityField(r, "priority")
	if err != nil {
		return core.CreateTaskInput{}, core.OnField(err, "priority")
	}
	due, err := dateField(f.Due, "due_at")
	if err != nil {
		return core.CreateTaskInput{}, err
	}
	return core.CreateTaskInput{
		ProjectRef: f.Project, Title: f.Title, Status: f.Status, Body: f.Body, Priority: priority,
		Tags: splitList(f.Tags), Recurrence: f.Repeat, AssigneeActorID: f.Assignee,
		ParentRef: f.Parent, DependsOn: splitList(f.DependsOn), DueAt: due,
	}, nil
}

// dateField reads a date control's YYYY-MM-DD value as midnight UTC, the same
// instant the command line's --due gives a bare date.
func dateField(raw, name string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return nil, core.Invalid("date %q must be YYYY-MM-DD", raw).WithDetail(core.DetailField, name)
	}
	return &t, nil
}

// projectForm is what the new project form and the project settings were
// submitted with.
type projectForm struct {
	Key         string
	Name        string
	Workflow    string
	Description string
	Color       string
	Icon        string
}

// Colour is the submitted or stored colour as the colour field draws it.
func (f projectForm) Colour() core.ProjectColor { return core.ProjectColor(f.Color) }

// readProjectForm reads either project form.
func readProjectForm(r *http.Request) projectForm {
	return projectForm{
		Key: field(r, "key"), Name: field(r, "name"), Workflow: field(r, "workflow_key"),
		Description: prose(r, "description"), Color: colourField(r, "color"), Icon: field(r, "icon"),
	}
}

// formOf is a project's stored values as its settings form shows them.
func formOf(p core.Project, workflowKey string) projectForm {
	return projectForm{Key: p.Key, Name: p.Name, Workflow: workflowKey,
		Description: p.Description, Color: string(p.Color), Icon: p.Icon}
}

// refusable reports whether a refused create or save is answered by drawing
// the form again rather than with the error screen. A not-found refusal is
// one the reader can correct when it names a field: an unknown parent or
// assignee is a typo in a box on the form.
func refusable(err error) bool {
	return correctable(err) || core.KindOf(err) == core.KindNotFound && core.FieldOf(err) != ""
}

// refusedRequest is the request a refused add is drawn for: the same request
// with the listing it was posted from as its address, so the list under the
// form keeps its filter, sort and page.
func refusedRequest(r *http.Request) *http.Request {
	clone := r.Clone(r.Context())
	if u, err := url.Parse(listingReferer(r)); err == nil && u.Path == RouteTasks {
		clone.URL = u
		return clone
	}
	clone.URL = &url.URL{Path: RouteTasks}
	return clone
}

// soleProject is the project a filter names when it names exactly one, which
// the add forms preselect.
func soleProject(keys []string) string {
	if len(keys) != 1 {
		return ""
	}
	return strings.ToLower(keys[0])
}
