package client

import (
	"context"
	"net/http"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
)

// PutWorkflow defines or redefines a workflow.
func (c *Client) PutWorkflow(ctx context.Context, in core.WorkflowInput) (*core.Workflow, error) {
	path := routePath(httpapi.RouteWorkflow, "key", in.Key)
	return call[core.Workflow](ctx, c, http.MethodPut, path, nil, in)
}

// GetWorkflow returns one workflow.
func (c *Client) GetWorkflow(ctx context.Context, key string) (*core.Workflow, error) {
	return call[core.Workflow](ctx, c, http.MethodGet, routePath(httpapi.RouteWorkflow, "key", key), nil, nil)
}

// ListWorkflows returns the tenant's workflows.
func (c *Client) ListWorkflows(ctx context.Context) ([]core.Workflow, error) {
	return listAll[core.Workflow](ctx, c, httpapi.RouteWorkflows, nil)
}

// DeleteWorkflow removes a workflow.
func (c *Client) DeleteWorkflow(ctx context.Context, key string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(httpapi.RouteWorkflow, "key", key), nil, nil)
}
