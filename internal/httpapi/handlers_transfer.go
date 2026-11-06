// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerTransferRoutes binds snapshot transfer and external sync.
func (rt *Router) registerTransferRoutes() {
	rt.mux.HandleFunc("POST "+wire.RouteExport, rt.handleExport)
	rt.mux.HandleFunc("POST "+wire.RouteImport, rt.handleImport)
	rt.mux.HandleFunc("POST "+wire.RouteProjectTaskExport, rt.handleTaskExport)
	rt.mux.HandleFunc("POST "+wire.RouteProjectTaskImport, rt.handleTaskImport)

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

// handleTaskExport streams one project's tasks in the format the body names.
// The status and media type are set on the first write, so an error found
// before any task is written still answers in the error envelope.
func (rt *Router) handleTaskExport(w http.ResponseWriter, r *http.Request) {
	var in core.TaskExportInput
	if !readOptionalJSON(w, r, &in) {
		return
	}
	in.ProjectRef = r.PathValue("ref")
	in = in.Normalize()
	if err := in.Validate(); err != nil {
		WriteError(w, err)
		return
	}
	out := &lazyDownload{w: w, contentType: wire.TaskFormatContent(string(in.Format)),
		filename: wire.TaskExportFilename(in.ProjectRef, in.Format.Extension())}
	if err := rt.cfg.Service.ExportTasks(r.Context(), in, out); err != nil {
		if out.started {
			return
		}
		WriteError(w, err)
	}
}

// lazyDownload sends its headers on the first byte rather than up front.
type lazyDownload struct {
	w           http.ResponseWriter
	contentType string
	filename    string
	started     bool
}

func (d *lazyDownload) Write(p []byte) (int, error) {
	if !d.started {
		d.started = true
		d.w.Header().Set(wire.HeaderContentType, d.contentType)
		d.w.Header().Set(wire.HeaderContentDisposition, `attachment; filename="`+d.filename+`"`)
		d.w.WriteHeader(http.StatusOK)
	}
	return d.w.Write(p)
}

// handleTaskImport reads a file of tasks into a project. The body is either
// JSON, the import choices with the file as content, or a multipart form with
// the file in its file part, an optional mapping part and the choices as
// fields of the same names.
func (rt *Router) handleTaskImport(w http.ResponseWriter, r *http.Request) {
	var in core.TaskImportInput
	var body io.Reader
	if mediaType(r) == wire.ContentMultipart {
		file, err := readImportForm(r, &in)
		if err != nil {
			WriteError(w, err)
			return
		}
		if file != nil {
			defer func() { _ = file.Close() }()
			body = file
		}
	} else if !readJSON(w, r, &in) {
		return
	}
	in.ProjectRef = r.PathValue("ref")
	result, err := rt.cfg.Service.ImportTasks(r.Context(), body, in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, result)
}

// mediaType is the request's content type without its parameters.
func mediaType(r *http.Request) string {
	ct, _, _ := strings.Cut(r.Header.Get(wire.HeaderContentType), ";")
	return strings.ToLower(strings.TrimSpace(ct))
}

// readImportForm reads a multipart import: the choices from its fields, the
// mapping from a mapping part or field, and the file part it returns open.
func readImportForm(r *http.Request, in *core.TaskImportInput) (multipart.File, error) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, core.Invalid("request body exceeds the configured limit")
		}
		return nil, core.Invalid("reading the upload: %v", err)
	}
	in.Format = core.TaskFormat(r.FormValue("format"))
	in.Mode = core.TaskImportMode(r.FormValue("mode"))
	in.Mapping = r.FormValue("mapping")
	in.Content = r.FormValue("content")
	in.DryRun = checkedForm(r.FormValue("dry_run"))
	if p := r.FormValue("preview"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, core.Invalid("preview %q is not a number", p).WithDetail(core.DetailField, "preview")
		}
		in.Preview = n
	}
	if parts := r.MultipartForm.File["mapping"]; len(parts) > 0 {
		raw, err := readPart(parts[0])
		if err != nil {
			return nil, err
		}
		in.Mapping = raw
	}
	parts := r.MultipartForm.File["file"]
	if len(parts) == 0 {
		return nil, nil
	}
	in.Filename = parts[0].Filename
	file, err := parts[0].Open()
	if err != nil {
		return nil, core.Invalid("reading the uploaded file: %v", err)
	}
	return file, nil
}

// readPart reads a small uploaded part, such as a mapping file.
func readPart(h *multipart.FileHeader) (string, error) {
	f, err := h.Open()
	if err != nil {
		return "", core.Invalid("reading %s: %v", h.Filename, err)
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return "", core.Invalid("reading %s: %v", h.Filename, err)
	}
	return string(raw), nil
}

// checkedForm reads a form boolean.
func checkedForm(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}
