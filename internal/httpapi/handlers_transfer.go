package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerTransferRoutes binds snapshot transfer and external sync.
func (rt *Router) registerTransferRoutes() {
	rt.mux.HandleFunc("POST "+wire.RouteExport, rt.handleExport)
	rt.mux.HandleFunc("POST "+wire.RouteImport, rt.handleImport)

	rt.mux.HandleFunc("GET "+wire.RouteSyncSources, rt.handleListSyncSources)
	rt.mux.HandleFunc("PUT "+wire.RouteSyncSources, rt.handlePutSyncSource)
	rt.mux.HandleFunc("DELETE "+wire.RouteSyncSource, rt.handleDeleteSyncSource)
	rt.mux.HandleFunc("POST "+wire.RouteSyncRun, rt.handleRunSync)
}

// handleExport streams a snapshot as ndjson.
func (rt *Router) handleExport(w http.ResponseWriter, r *http.Request) {
	var in core.ExportInput
	if !readOptionalJSON(w, r, &in) {
		return
	}
	w.Header().Set(wire.HeaderContentType, wire.ContentNDJSON)
	if err := rt.cfg.Service.ExportTo(r.Context(), in, w); err != nil {
		w.Header().Del(wire.HeaderContentType)
		WriteError(w, err)
		return
	}
}

// handleImport reads a streamed snapshot from the request body.
func (rt *Router) handleImport(w http.ResponseWriter, r *http.Request) {
	in := core.ImportInput{
		Mode:   core.ImportMode(r.URL.Query().Get("mode")),
		DryRun: boolParam(r, "dry_run"),
	}
	if err := in.Validate(); err != nil {
		WriteError(w, err)
		return
	}
	result, err := rt.cfg.Service.ImportFrom(r.Context(), r.Body, in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, result)
}

// handleListSyncSources returns the tenant's external import sources.
func (rt *Router) handleListSyncSources(w http.ResponseWriter, r *http.Request) {
	sources, err := rt.cfg.Service.ListSyncSources(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, sources, "")
}

// handlePutSyncSource registers or updates an external import source.
func (rt *Router) handlePutSyncSource(w http.ResponseWriter, r *http.Request) {
	var in core.SyncSourceInput
	if !readJSON(w, r, &in) {
		return
	}
	source, err := rt.cfg.Service.PutSyncSource(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, source)
}

// handleDeleteSyncSource removes an external import source.
func (rt *Router) handleDeleteSyncSource(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteSyncSource(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleRunSync runs an import from a configured source.
func (rt *Router) handleRunSync(w http.ResponseWriter, r *http.Request) {
	var in core.RunSyncInput
	if !readJSON(w, r, &in) {
		return
	}
	result, err := rt.cfg.Service.RunSync(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, result)
}
