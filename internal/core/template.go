// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Template limits.
const (
	MaxTemplateNameLength = 100
	// MaxTemplateTasks bounds the tasks one template holds, its root included,
	// so a use stays one transaction of a size a reader would plan by hand.
	MaxTemplateTasks = 200
	// MaxTemplateDueIn bounds a due offset at ten years.
	MaxTemplateDueIn = Duration(10 * 365 * 24 * time.Hour)
)

// TaskTemplate is a named shape of a task that is not itself work: it never
// appears where tasks do, and using it creates tasks.
type TaskTemplate struct {
	ID       string `json:"id" yaml:"id"`
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	Name     string `json:"name" yaml:"name"`
	// ProjectID is the project the template belongs to, empty for a template
	// the whole tenant shares. ProjectKey is that project's key, for display.
	ProjectID  string       `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	ProjectKey string       `json:"project_key,omitempty" yaml:"project_key,omitempty"`
	Task       TemplateTask `json:"task" yaml:"task"`
	// DueIn is how far from a use its task falls due, zero for no due date.
	DueIn Duration `json:"due_in,omitempty" yaml:"due_in,omitempty"`
	// Uses counts the times the template was used; the N placeholder is the
	// count the next use makes.
	Uses           int       `json:"uses" yaml:"uses"`
	CreatorActorID string    `json:"creator_actor_id,omitempty" yaml:"creator_actor_id,omitempty"`
	CreatedAt      time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" yaml:"updated_at"`
}

// TenantWide reports whether the template is shared by the whole tenant.
func (t TaskTemplate) TenantWide() bool { return t.ProjectID == "" }

// TemplateTask is the part of a task a template keeps, with its subtasks.
type TemplateTask struct {
	Title        string         `json:"title" yaml:"title"`
	Body         string         `json:"body,omitempty" yaml:"body,omitempty"`
	Priority     Priority       `json:"priority,omitempty" yaml:"priority,omitempty"`
	Tags         []string       `json:"tags,omitempty" yaml:"tags,omitempty"`
	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	Recurrence   string         `json:"recurrence,omitempty" yaml:"recurrence,omitempty"`
	Subtasks     []TemplateTask `json:"subtasks,omitempty" yaml:"subtasks,omitempty"`
}

// Count is the number of tasks the template task stands for, itself included.
func (t TemplateTask) Count() int {
	n := 1
	for _, s := range t.Subtasks {
		n += s.Count()
	}
	return n
}

// TemplateFilter narrows a template listing.
type TemplateFilter struct {
	// ProjectRef keeps the templates of one project and the tenant-wide ones.
	ProjectRef string `json:"project,omitempty" yaml:"project,omitempty"`
	Page       Page   `json:"page" yaml:"page"`
}

// SaveTemplateInput names a task to keep as a template.
type SaveTemplateInput struct {
	Name    string `json:"name" yaml:"name"`
	TaskRef string `json:"task_ref" yaml:"task_ref"`
	// TenantWide shares the template with every project rather than keeping
	// it in the task's own.
	TenantWide   bool     `json:"tenant_wide,omitempty" yaml:"tenant_wide,omitempty"`
	WithSubtasks bool     `json:"with_subtasks,omitempty" yaml:"with_subtasks,omitempty"`
	DueIn        Duration `json:"due_in,omitempty" yaml:"due_in,omitempty"`
}

// Validate checks the input.
func (in SaveTemplateInput) Validate() error {
	if err := ValidateTemplateName(in.Name); err != nil {
		return err
	}
	if strings.TrimSpace(in.TaskRef) == "" {
		return Invalid("a task to save is required").WithDetail(DetailField, "task_ref")
	}
	return validateDueIn(in.DueIn)
}

// UpdateTemplateInput changes a template. A nil field is left as it is.
type UpdateTemplateInput struct {
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// ProjectRef moves the template to a project; a pointer to the empty
	// string makes it tenant-wide.
	ProjectRef *string   `json:"project_ref,omitempty" yaml:"project_ref,omitempty"`
	DueIn      *Duration `json:"due_in,omitempty" yaml:"due_in,omitempty"`
	Title      *string   `json:"title,omitempty" yaml:"title,omitempty"`
	Body       *string   `json:"body,omitempty" yaml:"body,omitempty"`
	Priority   *Priority `json:"priority,omitempty" yaml:"priority,omitempty"`
	Tags       *[]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	// CustomFields sets the named fields; a null value removes one.
	CustomFields map[string]any `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
	// Recurrence replaces the repeat rule; a pointer to the empty string
	// removes it.
	Recurrence *string `json:"recurrence,omitempty" yaml:"recurrence,omitempty"`
}

// Validate checks the input.
func (in UpdateTemplateInput) Validate() error {
	if in.Name != nil {
		if err := ValidateTemplateName(*in.Name); err != nil {
			return err
		}
	}
	if in.Title != nil {
		if err := validateTemplateTitle(*in.Title); err != nil {
			return err
		}
	}
	if in.Body != nil && len(*in.Body) > MaxBodyLength {
		return Invalid("task body must be at most %d characters", MaxBodyLength).WithDetail(DetailField, "body")
	}
	if in.Priority != nil && !in.Priority.Valid() {
		return Invalid("priority %d is out of range", *in.Priority).WithDetail(DetailField, "priority")
	}
	if in.DueIn != nil {
		return validateDueIn(*in.DueIn)
	}
	return nil
}

// UseTemplateInput says where a task made from a template goes. An empty
// ProjectRef uses the template's project, an empty Title the template's, and
// a nil DueIn the template's offset; a zero DueIn sets no due date.
type UseTemplateInput struct {
	ProjectRef string    `json:"project_ref,omitempty" yaml:"project_ref,omitempty"`
	Title      string    `json:"title,omitempty" yaml:"title,omitempty"`
	DueIn      *Duration `json:"due_in,omitempty" yaml:"due_in,omitempty"`
}

// Validate checks the input.
func (in UseTemplateInput) Validate() error {
	if len(in.Title) > MaxTitleLength {
		return Invalid("task title must be at most %d characters", MaxTitleLength).WithDetail(DetailField, "title")
	}
	if in.DueIn != nil {
		return validateDueIn(*in.DueIn)
	}
	return nil
}

// ValidateTemplateName checks a template name.
func ValidateTemplateName(name string) error {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return Invalid("template name is required").WithDetail(DetailField, "name")
	case len(name) > MaxTemplateNameLength:
		return Invalid("template name must be at most %d characters", MaxTemplateNameLength).WithDetail(DetailField, "name")
	}
	return nil
}

func validateTemplateTitle(title string) error {
	title = strings.TrimSpace(title)
	switch {
	case title == "":
		return Invalid("task title is required").WithDetail(DetailField, "title")
	case len(title) > MaxTitleLength:
		return Invalid("task title must be at most %d characters", MaxTitleLength).WithDetail(DetailField, "title")
	}
	return nil
}

func validateDueIn(d Duration) error {
	if d < 0 || d > MaxTemplateDueIn {
		return Invalid("due offset must be between 0 and %s", MaxTemplateDueIn.Human()).WithDetail(DetailField, "due_in")
	}
	return nil
}

// TemplateValues are what a use fills a title's placeholders with.
type TemplateValues struct {
	Date    time.Time
	N       int
	Project string
}

var placeholder = regexp.MustCompile(`\{([A-Za-z]+)\}`)

// FillPlaceholders replaces the placeholders a template title may carry,
// whatever their case: DATE, N and PROJECT in braces. Any other braced word
// is left as written.
func FillPlaceholders(title string, v TemplateValues) string {
	return placeholder.ReplaceAllStringFunc(title, func(m string) string {
		switch strings.ToLower(m[1 : len(m)-1]) {
		case "date":
			return v.Date.UTC().Format(time.DateOnly)
		case "n":
			return strconv.Itoa(v.N)
		case "project":
			return v.Project
		}
		return m
	})
}

// TemplateService covers task templates.
type TemplateService interface {
	// SaveTemplate keeps a task, and its subtasks when asked, as a template.
	SaveTemplate(ctx context.Context, in SaveTemplateInput) (*TaskTemplate, error)
	// GetTemplate reads one template by name or identifier.
	GetTemplate(ctx context.Context, ref string) (*TaskTemplate, error)
	// ListTemplates lists the templates the caller can read, by name.
	ListTemplates(ctx context.Context, f TemplateFilter) ([]TaskTemplate, string, error)
	// UpdateTemplate renames, rescopes or edits a template.
	UpdateTemplate(ctx context.Context, ref string, in UpdateTemplateInput) (*TaskTemplate, error)
	// DeleteTemplate removes a template; tasks made from it stay.
	DeleteTemplate(ctx context.Context, ref string) error
	// UseTemplate creates a task, with the template's subtasks, and returns it.
	UseTemplate(ctx context.Context, ref string, in UseTemplateInput) (*Task, error)
}
