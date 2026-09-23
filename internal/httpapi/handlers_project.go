// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerProjectRoutes binds projects, their field definitions and workflows.
func (rt *Router) registerProjectRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteProjects, rt.handleListProjects)
	rt.mux.HandleFunc("POST "+wire.RouteProjects, rt.handleCreateProject)
	rt.mux.HandleFunc("GET "+wire.RouteProject, rt.handleGetProject)
	rt.mux.HandleFunc("PATCH "+wire.RouteProject, rt.handleUpdateProject)
	rt.mux.HandleFunc("DELETE "+wire.RouteProject, rt.handleDeleteProject)

	rt.mux.HandleFunc("GET "+wire.RouteProjectFields, rt.handleListFieldDefs)
	rt.mux.HandleFunc("PUT "+wire.RouteProjectFields, rt.handlePutFieldDef)
	rt.mux.HandleFunc("DELETE "+wire.RouteProjectField, rt.handleDeleteFieldDef)

	rt.mux.HandleFunc("GET "+wire.RouteWorkflows, rt.handleListWorkflows)
	rt.mux.HandleFunc("PUT "+wire.RouteWorkflows, rt.handlePutWorkflow)
	rt.mux.HandleFunc("GET "+wire.RouteWorkflow, rt.handleGetWorkflow)
	rt.mux.HandleFunc("PUT "+wire.RouteWorkflow, rt.handlePutWorkflowByKey)
	rt.mux.HandleFunc("DELETE "+wire.RouteWorkflow, rt.handleDeleteWorkflow)
}

// handleListProjects returns a page of projects.
func (rt *Router) handleListProjects(w http.ResponseWriter, r *http.Request) {
	page, err := pageFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	f := core.ProjectFilter{
		Keys:            r.URL.Query()["key"],
		IncludeArchived: boolParam(r, "include_archived"),
		Page:            page,
	}
	projects, next, err := rt.cfg.Service.ListProjects(r.Context(), f)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, projects, next)
}

// handleCreateProject creates a project.
func (rt *Router) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var in core.CreateProjectInput
	if !readJSON(w, r, &in) {
		return
	}
	project, err := rt.cfg.Service.CreateProject(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, project)
}

// handleGetProject returns one project.
func (rt *Router) handleGetProject(w http.ResponseWriter, r *http.Request) {
	project, err := rt.cfg.Service.GetProject(r.Context(), r.PathValue("ref"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, project)
}

// handleUpdateProject changes one project.
func (rt *Router) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	var in core.UpdateProjectInput
	if !readJSON(w, r, &in) {
		return
	}
	project, err := rt.cfg.Service.UpdateProject(r.Context(), r.PathValue("ref"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, project)
}

// handleDeleteProject deletes a project, or archives it when asked to.
func (rt *Router) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	ref := r.PathValue("ref")
	var err error
	if boolParam(r, "archive") {
		err = rt.cfg.Service.ArchiveProject(r.Context(), ref)
	} else {
		err = rt.cfg.Service.DeleteProject(r.Context(), ref)
	}
	if err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleListFieldDefs returns a project's custom field definitions.
func (rt *Router) handleListFieldDefs(w http.ResponseWriter, r *http.Request) {
	defs, err := rt.cfg.Service.ListFieldDefs(r.Context(), r.PathValue("ref"))
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, defs, "")
}

// handlePutFieldDef defines or redefines a custom field.
func (rt *Router) handlePutFieldDef(w http.ResponseWriter, r *http.Request) {
	var in core.FieldDefInput
	if !readJSON(w, r, &in) {
		return
	}
	def, err := rt.cfg.Service.PutFieldDef(r.Context(), r.PathValue("ref"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, def)
}

// handleDeleteFieldDef removes a custom field definition.
func (rt *Router) handleDeleteFieldDef(w http.ResponseWriter, r *http.Request) {
	err := rt.cfg.Service.DeleteFieldDef(r.Context(), r.PathValue("ref"), r.PathValue("key"))
	if err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleListWorkflows returns every workflow in the resolved tenant.
func (rt *Router) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows, err := rt.cfg.Service.ListWorkflows(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, workflows, "")
}

// handlePutWorkflow defines or redefines a workflow named in the body.
func (rt *Router) handlePutWorkflow(w http.ResponseWriter, r *http.Request) {
	rt.putWorkflow(w, r, "")
}

// handlePutWorkflowByKey defines or redefines the workflow named in the path.
func (rt *Router) handlePutWorkflowByKey(w http.ResponseWriter, r *http.Request) {
	rt.putWorkflow(w, r, r.PathValue("key"))
}

// putWorkflow upserts a workflow, preferring the key from the path.
func (rt *Router) putWorkflow(w http.ResponseWriter, r *http.Request, key string) {
	var in core.WorkflowInput
	if !readJSON(w, r, &in) {
		return
	}
	if key != "" {
		in.Key = key
	}
	workflow, err := rt.cfg.Service.PutWorkflow(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, workflow)
}

// handleGetWorkflow returns one workflow.
func (rt *Router) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	workflow, err := rt.cfg.Service.GetWorkflow(r.Context(), r.PathValue("key"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, workflow)
}

// handleDeleteWorkflow removes one workflow.
func (rt *Router) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteWorkflow(r.Context(), r.PathValue("key")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
