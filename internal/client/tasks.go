package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
)

// CreateTask creates a task.
func (c *Client) CreateTask(ctx context.Context, in core.CreateTaskInput) (*core.Task, error) {
	return call[core.Task](ctx, c, http.MethodPost, httpapi.RouteTasks, nil, in)
}

// GetTask returns one task.
func (c *Client) GetTask(ctx context.Context, ref core.TaskRef) (*core.Task, error) {
	return call[core.Task](ctx, c, http.MethodGet, taskPath(httpapi.RouteTask, ref), nil, nil)
}

// ListTasks returns one page of tasks.
func (c *Client) ListTasks(ctx context.Context, f core.TaskFilter) (core.TaskPage, error) {
	items, next, err := list[core.Task](ctx, c, httpapi.RouteTasks, taskFilterQuery(f))
	if err != nil {
		return core.TaskPage{}, err
	}
	return core.TaskPage{Tasks: items, NextCursor: next}, nil
}

// UpdateTask changes a task.
func (c *Client) UpdateTask(ctx context.Context, ref core.TaskRef, in core.UpdateTaskInput) (*core.Task, error) {
	return call[core.Task](ctx, c, http.MethodPatch, taskPath(httpapi.RouteTask, ref), nil, in)
}

// TransitionTask moves a task to a new status.
func (c *Client) TransitionTask(ctx context.Context, ref core.TaskRef, in core.TransitionInput) (*core.Task, error) {
	return call[core.Task](ctx, c, http.MethodPost, taskPath(httpapi.RouteTaskTransition, ref), nil, in)
}

// DeleteTask removes a task, soft by default.
func (c *Client) DeleteTask(ctx context.Context, ref core.TaskRef, in core.DeleteTaskInput) error {
	q := url.Values{}
	setBool(q, "hard", in.Hard)
	setBool(q, "cascade", in.Cascade)
	return callVoid(ctx, c, http.MethodDelete, taskPath(httpapi.RouteTask, ref), q, nil)
}

// RestoreTask undoes a soft delete.
func (c *Client) RestoreTask(ctx context.Context, ref core.TaskRef) (*core.Task, error) {
	return call[core.Task](ctx, c, http.MethodPost, taskPath(httpapi.RouteTaskRestore, ref), nil, nil)
}

// TaskTree returns a task and its descendants.
func (c *Client) TaskTree(ctx context.Context, ref core.TaskRef, depth int) ([]core.Task, error) {
	q := url.Values{}
	if depth != 0 {
		q.Set("depth", strconv.Itoa(depth))
	}
	return listAll[core.Task](ctx, c, taskPath(httpapi.RouteTaskTree, ref), q)
}

// AddDependency records that ref waits for dependsOn.
func (c *Client) AddDependency(ctx context.Context, ref, dependsOn core.TaskRef) error {
	body := struct {
		DependsOn string `json:"depends_on"`
	}{DependsOn: dependsOn.String()}
	return callVoid(ctx, c, http.MethodPost, taskPath(httpapi.RouteTaskDeps, ref), nil, body)
}

// RemoveDependency drops a dependency edge.
func (c *Client) RemoveDependency(ctx context.Context, ref, dependsOn core.TaskRef) error {
	path := routePath(httpapi.RouteTaskDep, "ref", ref.String(), "dep", dependsOn.String())
	return callVoid(ctx, c, http.MethodDelete, path, nil, nil)
}

// ListDependencies returns a task's dependency edges.
func (c *Client) ListDependencies(ctx context.Context, ref core.TaskRef) ([]core.Dependency, error) {
	return listAll[core.Dependency](ctx, c, taskPath(httpapi.RouteTaskDeps, ref), nil)
}

// AddTag attaches a tag to a task.
func (c *Client) AddTag(ctx context.Context, ref core.TaskRef, tag string) error {
	body := struct {
		Name string `json:"name"`
	}{Name: tag}
	return callVoid(ctx, c, http.MethodPost, taskPath(httpapi.RouteTaskLabels, ref), nil, body)
}

// RemoveTag detaches a tag from a task.
func (c *Client) RemoveTag(ctx context.Context, ref core.TaskRef, tag string) error {
	path := routePath(httpapi.RouteTaskLabel, "ref", ref.String(), "name", tag)
	return callVoid(ctx, c, http.MethodDelete, path, nil, nil)
}

// ListTags returns the tenant's tags.
func (c *Client) ListTags(ctx context.Context) ([]core.Tag, error) {
	return listAll[core.Tag](ctx, c, httpapi.RouteLabels, nil)
}

// AddComment posts a comment on a task.
func (c *Client) AddComment(ctx context.Context, ref core.TaskRef, body string) (*core.Comment, error) {
	payload := struct {
		Body string `json:"body"`
	}{Body: body}
	return call[core.Comment](ctx, c, http.MethodPost, taskPath(httpapi.RouteTaskComments, ref), nil, payload)
}

// ListComments returns a task's comments.
func (c *Client) ListComments(ctx context.Context, ref core.TaskRef) ([]core.Comment, error) {
	return listAll[core.Comment](ctx, c, taskPath(httpapi.RouteTaskComments, ref), nil)
}

// EditComment rewrites a comment body.
func (c *Client) EditComment(ctx context.Context, id, body string) (*core.Comment, error) {
	payload := struct {
		Body string `json:"body"`
	}{Body: body}
	return call[core.Comment](ctx, c, http.MethodPatch, routePath(httpapi.RouteComment, "id", id), nil, payload)
}

// DeleteComment removes a comment.
func (c *Client) DeleteComment(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(httpapi.RouteComment, "id", id), nil, nil)
}

// PutArtifact records structured output on a task.
func (c *Client) PutArtifact(ctx context.Context, ref core.TaskRef, in core.ArtifactInput) (*core.Artifact, error) {
	return call[core.Artifact](ctx, c, http.MethodPut, taskPath(httpapi.RouteTaskArtifacts, ref), nil, in)
}

// ListArtifacts returns a task's artifacts.
func (c *Client) ListArtifacts(ctx context.Context, ref core.TaskRef) ([]core.Artifact, error) {
	return listAll[core.Artifact](ctx, c, taskPath(httpapi.RouteTaskArtifacts, ref), nil)
}

func taskPath(pattern string, ref core.TaskRef) string {
	return routePath(pattern, "ref", ref.String())
}

func taskFilterQuery(f core.TaskFilter) url.Values {
	q := pageQuery(f.Page)
	setStrings(q, "project_id", f.ProjectIDs)
	setStrings(q, "project", f.ProjectKeys)
	setStrings(q, "status", f.Statuses)
	setStrings(q, "tag", f.Tags)
	setStrings(q, "assignee", f.AssigneeIDs)
	setStrings(q, "creator", f.CreatorIDs)
	setStrings(q, "claimed_by", f.ClaimedBy)
	for _, p := range f.Priorities {
		q.Add("priority", strconv.Itoa(int(p)))
	}
	setTime(q, "due_before", f.DueBefore)
	setTime(q, "due_after", f.DueAfter)
	if f.ParentID != "" {
		q.Set("parent_id", f.ParentID)
	}
	setBool(q, "root_only", f.ParentIsNull)
	setTriState(q, "claimed", f.Claimed)
	setTriState(q, "blocked", f.Blocked)
	if f.Query != "" {
		q.Set("q", f.Query)
	}
	setJSON(q, "custom_fields", f.CustomFields)
	setBool(q, "include_deleted", f.IncludeDeleted)
	return q
}
