package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
)

// ClaimTask claims one task.
func (c *Client) ClaimTask(ctx context.Context, ref core.TaskRef, in core.ClaimInput) (*core.Claim, error) {
	return call[core.Claim](ctx, c, http.MethodPost, taskPath(httpapi.RouteTaskClaim, ref), nil, in)
}

// ClaimNext claims the next eligible task.
func (c *Client) ClaimNext(ctx context.Context, in core.ClaimNextInput) (*core.Claim, error) {
	return call[core.Claim](ctx, c, http.MethodPost, httpapi.RouteClaimNext, nil, in)
}

// RenewLease extends a held lease.
func (c *Client) RenewLease(ctx context.Context, ref core.TaskRef, token string, ttl core.Duration) (*core.Claim, error) {
	body := struct {
		Token string        `json:"token"`
		TTL   core.Duration `json:"ttl,omitempty"`
	}{Token: token, TTL: ttl}
	return call[core.Claim](ctx, c, http.MethodPost, taskPath(httpapi.RouteTaskClaimRenew, ref), nil, body)
}

// ReleaseLease gives up a held lease.
func (c *Client) ReleaseLease(ctx context.Context, ref core.TaskRef, token string, in core.ReleaseInput) error {
	body := struct {
		Token string `json:"token"`
		core.ReleaseInput
	}{Token: token, ReleaseInput: in}
	return callVoid(ctx, c, http.MethodPost, taskPath(httpapi.RouteTaskClaimRelease, ref), nil, body)
}

// SweepLeases expires stale leases and reports how many it touched.
func (c *Client) SweepLeases(ctx context.Context, limit int) (int, error) {
	body := struct {
		Limit int `json:"limit,omitempty"`
	}{Limit: limit}
	out, err := call[struct {
		Swept int `json:"swept"`
	}](ctx, c, http.MethodPost, httpapi.RouteClaimSweep, nil, body)
	if err != nil {
		return 0, err
	}
	return out.Swept, nil
}
