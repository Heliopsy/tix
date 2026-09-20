package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
)

// WhoAmI returns the actor the server resolved for this credential.
func (c *Client) WhoAmI(ctx context.Context) (*core.Actor, error) {
	return call[core.Actor](ctx, c, http.MethodGet, httpapi.RouteWhoAmI, nil, nil)
}

// CreateUser creates a user.
func (c *Client) CreateUser(ctx context.Context, in core.CreateUserInput) (*core.User, error) {
	return call[core.User](ctx, c, http.MethodPost, httpapi.RouteUsers, nil, in)
}

// GetUser returns one user.
func (c *Client) GetUser(ctx context.Context, id string) (*core.User, error) {
	return call[core.User](ctx, c, http.MethodGet, routePath(httpapi.RouteUser, "id", id), nil, nil)
}

// ListUsers returns one page of users.
func (c *Client) ListUsers(ctx context.Context, page core.Page) ([]core.User, string, error) {
	return list[core.User](ctx, c, httpapi.RouteUsers, pageQuery(page))
}

// UpdateUser changes a user.
func (c *Client) UpdateUser(ctx context.Context, id string, in core.UpdateUserInput) (*core.User, error) {
	return call[core.User](ctx, c, http.MethodPatch, routePath(httpapi.RouteUser, "id", id), nil, in)
}

// DeleteUser removes a user.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(httpapi.RouteUser, "id", id), nil, nil)
}

// Login exchanges credentials for a session.
func (c *Client) Login(ctx context.Context, email, password string) (*core.Session, error) {
	body := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Email: email, Password: password}
	return call[core.Session](ctx, c, http.MethodPost, httpapi.RouteLogin, nil, body)
}

// Logout ends the current session.
func (c *Client) Logout(ctx context.Context) error {
	return callVoid(ctx, c, http.MethodPost, httpapi.RouteLogout, nil, nil)
}

// CreateToken mints an API token.
func (c *Client) CreateToken(ctx context.Context, in core.CreateTokenInput) (*core.IssuedToken, error) {
	return call[core.IssuedToken](ctx, c, http.MethodPost, httpapi.RouteTokens, nil, in)
}

// ListTokens returns an actor's API tokens.
func (c *Client) ListTokens(ctx context.Context, actorID string) ([]core.APIToken, error) {
	q := url.Values{}
	if actorID != "" {
		q.Set("actor_id", actorID)
	}
	return listAll[core.APIToken](ctx, c, httpapi.RouteTokens, q)
}

// RevokeToken revokes an API token.
func (c *Client) RevokeToken(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(httpapi.RouteToken, "id", id), nil, nil)
}
