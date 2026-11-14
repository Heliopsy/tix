// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"

	"github.com/heliopsy/tix/internal/core"
)

// PutTenantIcon stores a tenant's icon image on its row, replacing any it had.
// The digest is the row's icon_image, which UpdateTenant writes.
func (t *tx) PutTenantIcon(ctx context.Context, img *core.TenantIconImage) error {
	img.UpdatedAt = t.store.clock.Now()
	b := t.builder("tenants").
		Where("id = ?", img.TenantID).
		Set("icon_type", img.ContentType).
		Set("icon_data", img.Data).
		Set("icon_updated_at", timeArg(img.UpdatedAt))
	n, err := t.execUpdate(ctx, b, "storing the icon of tenant %q", img.TenantID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("tenant %q", img.TenantID)
	}
	return nil
}

// GetTenantIcon reads a tenant's icon image.
func (t *tx) GetTenantIcon(ctx context.Context, tenantID string) (*core.TenantIconImage, error) {
	b := t.builder("tenants").
		Select("id", "icon_type", "icon_image", "icon_data", "icon_updated_at").
		Where("id = ?", tenantID).
		Where("icon_data IS NOT NULL").
		Limit(1)
	q, args := b.SelectQuery()
	var (
		img     core.TenantIconImage
		updated sql.NullTime
	)
	err := t.ex.QueryRowContext(ctx, q, args...).Scan(&img.TenantID, &img.ContentType, &img.Digest, &img.Data, &updated)
	if err != nil {
		err = mapErr(err, "reading the icon of tenant %q", tenantID)
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("icon image of tenant %q", tenantID)
		}
		return nil, err
	}
	img.UpdatedAt = scanTime(updated)
	return &img, nil
}

// DeleteTenantIcon removes a tenant's icon image, if it has one.
func (t *tx) DeleteTenantIcon(ctx context.Context, tenantID string) error {
	b := t.builder("tenants").
		Where("id = ?", tenantID).
		Set("icon_type", "").
		Set("icon_data", nil).
		Set("icon_updated_at", nil)
	_, err := t.execUpdate(ctx, b, "removing the icon of tenant %q", tenantID)
	return err
}
