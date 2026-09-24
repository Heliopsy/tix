// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// transferRoutes are the snapshot export and import screens. The external
// sync screens live in sync.go.
func (h *handler) transferRoutes() []route {
	return []route{
		get(RouteTransfer, "transfer.html", h.showTransfer),
		post(RouteExport, h.exportSnapshot, "ExportTo"),
		post(RouteImport, h.importSnapshot, "ImportFrom"),
	}
}

// transferView is what the import and export screen renders.
type transferView struct {
	Modes  []core.ImportMode
	Result *core.ImportResult
}

// showTransfer renders the export and import forms.
func (h *handler) showTransfer(w http.ResponseWriter, r *http.Request) error {
	return h.render(w, r, "transfer.html", "Transfer", transferView{Modes: core.ImportModes})
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
		transferView{Modes: core.ImportModes, Result: result})
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
