// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// ClaimTask claims one task.
func (c *Client) ClaimTask(ctx context.Context, ref core.TaskRef, in core.ClaimInput) (*core.Claim, error) {
	return call[core.Claim](ctx, c, http.MethodPost, taskPath(wire.RouteTaskClaim, ref), nil, in)
}

// ClaimNext claims the next eligible task.
func (c *Client) ClaimNext(ctx context.Context, in core.ClaimNextInput) (*core.Claim, error) {
	return call[core.Claim](ctx, c, http.MethodPost, wire.RouteClaimNext, nil, in)
}

// RenewLease extends a held lease.
func (c *Client) RenewLease(ctx context.Context, ref core.TaskRef, token string, ttl core.Duration) (*core.Claim, error) {
	body := struct {
		Token string        `json:"token"`
		TTL   core.Duration `json:"ttl,omitempty"`
	}{Token: token, TTL: ttl}
	return call[core.Claim](ctx, c, http.MethodPost, taskPath(wire.RouteTaskClaimRenew, ref), nil, body)
}

// ReleaseLease gives up a held lease.
func (c *Client) ReleaseLease(ctx context.Context, ref core.TaskRef, token string, in core.ReleaseInput) error {
	body := struct {
		Token string `json:"token"`
		core.ReleaseInput
	}{Token: token, ReleaseInput: in}
	return callVoid(ctx, c, http.MethodPost, taskPath(wire.RouteTaskClaimRelease, ref), nil, body)
}

// SweepLeases expires stale leases and reports how many it touched.
func (c *Client) SweepLeases(ctx context.Context, limit int) (int, error) {
	body := struct {
		Limit int `json:"limit,omitempty"`
	}{Limit: limit}
	out, err := call[struct {
		Swept int `json:"swept"`
	}](ctx, c, http.MethodPost, wire.RouteClaimSweep, nil, body)
	if err != nil {
		return 0, err
	}
	return out.Swept, nil
}
