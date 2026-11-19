// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerSSORoutes binds the tenant's sign-in providers and signing in
// through them.
func (rt *Router) registerSSORoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteSignInOptions, rt.handleSignInOptions)
	rt.mux.HandleFunc("POST "+wire.RouteSSOBegin, rt.handleBeginSSO)
	rt.mux.HandleFunc("POST "+wire.RouteSSOComplete, rt.handleCompleteSSO)
	rt.mux.HandleFunc("GET "+wire.RouteSAMLMetadata, rt.handleSAMLMetadata)

	rt.mux.HandleFunc("GET "+wire.RouteSSOProviders, rt.handleListSSOProviders)
	rt.mux.HandleFunc("POST "+wire.RouteSSOProviders, rt.handleCreateSSOProvider)
	rt.mux.HandleFunc("GET "+wire.RouteSSOProvider, rt.handleGetSSOProvider)
	rt.mux.HandleFunc("PATCH "+wire.RouteSSOProvider, rt.handleUpdateSSOProvider)
	rt.mux.HandleFunc("DELETE "+wire.RouteSSOProvider, rt.handleDeleteSSOProvider)
	rt.mux.HandleFunc("POST "+wire.RouteSSOProviderRefresh, rt.handleRefreshSAMLMetadata)
}

// handleSignInOptions answers what the tenant's sign-in screen offers.
func (rt *Router) handleSignInOptions(w http.ResponseWriter, r *http.Request) {
	out, err := rt.cfg.Service.SignInOptions(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// handleBeginSSO starts a sign-in through a provider.
func (rt *Router) handleBeginSSO(w http.ResponseWriter, r *http.Request) {
	var in core.BeginSSOInput
	if !readJSON(w, r, &in) {
		return
	}
	out, err := rt.cfg.Service.BeginSSOLogin(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// handleCompleteSSO finishes a sign-in through a provider. It answers as
// handleLogin does.
func (rt *Router) handleCompleteSSO(w http.ResponseWriter, r *http.Request) {
	var in core.CompleteSSOInput
	if !readJSON(w, r, &in) {
		return
	}
	result, err := rt.cfg.Service.CompleteSSOLogin(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	if result.Session != nil {
		http.SetCookie(w, auth.SessionCookie(*result.Session, rt.secureCookie(r)))
	}
	WriteJSON(w, http.StatusOK, result)
}

// handleListSSOProviders lists the tenant's sign-in providers.
func (rt *Router) handleListSSOProviders(w http.ResponseWriter, r *http.Request) {
	out, err := rt.cfg.Service.ListSSOProviders(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, out, "")
}

// handleCreateSSOProvider adds a sign-in provider.
func (rt *Router) handleCreateSSOProvider(w http.ResponseWriter, r *http.Request) {
	var in core.CreateSSOProviderInput
	if !readJSON(w, r, &in) {
		return
	}
	out, err := rt.cfg.Service.CreateSSOProvider(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, out)
}

// handleGetSSOProvider returns one sign-in provider.
func (rt *Router) handleGetSSOProvider(w http.ResponseWriter, r *http.Request) {
	out, err := rt.cfg.Service.GetSSOProvider(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// handleUpdateSSOProvider changes a sign-in provider.
func (rt *Router) handleUpdateSSOProvider(w http.ResponseWriter, r *http.Request) {
	var in core.UpdateSSOProviderInput
	if !readJSON(w, r, &in) {
		return
	}
	out, err := rt.cfg.Service.UpdateSSOProvider(r.Context(), r.PathValue("id"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// handleDeleteSSOProvider removes a sign-in provider.
func (rt *Router) handleDeleteSSOProvider(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteSSOProvider(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// samlMetadataPrefix and samlMetadataSuffix bracket the provider in the SAML
// metadata route.
var samlMetadataPrefix, samlMetadataSuffix, _ = strings.Cut(wire.RouteSAMLMetadata, "{id}")

// isSAMLMetadataPath reports whether path is a provider's SAML metadata.
func isSAMLMetadataPath(path string) bool {
	rest, ok := strings.CutPrefix(path, samlMetadataPrefix)
	if !ok {
		return false
	}
	id, ok := strings.CutSuffix(rest, samlMetadataSuffix)
	return ok && id != "" && !strings.Contains(id, "/")
}

// handleSAMLMetadata answers a SAML provider's service provider metadata,
// built on the address this request reached.
func (rt *Router) handleSAMLMetadata(w http.ResponseWriter, r *http.Request) {
	out, err := rt.cfg.Service.SAMLMetadata(r.Context(), core.SAMLMetadataInput{Provider: r.PathValue("id"),
		BaseURL: Origin(r)})
	if err != nil {
		WriteError(w, err)
		return
	}
	w.Header().Set(wire.HeaderContentType, wire.ContentSAMLMetadata)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(out.XML))
}

// handleRefreshSAMLMetadata reads a SAML provider's IdP metadata again.
func (rt *Router) handleRefreshSAMLMetadata(w http.ResponseWriter, r *http.Request) {
	out, err := rt.cfg.Service.RefreshSAMLMetadata(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}
