package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// LoginRequest carries the credentials of a password login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AddMemberRequest grants an actor a role in the resolved tenant.
type AddMemberRequest struct {
	ActorID string    `json:"actor_id"`
	Role    core.Role `json:"role"`
}

// SweepRequest bounds one lease sweep.
type SweepRequest struct {
	Limit int `json:"limit,omitempty"`
}

// SweepResult reports how many leases a sweep materialized.
type SweepResult struct {
	Swept int `json:"swept"`
}

// registerAuthRoutes binds sessions, users, API tokens and enrolled SSH keys.
func (rt *Router) registerAuthRoutes() {
	rt.mux.HandleFunc("POST "+RouteLogin, rt.handleLogin)
	rt.mux.HandleFunc("POST "+RouteLogout, rt.handleLogout)

	rt.mux.HandleFunc("GET "+RouteActor, rt.handleGetActor)

	rt.mux.HandleFunc("GET "+RouteUsers, rt.handleListUsers)
	rt.mux.HandleFunc("POST "+RouteUsers, rt.handleCreateUser)
	rt.mux.HandleFunc("GET "+RouteUser, rt.handleGetUser)
	rt.mux.HandleFunc("PATCH "+RouteUser, rt.handleUpdateUser)
	rt.mux.HandleFunc("DELETE "+RouteUser, rt.handleDeleteUser)

	rt.mux.HandleFunc("GET "+RouteTokens, rt.handleListTokens)
	rt.mux.HandleFunc("POST "+RouteTokens, rt.handleCreateToken)
	rt.mux.HandleFunc("DELETE "+RouteToken, rt.handleRevokeToken)

	rt.mux.HandleFunc("GET "+RouteSSHKeys, rt.handleListSSHKeys)
	rt.mux.HandleFunc("POST "+RouteSSHKeys, rt.handleEnrolSSHKey)
	rt.mux.HandleFunc("DELETE "+RouteSSHKey, rt.handleRevokeSSHKey)
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

// handleLogin exchanges an email and password for a session.
func (rt *Router) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in LoginRequest
	if !readJSON(w, r, &in) {
		return
	}
	session, err := rt.cfg.Service.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		WriteError(w, err)
		return
	}
	http.SetCookie(w, auth.NewSessionCookie(session.Token, session.ExpiresAt, rt.secureCookie(r)))
	WriteJSON(w, http.StatusOK, session)
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
