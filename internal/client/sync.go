package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
)

// PutSyncSource registers or updates an external import source.
func (c *Client) PutSyncSource(ctx context.Context, in core.SyncSourceInput) (*core.SyncSource, error) {
	return call[core.SyncSource](ctx, c, http.MethodPut, httpapi.RouteSyncSources, nil, in)
}

// ListSyncSources returns the tenant's import sources.
func (c *Client) ListSyncSources(ctx context.Context) ([]core.SyncSource, error) {
	return listAll[core.SyncSource](ctx, c, httpapi.RouteSyncSources, nil)
}

// DeleteSyncSource removes an import source.
func (c *Client) DeleteSyncSource(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(httpapi.RouteSyncSource, "id", id), nil, nil)
}

// RunSync imports from a configured source.
func (c *Client) RunSync(ctx context.Context, in core.RunSyncInput) (*core.SyncResult, error) {
	return call[core.SyncResult](ctx, c, http.MethodPost, httpapi.RouteSyncRun, nil, in)
}
