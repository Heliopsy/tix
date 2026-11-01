// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerTemplateRoutes binds task templates.
func (rt *Router) registerTemplateRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteTemplates, rt.handleListTemplates)
	rt.mux.HandleFunc("POST "+wire.RouteTemplates, rt.handleSaveTemplate)
	rt.mux.HandleFunc("GET "+wire.RouteTemplate, rt.handleGetTemplate)
	rt.mux.HandleFunc("PATCH "+wire.RouteTemplate, rt.handleUpdateTemplate)
	rt.mux.HandleFunc("DELETE "+wire.RouteTemplate, rt.handleDeleteTemplate)
	rt.mux.HandleFunc("POST "+wire.RouteTemplateUse, rt.handleUseTemplate)
}

// handleListTemplates lists the templates the caller can read.
func (rt *Router) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	page, err := pageFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	f := core.TemplateFilter{ProjectRef: r.URL.Query().Get("project"), Page: page}
	items, next, err := rt.cfg.Service.ListTemplates(r.Context(), f)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, items, next)
}

// handleSaveTemplate keeps a task as a template.
func (rt *Router) handleSaveTemplate(w http.ResponseWriter, r *http.Request) {
	var in core.SaveTemplateInput
	if !readJSON(w, r, &in) {
		return
	}
	tpl, err := rt.cfg.Service.SaveTemplate(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, tpl)
}

// handleGetTemplate reads one template by name or identifier.
func (rt *Router) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	tpl, err := rt.cfg.Service.GetTemplate(r.Context(), r.PathValue("ref"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, tpl)
}

// handleUpdateTemplate renames, rescopes or edits a template.
func (rt *Router) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	var in core.UpdateTemplateInput
	if !readJSON(w, r, &in) {
		return
	}
	tpl, err := rt.cfg.Service.UpdateTemplate(r.Context(), r.PathValue("ref"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, tpl)
}

// handleDeleteTemplate removes a template.
func (rt *Router) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteTemplate(r.Context(), r.PathValue("ref")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleUseTemplate creates a task from a template and answers with it.
func (rt *Router) handleUseTemplate(w http.ResponseWriter, r *http.Request) {
	var in core.UseTemplateInput
	if !readOptionalJSON(w, r, &in) {
		return
	}
	task, err := rt.cfg.Service.UseTemplate(r.Context(), r.PathValue("ref"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, task)
}
