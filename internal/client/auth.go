package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// WhoAmI returns the actor the server resolved for this credential.
func (c *Client) WhoAmI(ctx context.Context) (*core.Actor, error) {
	return call[core.Actor](ctx, c, http.MethodGet, wire.RouteWhoAmI, nil, nil)
}

// GetActor resolves an actor identifier to the identity behind it.
func (c *Client) GetActor(ctx context.Context, id string) (*core.Actor, error) {
	return call[core.Actor](ctx, c, http.MethodGet, routePath(wire.RouteActor, "id", id), nil, nil)
}

// CreateUser creates a user.
func (c *Client) CreateUser(ctx context.Context, in core.CreateUserInput) (*core.User, error) {
	return call[core.User](ctx, c, http.MethodPost, wire.RouteUsers, nil, in)
}

// GetUser returns one user.
func (c *Client) GetUser(ctx context.Context, id string) (*core.User, error) {
	return call[core.User](ctx, c, http.MethodGet, routePath(wire.RouteUser, "id", id), nil, nil)
}

// ListUsers returns one page of users.
func (c *Client) ListUsers(ctx context.Context, page core.Page) ([]core.User, string, error) {
	return list[core.User](ctx, c, wire.RouteUsers, pageQuery(page))
}

// UpdateUser changes a user.
func (c *Client) UpdateUser(ctx context.Context, id string, in core.UpdateUserInput) (*core.User, error) {
	return call[core.User](ctx, c, http.MethodPatch, routePath(wire.RouteUser, "id", id), nil, in)
}

// DeleteUser removes a user.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteUser, "id", id), nil, nil)
}

// Login exchanges credentials for a session.
func (c *Client) Login(ctx context.Context, email, password string) (*core.Session, error) {
	body := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Email: email, Password: password}
	return call[core.Session](ctx, c, http.MethodPost, wire.RouteLogin, nil, body)
}

// Logout ends the current session.
func (c *Client) Logout(ctx context.Context) error {
	return callVoid(ctx, c, http.MethodPost, wire.RouteLogout, nil, nil)
}

// CreateToken mints an API token.
func (c *Client) CreateToken(ctx context.Context, in core.CreateTokenInput) (*core.IssuedToken, error) {
	return call[core.IssuedToken](ctx, c, http.MethodPost, wire.RouteTokens, nil, in)
}

// ListTokens returns an actor's API tokens.
func (c *Client) ListTokens(ctx context.Context, actorID string) ([]core.APIToken, error) {
	q := url.Values{}
	if actorID != "" {
		q.Set("actor_id", actorID)
	}
	return listAll[core.APIToken](ctx, c, wire.RouteTokens, q)
}

// RevokeToken revokes an API token.
func (c *Client) RevokeToken(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteToken, "id", id), nil, nil)
}
