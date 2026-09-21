package web

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// transferRoutes are the export, import and external sync screens.
func (h *handler) transferRoutes() []route {
	return []route{
		get(RouteTransfer, "transfer.html", h.showTransfer),
		post(RouteExport, h.exportSnapshot, "ExportTo"),
		post(RouteImport, h.importSnapshot, "ImportFrom"),
		get(RouteSync, "sync.html", h.showSync, "ListSyncSources"),
		post(RouteSyncSources, h.putSyncSource, "PutSyncSource"),
		post(RouteSyncDelete, h.deleteSyncSource, "DeleteSyncSource"),
		post(RouteSyncRun, h.runSync, "RunSync"),
	}
}

// importModes is the vocabulary the import form offers.
var importModes = []core.ImportMode{core.ImportMerge, core.ImportReplace}

// syncSystems is the vocabulary the sync source form offers.
var syncSystems = []string{core.SystemGeneric, core.SystemJira, core.SystemOpenProject}

// transferView is what the import and export screen renders.
type transferView struct {
	Modes  []core.ImportMode
	Result *core.ImportResult
}

// showTransfer renders the export and import forms.
func (h *handler) showTransfer(w http.ResponseWriter, r *http.Request) error {
	return h.render(w, r, "transfer.html", "Transfer", transferView{Modes: importModes})
}

// exportSnapshot streams a snapshot as a download.
func (h *handler) exportSnapshot(w http.ResponseWriter, r *http.Request) error {
	in := core.ExportInput{
		ProjectRefs:      fieldList(r, "projects"),
		IncludeArtifacts: checked(r, "include_artifacts"),
		IncludeComments:  checked(r, "include_comments"),
	}
	var buf bytes.Buffer
	if err := h.svc.ExportTo(r.Context(), in, &buf); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", `attachment; filename="tix-snapshot.ndjson"`)
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(buf.Bytes())
	return err
}

// importSnapshot applies an uploaded or pasted snapshot.
func (h *handler) importSnapshot(w http.ResponseWriter, r *http.Request) error {
	reader, err := snapshotReader(r)
	if err != nil {
		return err
	}
	in := core.ImportInput{Mode: core.ImportMode(field(r, "mode")), DryRun: checked(r, "dry_run")}
	if err := in.Validate(); err != nil {
		return err
	}
	result, err := h.svc.ImportFrom(r.Context(), reader, in)
	if err != nil {
		return err
	}
	return h.render(w, r, "transfer.html", "Transfer",
		transferView{Modes: importModes, Result: result})
}

// snapshotReader returns the uploaded file, or the pasted text when no file
// was chosen, so the screen works with or without a file picker.
func snapshotReader(r *http.Request) (io.Reader, error) {
	if r.MultipartForm != nil {
		if files := r.MultipartForm.File["snapshot"]; len(files) > 0 {
			file, err := files[0].Open()
			if err != nil {
				return nil, core.Invalid("the uploaded snapshot could not be read")
			}
			return file, nil
		}
	}
	pasted := strings.TrimSpace(r.PostFormValue("snapshot_text"))
	if pasted == "" {
		return nil, core.Invalid("choose a snapshot file or paste a snapshot")
	}
	return strings.NewReader(pasted), nil
}

// syncView is what the external sync screen renders.
type syncView struct {
	Sources []core.SyncSource
	Systems []string
	Result  *core.SyncResult
}

// showSync renders the configured external import sources.
func (h *handler) showSync(w http.ResponseWriter, r *http.Request) error {
	sources, err := h.svc.ListSyncSources(r.Context())
	if err != nil {
		return err
	}
	return h.render(w, r, "sync.html", "Sync", syncView{Sources: sources, Systems: syncSystems})
}

// putSyncSource registers or updates an external import source.
func (h *handler) putSyncSource(w http.ResponseWriter, r *http.Request) error {
	in := core.SyncSourceInput{
		ID:          field(r, "id"),
		System:      field(r, "system"),
		Name:        field(r, "name"),
		Config:      pairs(field(r, "config")),
		MappingPath: field(r, "mapping_path"),
	}
	if _, err := h.svc.PutSyncSource(r.Context(), in); err != nil {
		return err
	}
	redirect(w, r, RouteSync, "source saved")
	return nil
}

// deleteSyncSource removes an external import source.
func (h *handler) deleteSyncSource(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteSyncSource(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteSync, "source deleted")
	return nil
}

// runSync runs or refreshes an external import and shows its result.
func (h *handler) runSync(w http.ResponseWriter, r *http.Request) error {
	sources, err := h.svc.ListSyncSources(r.Context())
	if err != nil {
		return err
	}
	result, err := h.svc.RunSync(r.Context(), core.RunSyncInput{
		SourceID: field(r, "source_id"),
		DryRun:   checked(r, "dry_run"),
		Full:     checked(r, "full"),
	})
	if err != nil {
		return err
	}
	return h.render(w, r, "sync.html", "Sync",
		syncView{Sources: sources, Systems: syncSystems, Result: result})
}
