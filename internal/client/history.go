package client

import (
	"context"
	"net/http"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
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
	return list[core.AuditEntry](ctx, c, httpapi.RouteAudit, q)
}

// Prune removes records past their retention window.
func (c *Client) Prune(ctx context.Context, in core.PruneInput) (*core.PruneResult, error) {
	return call[core.PruneResult](ctx, c, http.MethodPost, httpapi.RoutePrune, nil, in)
}

// GetRetention returns the tenant's retention policy.
func (c *Client) GetRetention(ctx context.Context) (*core.RetentionPolicy, error) {
	return call[core.RetentionPolicy](ctx, c, http.MethodGet, httpapi.RouteRetention, nil, nil)
}

// PutRetention replaces the tenant's retention policy.
func (c *Client) PutRetention(ctx context.Context, p core.RetentionPolicy) (*core.RetentionPolicy, error) {
	return call[core.RetentionPolicy](ctx, c, http.MethodPut, httpapi.RouteRetention, nil, p)
}
