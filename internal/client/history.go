// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// ListAudit returns one page of audit entries.
func (c *Client) ListAudit(ctx context.Context, f core.AuditFilter) ([]core.AuditEntry, string, error) {
	q := pageQuery(f.Page)
	if f.SubjectType != "" {
		q.Set("subject_type", f.SubjectType)
	}
	if f.SubjectID != "" {
		q.Set("subject_id", f.SubjectID)
	}
	setStrings(q, "actor_id", f.ActorIDs)
	setStrings(q, "action", f.Actions)
	for _, s := range f.Sources {
		q.Add("source", string(s))
	}
	setTime(q, "since", f.Since)
	setTime(q, "until", f.Until)
	return list[core.AuditEntry](ctx, c, wire.RouteAudit, q)
}

// Prune removes records past their retention window.
func (c *Client) Prune(ctx context.Context, in core.PruneInput) (*core.PruneResult, error) {
	return call[core.PruneResult](ctx, c, http.MethodPost, wire.RoutePrune, nil, in)
}

// GetRetention returns the tenant's retention policy.
func (c *Client) GetRetention(ctx context.Context) (*core.RetentionPolicy, error) {
	return call[core.RetentionPolicy](ctx, c, http.MethodGet, wire.RouteRetention, nil, nil)
}

// PutRetention replaces the tenant's retention policy.
func (c *Client) PutRetention(ctx context.Context, p core.RetentionPolicy) (*core.RetentionPolicy, error) {
	return call[core.RetentionPolicy](ctx, c, http.MethodPut, wire.RouteRetention, nil, p)
}
