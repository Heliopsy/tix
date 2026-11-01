// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"

	"github.com/heliopsy/tix/internal/core"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// PutProjectIcon stores a project's icon image, replacing any it had. It
// deletes before it inserts rather than upserting, because the two engines
// spell an upsert differently and a failed insert would abort a PostgreSQL
// transaction.
func (t *tx) PutProjectIcon(ctx context.Context, img *core.ProjectIconImage) error {
	if err := t.DeleteProjectIcon(ctx, img.ProjectID); err != nil {
		return err
	}
	img.UpdatedAt = t.store.clock.Now()
	ins := t.insert("project_icons").
		Set("project_id", img.ProjectID).
		Set("content_type", img.ContentType).
		Set("digest", img.Digest).
		Set("data", img.Data).
		Set("updated_at", sqlb.TimeText(img.UpdatedAt))
	_, err := t.execInsert(ctx, ins, "storing the icon of project %q", img.ProjectID)
	return err
}

// GetProjectIcon reads a project's icon image.
func (t *tx) GetProjectIcon(ctx context.Context, projectID string) (*core.ProjectIconImage, error) {
	b := t.builder("project_icons").
		Select("project_id", "content_type", "digest", "data", "updated_at").
		Where("project_id = ?", projectID).
		Limit(1)
	q, args := b.SelectQuery()
	var (
		img     core.ProjectIconImage
		updated sql.NullString
	)
	err := t.ex.QueryRowContext(ctx, q, args...).Scan(&img.ProjectID, &img.ContentType, &img.Digest, &img.Data, &updated)
	if err != nil {
		err = mapErr(err, "reading the icon of project %q", projectID)
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("icon image of project %q", projectID)
		}
		return nil, err
	}
	if img.UpdatedAt, err = sqlb.ScanTime(updated); err != nil {
		return nil, err
	}
	return &img, nil
}

// DeleteProjectIcon removes a project's icon image, if it has one.
func (t *tx) DeleteProjectIcon(ctx context.Context, projectID string) error {
	b := t.builder("project_icons").Where("project_id = ?", projectID)
	_, err := t.execDelete(ctx, b, "removing the icon of project %q", projectID)
	return err
}
