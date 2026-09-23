// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// defaultBundleFilename names a download whose bundle carries no usable name.
const defaultBundleFilename = "tix-bundle"

// registerBundleRoutes binds component sharing.
func (rt *Router) registerBundleRoutes() {
	rt.mux.HandleFunc("POST "+wire.RouteBundleExport, rt.handleBundleExport)
	rt.mux.HandleFunc("POST "+wire.RouteBundleImport, rt.handleBundleImport)
}

// handleBundleExport streams a component bundle as the service produces it.
func (rt *Router) handleBundleExport(w http.ResponseWriter, r *http.Request) {
	var in core.BundleExportInput
	if !readOptionalJSON(w, r, &in) {
		return
	}
	if err := in.Validate(); err != nil {
		WriteError(w, err)
		return
	}
	w.Header().Set(wire.HeaderContentType, wire.ContentBundle)
	w.Header().Set(wire.HeaderContentDisposition, bundleAttachment(in.Name))
	if err := rt.cfg.Service.ExportBundle(r.Context(), in, w); err != nil {
		w.Header().Del(wire.HeaderContentType)
		w.Header().Del(wire.HeaderContentDisposition)
		WriteError(w, err)
		return
	}
}

// handleBundleImport reads a bundle from the request body and applies it. The
// tenant comes from the request context, never from the bundle.
func (rt *Router) handleBundleImport(w http.ResponseWriter, r *http.Request) {
	in := core.BundleImportInput{
		OnCollision: core.CollisionPolicy(strings.TrimSpace(r.URL.Query().Get("on_collision"))),
		Preview:     boolParam(r, "preview"),
		ProjectRef:  strings.TrimSpace(r.URL.Query().Get("project_ref")),
	}
	if err := in.Validate(); err != nil {
		WriteError(w, err)
		return
	}
	result, err := rt.cfg.Service.ImportBundle(r.Context(), r.Body, in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, result)
}

// bundleAttachment renders the download header for a bundle, keeping only
// characters a header may carry so a bundle name cannot forge one.
func bundleAttachment(name string) string {
	return `attachment; filename="` + bundleFilename(name) + `.ndjson"`
}

// bundleFilename reduces a bundle name to a safe file name stem.
func bundleFilename(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ' ':
			b.WriteRune('-')
		}
	}
	stem := strings.Trim(b.String(), "-")
	if stem == "" {
		return defaultBundleFilename
	}
	return defaultBundleFilename + "-" + stem
}
