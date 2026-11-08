// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// LoginRequest carries one step of an interactive sign-in. The first step
// sends the address, the password and the surface; the second sends the
// ticket the challenge answered with and the code.
type LoginRequest struct {
	// Email identifies the user, in the first step.
	Email string `json:"email,omitempty"`
	// Password is the user's password, in the first step.
	Password string `json:"password,omitempty"`
	// NewPassword replaces the password, sent with the email and password
	// again after a password_change challenge.
	NewPassword string `json:"new_password,omitempty"`
	// Surface is the sign-in path the session is for, which decides whether a
	// second factor is required.
	Surface core.Surface `json:"surface,omitempty"`
	// Ticket is the challenge ticket from the first step, in the second step.
	Ticket string `json:"ticket,omitempty"`
	// Code is the six-digit TOTP code or a recovery code, in the second step.
	Code string `json:"code,omitempty"`
}

// TOTPCodeRequest carries the code proving possession of an authenticator.
type TOTPCodeRequest struct {
	// Code is the current code from the authenticator, or a recovery code
	// where one is accepted.
	Code string `json:"code"`
}

// AddMemberRequest grants an actor a role in the resolved tenant.
type AddMemberRequest struct {
	// ActorID is the actor to grant the role to.
	ActorID string `json:"actor_id"`
	// Role is the role to grant, which decides the member's scopes.
	Role core.Role `json:"role"`
}

// SweepRequest bounds one lease sweep.
type SweepRequest struct {
	// Limit caps how many records this pass handles; 0 or omitted means 50.
	Limit int `json:"limit,omitempty"`
}

// WakeResult reports how many snoozes a wake pass cleared.
type WakeResult struct {
	// Woken is how many expired snoozes were cleared.
	Woken int `json:"woken"`
}

// SweepResult reports how many leases a sweep materialized.
type SweepResult struct {
	// Swept is how many expired leases were cleared.
	Swept int `json:"swept"`
}

// registerAuthRoutes binds sessions, users, API tokens and enrolled SSH keys.
func (rt *Router) registerAuthRoutes() {
	rt.mux.HandleFunc("POST "+wire.RouteLogin, rt.handleLogin)
	rt.mux.HandleFunc("POST "+wire.RouteLogout, rt.handleLogout)
	rt.mux.HandleFunc("POST "+wire.RouteSwitch, rt.handleSwitch)
	rt.mux.HandleFunc("POST "+wire.RouteSSHChallenge, rt.handleSSHChallenge)
	rt.mux.HandleFunc("POST "+wire.RouteSSHVerify, rt.handleSSHLogin)

	rt.mux.HandleFunc("GET "+wire.RouteTOTP, rt.handleTOTPStatus)
	rt.mux.HandleFunc("POST "+wire.RouteTOTP, rt.handleBeginTOTP)
	rt.mux.HandleFunc("DELETE "+wire.RouteTOTP, rt.handleRemoveTOTP)
	rt.mux.HandleFunc("POST "+wire.RouteTOTPConfirm, rt.handleConfirmTOTP)
	rt.mux.HandleFunc("POST "+wire.RouteTOTPVerify, rt.handleVerifySecondFactor)
	rt.mux.HandleFunc("POST "+wire.RouteTOTPCodes, rt.handleRegenerateRecoveryCodes)
	rt.mux.HandleFunc("DELETE "+wire.RouteUserTOTP, rt.handleClearUserTOTP)

	rt.mux.HandleFunc("GET "+wire.RouteActors, rt.handleListActors)
	rt.mux.HandleFunc("GET "+wire.RouteActor, rt.handleGetActor)

	rt.mux.HandleFunc("GET "+wire.RouteUsers, rt.handleListUsers)
	rt.mux.HandleFunc("POST "+wire.RouteUsers, rt.handleCreateUser)
	rt.mux.HandleFunc("GET "+wire.RouteUser, rt.handleGetUser)
	rt.mux.HandleFunc("PATCH "+wire.RouteUser, rt.handleUpdateUser)
	rt.mux.HandleFunc("DELETE "+wire.RouteUser, rt.handleDeleteUser)

	rt.mux.HandleFunc("GET "+wire.RouteTokens, rt.handleListTokens)
	rt.mux.HandleFunc("POST "+wire.RouteTokens, rt.handleCreateToken)
	rt.mux.HandleFunc("DELETE "+wire.RouteToken, rt.handleRevokeToken)

	rt.mux.HandleFunc("GET "+wire.RouteSSHKeys, rt.handleListSSHKeys)
	rt.mux.HandleFunc("POST "+wire.RouteSSHKeys, rt.handleEnrolSSHKey)
	rt.mux.HandleFunc("DELETE "+wire.RouteSSHKey, rt.handleRevokeSSHKey)
}

// handleWhoAmI returns the authenticated actor.
func (rt *Router) handleWhoAmI(w http.ResponseWriter, r *http.Request) {
	actor, err := rt.cfg.Service.WhoAmI(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, actor)
}

// handleGetActor resolves an actor identifier to the identity behind it.
func (rt *Router) handleGetActor(w http.ResponseWriter, r *http.Request) {
	actor, err := rt.cfg.Service.GetActor(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, actor)
}

// handleListActors returns a page of this tenant's actors.
func (rt *Router) handleListActors(w http.ResponseWriter, r *http.Request) {
	page, err := pageFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	actors, next, err := rt.cfg.Service.ListActors(r.Context(), page)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, actors, next)
}

// handleLogin performs one step of a sign-in. A result carrying a challenge
// is a complete answer with a 200, not an error: the credentials were right
// and the caller has one more thing to send.
func (rt *Router) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in LoginRequest
	if !readJSON(w, r, &in) {
		return
	}
	result, err := rt.cfg.Service.Login(r.Context(), core.LoginInput{
		Email:       in.Email,
		Password:    in.Password,
		NewPassword: in.NewPassword,
		Surface:     in.Surface,
		Ticket:      in.Ticket,
		Code:        in.Code,
	})
	if err != nil {
		WriteError(w, err)
		return
	}
	if result.Session != nil {
		http.SetCookie(w, auth.NewSessionCookie(
			result.Session.Token, result.Session.ExpiresAt, rt.secureCookie(r)))
	}
	WriteJSON(w, http.StatusOK, result)
}

// handleSSHChallenge issues a nonce bound to the host the request names, the
// same value that resolved the tenant.
func (rt *Router) handleSSHChallenge(w http.ResponseWriter, r *http.Request) {
	challenge, err := rt.cfg.Service.SSHLoginChallenge(core.WithAudience(r.Context(), hostOf(r)))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, challenge)
}

// handleSSHLogin verifies a signed challenge. It answers as handleLogin does.
func (rt *Router) handleSSHLogin(w http.ResponseWriter, r *http.Request) {
	var in core.SSHLoginInput
	if !readJSON(w, r, &in) {
		return
	}
	result, err := rt.cfg.Service.SSHLogin(core.WithAudience(r.Context(), hostOf(r)), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	if result.Session != nil {
		http.SetCookie(w, auth.NewSessionCookie(
			result.Session.Token, result.Session.ExpiresAt, rt.secureCookie(r)))
	}
	WriteJSON(w, http.StatusOK, result)
}

// handleTOTPStatus reports the caller's own enrolment.
func (rt *Router) handleTOTPStatus(w http.ResponseWriter, r *http.Request) {
	status, err := rt.cfg.Service.TOTPStatus(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, status)
}

// handleBeginTOTP starts an enrolment, which takes effect only once confirmed.
func (rt *Router) handleBeginTOTP(w http.ResponseWriter, r *http.Request) {
	enrolment, err := rt.cfg.Service.BeginTOTPEnrolment(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, enrolment)
}

// handleConfirmTOTP proves possession and returns the recovery codes once.
func (rt *Router) handleConfirmTOTP(w http.ResponseWriter, r *http.Request) {
	var in TOTPCodeRequest
	if !readJSON(w, r, &in) {
		return
	}
	codes, err := rt.cfg.Service.ConfirmTOTPEnrolment(r.Context(), in.Code)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, codes)
}

// handleVerifySecondFactor proves an existing factor and mints nothing.
func (rt *Router) handleVerifySecondFactor(w http.ResponseWriter, r *http.Request) {
	var in TOTPCodeRequest
	if !readJSON(w, r, &in) {
		return
	}
	if err := rt.cfg.Service.VerifySecondFactor(r.Context(), in.Code); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleRemoveTOTP drops the caller's own enrolment.
func (rt *Router) handleRemoveTOTP(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RemoveTOTP(r.Context()); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleRegenerateRecoveryCodes replaces the caller's codes.
func (rt *Router) handleRegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	codes, err := rt.cfg.Service.RegenerateRecoveryCodes(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, codes)
}

// handleClearUserTOTP returns another user to an unenrolled state.
func (rt *Router) handleClearUserTOTP(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.ClearUserTOTP(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleLogout ends the current session.
func (rt *Router) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.Logout(r.Context()); err != nil {
		WriteError(w, err)
		return
	}
	http.SetCookie(w, auth.ClearSessionCookie(rt.secureCookie(r)))
	writeNoContent(w)
}

// handleSwitch moves the caller's session to another tenant on this host. A
// result carrying a code challenge is a 200, as a sign-in's is.
func (rt *Router) handleSwitch(w http.ResponseWriter, r *http.Request) {
	var in core.SwitchTenantInput
	if !readJSON(w, r, &in) {
		return
	}
	result, err := rt.cfg.Service.SwitchTenant(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	if result.Session != nil {
		http.SetCookie(w, auth.NewSessionCookie(
			result.Session.Token, result.Session.ExpiresAt, rt.secureCookie(r)))
	}
	WriteJSON(w, http.StatusOK, result)
}

// handleListUsers returns a page of users.
func (rt *Router) handleListUsers(w http.ResponseWriter, r *http.Request) {
	page, err := pageFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	users, next, err := rt.cfg.Service.ListUsers(r.Context(), page)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, users, next)
}

// handleCreateUser creates a user.
func (rt *Router) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var in core.CreateUserInput
	if !readJSON(w, r, &in) {
		return
	}
	user, err := rt.cfg.Service.CreateUser(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, user)
}

// handleGetUser returns one user.
func (rt *Router) handleGetUser(w http.ResponseWriter, r *http.Request) {
	user, err := rt.cfg.Service.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, user)
}

// handleUpdateUser changes one user.
func (rt *Router) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	var in core.UpdateUserInput
	if !readJSON(w, r, &in) {
		return
	}
	user, err := rt.cfg.Service.UpdateUser(r.Context(), r.PathValue("id"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, user)
}

// handleDeleteUser removes one user.
func (rt *Router) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteUser(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleListTokens returns the tokens of one actor.
func (rt *Router) handleListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := rt.cfg.Service.ListTokens(r.Context(), r.URL.Query().Get("actor_id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, tokens, "")
}

// handleCreateToken mints an API token, returning its value exactly once.
func (rt *Router) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var in core.CreateTokenInput
	if !readJSON(w, r, &in) {
		return
	}
	token, err := rt.cfg.Service.CreateToken(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, token)
}

// handleRevokeToken revokes an API token.
func (rt *Router) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RevokeToken(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleListSSHKeys returns the enrolled keys of one actor, revoked ones
// included, because a key that stopped working is what an operator looks for.
func (rt *Router) handleListSSHKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := rt.cfg.Service.ListSSHKeys(r.Context(), r.URL.Query().Get("actor_id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, keys, "")
}

// handleEnrolSSHKey enrols a public key against an actor.
func (rt *Router) handleEnrolSSHKey(w http.ResponseWriter, r *http.Request) {
	var in core.EnrolSSHKeyInput
	if !readJSON(w, r, &in) {
		return
	}
	key, err := rt.cfg.Service.EnrolSSHKey(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, key)
}

// handleRevokeSSHKey stops an enrolled key authenticating.
func (rt *Router) handleRevokeSSHKey(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RevokeSSHKey(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
