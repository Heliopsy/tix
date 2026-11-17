// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerSCIMRoutes binds the administration of SCIM provisioning. The SCIM
// protocol itself is served by Config.SCIM under core.SCIMPath.
func (rt *Router) registerSCIMRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteSCIM, rt.handleGetSCIM)
	rt.mux.HandleFunc("POST "+wire.RouteSCIMEnable, rt.handleEnableSCIM)
	rt.mux.HandleFunc("POST "+wire.RouteSCIMDisable, rt.handleDisableSCIM)
	rt.mux.HandleFunc("POST "+wire.RouteSCIMToken, rt.handleRotateSCIMToken)
	rt.mux.HandleFunc("PUT "+wire.RouteSCIMGroups, rt.handleMapSCIMGroup)
	rt.mux.HandleFunc("DELETE "+wire.RouteSCIMGroup, rt.handleUnmapSCIMGroup)
}

// writeSCIM answers with the settings, the endpoint made absolute under the
// address this request reached.
func writeSCIM(w http.ResponseWriter, r *http.Request, s *core.SCIMSettings, err error) {
	if err != nil {
		WriteError(w, err)
		return
	}
	if s.URL == "" {
		s.URL = Origin(r) + s.Path
	}
	WriteJSON(w, http.StatusOK, s)
}

func (rt *Router) handleGetSCIM(w http.ResponseWriter, r *http.Request) {
	s, err := rt.cfg.Service.GetSCIM(r.Context())
	writeSCIM(w, r, s, err)
}

func (rt *Router) handleEnableSCIM(w http.ResponseWriter, r *http.Request) {
	s, err := rt.cfg.Service.EnableSCIM(r.Context())
	writeSCIM(w, r, s, err)
}

func (rt *Router) handleDisableSCIM(w http.ResponseWriter, r *http.Request) {
	s, err := rt.cfg.Service.DisableSCIM(r.Context())
	writeSCIM(w, r, s, err)
}

func (rt *Router) handleRotateSCIMToken(w http.ResponseWriter, r *http.Request) {
	s, err := rt.cfg.Service.RotateSCIMToken(r.Context())
	writeSCIM(w, r, s, err)
}

func (rt *Router) handleMapSCIMGroup(w http.ResponseWriter, r *http.Request) {
	var in core.SCIMGroupRoleInput
	if !readJSON(w, r, &in) {
		return
	}
	m, err := rt.cfg.Service.MapSCIMGroup(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, m)
}

func (rt *Router) handleUnmapSCIMGroup(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.UnmapSCIMGroup(r.Context(), r.PathValue("group")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
