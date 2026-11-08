// SPDX-License-Identifier: AGPL-3.0-or-later

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

// ListActors returns one page of this tenant's actors.
func (c *Client) ListActors(ctx context.Context, page core.Page) ([]core.Actor, string, error) {
	return list[core.Actor](ctx, c, wire.RouteActors, pageQuery(page))
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
func (c *Client) ListUsers(ctx context.Context, f core.UserFilter) ([]core.User, string, error) {
	return list[core.User](ctx, c, wire.RouteUsers, userQuery(f))
}

// UpdateUser changes a user.
func (c *Client) UpdateUser(ctx context.Context, id string, in core.UpdateUserInput) (*core.User, error) {
	return call[core.User](ctx, c, http.MethodPatch, routePath(wire.RouteUser, "id", id), nil, in)
}

// DeleteUser removes a user.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteUser, "id", id), nil, nil)
}

// GrantGlobalAdmin makes a user a global admin.
func (c *Client) GrantGlobalAdmin(ctx context.Context, in core.GlobalAdminInput) (*core.User, error) {
	return call[core.User](ctx, c, http.MethodPost, wire.RouteGlobalAdmins, nil, in)
}

// RevokeGlobalAdmin takes global admin away from a user.
func (c *Client) RevokeGlobalAdmin(ctx context.Context, email string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteGlobalAdmin, "email", email), nil, nil)
}

// ListGlobalAdmins lists the global admins.
func (c *Client) ListGlobalAdmins(ctx context.Context) ([]core.User, error) {
	out, _, err := list[core.User](ctx, c, wire.RouteGlobalAdmins, nil)
	return out, err
}

// Login performs one step of a sign-in. The client marshals and nothing else:
// which step this is, and what to do with a challenge, belongs to the caller.
func (c *Client) Login(ctx context.Context, in core.LoginInput) (*core.LoginResult, error) {
	return call[core.LoginResult](ctx, c, http.MethodPost, wire.RouteLogin, nil, in)
}

// SSHLoginChallenge asks the server for a nonce to sign.
func (c *Client) SSHLoginChallenge(ctx context.Context) (*core.SSHChallenge, error) {
	return call[core.SSHChallenge](ctx, c, http.MethodPost, wire.RouteSSHChallenge, nil, nil)
}

// SSHLogin presents a signed challenge.
func (c *Client) SSHLogin(ctx context.Context, in core.SSHLoginInput) (*core.LoginResult, error) {
	return call[core.LoginResult](ctx, c, http.MethodPost, wire.RouteSSHVerify, nil, in)
}

// TOTPStatus reports the caller's own second factor.
func (c *Client) TOTPStatus(ctx context.Context) (*core.TOTPStatus, error) {
	return call[core.TOTPStatus](ctx, c, http.MethodGet, wire.RouteTOTP, nil, nil)
}

// BeginTOTPEnrolment starts an enrolment.
func (c *Client) BeginTOTPEnrolment(ctx context.Context) (*core.TOTPEnrolment, error) {
	return call[core.TOTPEnrolment](ctx, c, http.MethodPost, wire.RouteTOTP, nil, nil)
}

// ConfirmTOTPEnrolment proves possession and returns the recovery codes once.
func (c *Client) ConfirmTOTPEnrolment(ctx context.Context, code string) (*core.TOTPRecoveryCodes, error) {
	body := struct {
		Code string `json:"code"`
	}{Code: code}
	return call[core.TOTPRecoveryCodes](ctx, c, http.MethodPost, wire.RouteTOTPConfirm, nil, body)
}

// VerifySecondFactor proves the caller's second factor on its own.
func (c *Client) VerifySecondFactor(ctx context.Context, code string) error {
	body := struct {
		Code string `json:"code"`
	}{Code: code}
	return callVoid(ctx, c, http.MethodPost, wire.RouteTOTPVerify, nil, body)
}

// RemoveTOTP drops the caller's own enrolment.
func (c *Client) RemoveTOTP(ctx context.Context) error {
	return callVoid(ctx, c, http.MethodDelete, wire.RouteTOTP, nil, nil)
}

// RegenerateRecoveryCodes replaces the caller's codes.
func (c *Client) RegenerateRecoveryCodes(ctx context.Context) (*core.TOTPRecoveryCodes, error) {
	return call[core.TOTPRecoveryCodes](ctx, c, http.MethodPost, wire.RouteTOTPCodes, nil, nil)
}

// ClearUserTOTP returns another user to an unenrolled state.
func (c *Client) ClearUserTOTP(ctx context.Context, userID string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteUserTOTP, "id", userID), nil, nil)
}

// Logout ends the current session.
func (c *Client) Logout(ctx context.Context) error {
	return callVoid(ctx, c, http.MethodPost, wire.RouteLogout, nil, nil)
}

// SwitchTenant moves the caller's session to another tenant on the same host.
func (c *Client) SwitchTenant(ctx context.Context, in core.SwitchTenantInput) (*core.LoginResult, error) {
	return call[core.LoginResult](ctx, c, http.MethodPost, wire.RouteSwitch, nil, in)
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

// RotateToken replaces an API token with a new one and returns its value once.
func (c *Client) RotateToken(ctx context.Context, id string, in core.RotateTokenInput) (*core.IssuedToken, error) {
	return call[core.IssuedToken](ctx, c, http.MethodPost, routePath(wire.RouteTokenRotate, "id", id), nil, in)
}

// ListSessions returns one page of the caller's own sessions.
func (c *Client) ListSessions(ctx context.Context, page core.Page) ([]core.SessionInfo, string, error) {
	return list[core.SessionInfo](ctx, c, wire.RouteAccountSessions, pageQuery(page))
}

// RevokeSession ends one of the caller's own sessions.
func (c *Client) RevokeSession(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteAccountSession, "id", id), nil, nil)
}
