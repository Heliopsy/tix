// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// AddDependency records that a task waits for another.
func (t *tx) AddDependency(ctx context.Context, d *core.Dependency) error {
	d.TenantID = t.scope.TenantID
	if d.CreatedAt.IsZero() {
		d.CreatedAt = t.store.clock.Now()
	}
	if d.TaskID == d.DependsOn {
		return core.Invalid("a task cannot depend on itself")
	}
	ins := t.insert("task_deps").
		Set("task_id", d.TaskID).
		Set("depends_on", d.DependsOn).
		Set("created_at", timeArg(d.CreatedAt))
	_, err := t.execInsert(ctx, ins, "adding dependency %q -> %q", d.TaskID, d.DependsOn)
	return err
}

// RemoveDependency drops one dependency edge.
func (t *tx) RemoveDependency(ctx context.Context, taskID, dependsOn string) error {
	b := t.builder("task_deps").
		Where("task_id = ?", taskID).
		Where("depends_on = ?", dependsOn)
	n, err := t.execDelete(ctx, b, "removing dependency")
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("task %q does not depend on %q", taskID, dependsOn)
	}
	return nil
}

// ListDependencies returns the edges leaving a task.
func (t *tx) ListDependencies(ctx context.Context, taskID string) ([]core.Dependency, error) {
	b := t.builder("task_deps").
		Select("tenant_id", "task_id", "depends_on", "created_at").
		Where("task_id = ?", taskID).
		OrderBy("depends_on", core.Ascending)
	rows, err := t.query(ctx, b, "listing dependencies")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Dependency{}
	for rows.Next() {
		var (
			d       core.Dependency
			created sql.NullTime
		)
		if err := rows.Scan(&d.TenantID, &d.TaskID, &d.DependsOn, &created); err != nil {
			return nil, mapErr(err, "scanning dependency")
		}
		d.CreatedAt = scanTime(created)
		out = append(out, d)
	}
	return out, mapRowsErr(rows, "listing dependencies")
}

// DependencyPathExists reports whether dependencies lead from one task to another.
func (t *tx) DependencyPathExists(ctx context.Context, from, to string) (bool, error) {
	q, args, err := sqlb.DependencyPathQuery(dialect, t.scope, from, to)
	if err != nil {
		return false, err
	}
	var found bool
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&found); err != nil {
		return false, mapErr(err, "walking dependencies from %q to %q", from, to)
	}
	return found, nil
}

var tagColumns = []string{"id", "tenant_id", "project_id", "name", "color"}

func scanTag(s scanner) (core.Tag, error) {
	var (
		l       core.Tag
		project sql.NullString
	)
	if err := s.Scan(&l.ID, &l.TenantID, &project, &l.Name, &l.Color); err != nil {
		return core.Tag{}, mapErr(err, "scanning tag")
	}
	l.ProjectID = text(project)
	return l, nil
}

// PutTag creates or replaces a tag, keyed by its project and name.
func (t *tx) PutTag(ctx context.Context, l *core.Tag) error {
	l.TenantID = t.scope.TenantID

	upd := t.builder("tags").Where("name = ?", l.Name)
	scopeTagToProject(upd, l.ProjectID)
	upd.Set("color", l.Color)
	n, err := t.execUpdate(ctx, upd, "updating tag %q", l.Name)
	if err != nil {
		return err
	}
	if n > 0 {
		existing, err := t.getTag(ctx, l.ProjectID, l.Name)
		if err != nil {
			return err
		}
		l.ID = existing.ID
		return nil
	}

	if l.ID == "" {
		l.ID = id.New()
	}
	ins := t.insert("tags").
		Set("id", l.ID).
		Set("project_id", nullText(l.ProjectID)).
		Set("name", l.Name).
		Set("color", l.Color)
	_, err = t.execInsert(ctx, ins, "creating tag %q", l.Name)
	return err
}

func scopeTagToProject(b *sqlb.Builder, projectID string) {
	if projectID == "" {
		b.Where("project_id IS NULL")
		return
	}
	b.Where("project_id = ?", projectID)
}

func (t *tx) getTag(ctx context.Context, projectID, name string) (*core.Tag, error) {
	b := t.builder("tags").Select(tagColumns...).Where("name = ?", name)
	scopeTagToProject(b, projectID)
	b.Limit(1)
	q, args := b.SelectQuery()
	l, err := scanTag(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("tag %q", name)
		}
		return nil, err
	}
	return &l, nil
}

// ListTags returns every tag in this tenant.
func (t *tx) ListTags(ctx context.Context) ([]core.Tag, error) {
	b := t.builder("tags").Select(tagColumns...).OrderBy("name", core.Ascending)
	rows, err := t.query(ctx, b, "listing tags")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Tag{}
	for rows.Next() {
		v, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing tags")
}

// AttachTag puts a tag on a task.
func (t *tx) AttachTag(ctx context.Context, taskID, tagID string) error {
	ins := t.insert("task_tags").Set("task_id", taskID).Set("tag_id", tagID)
	_, err := t.execInsert(ctx, ins, "attaching tag %q to task %q", tagID, taskID)
	if err != nil && core.IsKind(err, core.KindConflict) {
		return nil
	}
	return err
}

// DetachTag takes a tag off a task.
func (t *tx) DetachTag(ctx context.Context, taskID, tagID string) error {
	b := t.builder("task_tags").
		Where("task_id = ?", taskID).
		Where("tag_id = ?", tagID)
	n, err := t.execDelete(ctx, b, "detaching tag")
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("task %q does not carry tag %q", taskID, tagID)
	}
	return nil
}

var commentColumns = []string{
	"id", "tenant_id", "task_id", "author_actor_id", "body", "created_at", "updated_at", "deleted_at",
}

func scanComment(s scanner) (core.Comment, error) {
	var (
		c       core.Comment
		created sql.NullTime
		updated sql.NullTime
		deleted sql.NullTime
	)
	if err := s.Scan(&c.ID, &c.TenantID, &c.TaskID, &c.AuthorActorID, &c.Body,
		&created, &updated, &deleted); err != nil {
		return core.Comment{}, mapErr(err, "scanning comment")
	}
	c.CreatedAt = scanTime(created)
	c.UpdatedAt = scanTime(updated)
	c.DeletedAt = scanNullTime(deleted)
	return c, nil
}

// CreateComment inserts a comment on a task.
func (t *tx) CreateComment(ctx context.Context, c *core.Comment) error {
	if c.ID == "" {
		c.ID = id.New()
	}
	c.TenantID = t.scope.TenantID
	now := t.store.clock.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now

	ins := t.insert("comments").
		Set("id", c.ID).
		Set("task_id", c.TaskID).
		Set("author_actor_id", c.AuthorActorID).
		Set("body", c.Body).
		Set("created_at", timeArg(c.CreatedAt)).
		Set("updated_at", timeArg(c.UpdatedAt)).
		Set("deleted_at", nullTimeArg(c.DeletedAt))
	_, err := t.execInsert(ctx, ins, "creating comment on task %q", c.TaskID)
	return err
}

// GetComment returns one comment.
func (t *tx) GetComment(ctx context.Context, commentID string) (*core.Comment, error) {
	b := t.builder("comments").Select(commentColumns...).Where("id = ?", commentID).Limit(1)
	q, args := b.SelectQuery()
	c, err := scanComment(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("comment %q", commentID)
		}
		return nil, err
	}
	return &c, nil
}

// ListComments returns a task's live comments oldest first.
func (t *tx) ListComments(ctx context.Context, taskID string) ([]core.Comment, error) {
	b := t.builder("comments").
		Select(commentColumns...).
		Where("task_id = ?", taskID).
		Where("deleted_at IS NULL").
		OrderBy("created_at", core.Ascending).
		OrderBy("id", core.Ascending)
	rows, err := t.query(ctx, b, "listing comments")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Comment{}
	for rows.Next() {
		v, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing comments")
}

// UpdateComment rewrites a comment's body.
func (t *tx) UpdateComment(ctx context.Context, c *core.Comment) error {
	c.UpdatedAt = t.store.clock.Now()
	b := t.builder("comments").
		Where("id = ?", c.ID).
		Set("body", c.Body).
		Set("updated_at", timeArg(c.UpdatedAt)).
		Set("deleted_at", nullTimeArg(c.DeletedAt))
	n, err := t.execUpdate(ctx, b, "updating comment %q", c.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("comment %q", c.ID)
	}
	return nil
}

// DeleteComment soft deletes a comment.
func (t *tx) DeleteComment(ctx context.Context, commentID string) error {
	b := t.builder("comments").
		Where("id = ?", commentID).
		Where("deleted_at IS NULL").
		Set("deleted_at", t.now()).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, b, "deleting comment %q", commentID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("comment %q", commentID)
	}
	return nil
}

var artifactColumns = []string{
	"id", "tenant_id", "task_id", "actor_id", "kind", "name", "payload",
	"content_type", "blob", "created_at",
}

func scanArtifact(s scanner) (core.Artifact, error) {
	var (
		a       core.Artifact
		payload string
		created sql.NullTime
	)
	if err := s.Scan(&a.ID, &a.TenantID, &a.TaskID, &a.ActorID, &a.Kind, &a.Name,
		&payload, &a.ContentType, &a.Blob, &created); err != nil {
		return core.Artifact{}, mapErr(err, "scanning artifact")
	}
	if err := sqlb.ParseJSON(payload, &a.Payload); err != nil {
		return core.Artifact{}, core.Internal("decoding payload of artifact %q", a.ID).Wrap(err)
	}
	a.CreatedAt = scanTime(created)
	return a, nil
}

// PutArtifact creates or replaces structured output attached to a task.
func (t *tx) PutArtifact(ctx context.Context, a *core.Artifact) error {
	a.TenantID = t.scope.TenantID
	if a.ContentType == "" {
		a.ContentType = "application/json"
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = t.store.clock.Now()
	}
	payload, err := sqlb.JSONText(a.Payload, "{}")
	if err != nil {
		return core.Internal("encoding payload of artifact %q", a.Name).Wrap(err)
	}

	if a.ID != "" {
		upd := t.builder("artifacts").
			Where("id = ?", a.ID).
			Set("task_id", a.TaskID).
			Set("actor_id", a.ActorID).
			Set("kind", string(a.Kind)).
			Set("name", a.Name).
			Set("payload", payload).
			Set("content_type", a.ContentType).
			Set("blob", a.Blob)
		n, err := t.execUpdate(ctx, upd, "updating artifact %q", a.ID)
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
	}
	if a.ID == "" {
		a.ID = id.New()
	}
	ins := t.insert("artifacts").
		Set("id", a.ID).
		Set("task_id", a.TaskID).
		Set("actor_id", a.ActorID).
		Set("kind", string(a.Kind)).
		Set("name", a.Name).
		Set("payload", payload).
		Set("content_type", a.ContentType).
		Set("blob", a.Blob).
		Set("created_at", timeArg(a.CreatedAt))
	_, err = t.execInsert(ctx, ins, "creating artifact %q", a.Name)
	return err
}

// ListArtifacts returns a task's artifacts oldest first.
func (t *tx) ListArtifacts(ctx context.Context, taskID string) ([]core.Artifact, error) {
	b := t.builder("artifacts").
		Select(artifactColumns...).
		Where("task_id = ?", taskID).
		OrderBy("created_at", core.Ascending).
		OrderBy("id", core.Ascending)
	rows, err := t.query(ctx, b, "listing artifacts")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Artifact{}
	for rows.Next() {
		v, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing artifacts")
}
