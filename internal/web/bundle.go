package web

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/thereisnotime/tix/internal/core"
)

// bundleContentType is the media type a downloaded component bundle carries.
const bundleContentType = "application/vnd.tix.bundle+json"

// defaultBundleFilename names a download whose bundle carries no usable name.
const defaultBundleFilename = "tix-bundle"

// collisionPolicies is the vocabulary the import form offers. It has no
// default: replace overwrites a component other people may be using.
var collisionPolicies = []core.CollisionPolicy{
	core.CollisionSkip, core.CollisionRename, core.CollisionReplace,
}

// bundleRoutes are the component sharing screens.
func (h *handler) bundleRoutes() []route {
	return []route{
		get(RouteBundles, "bundle.html", h.showBundles),
		post(RouteBundleExport, h.exportBundle, "ExportBundle"),
		post(RouteBundleImport, h.importBundle, "ImportBundle"),
	}
}

// bundleView is what the component sharing screen renders.
type bundleView struct {
	Kinds    []core.ComponentKind
	Policies []core.CollisionPolicy
	Result   *core.BundleResult
}

// newBundleView assembles the screen around an optional import result.
func newBundleView(result *core.BundleResult) bundleView {
	return bundleView{Kinds: core.ComponentKinds, Policies: collisionPolicies, Result: result}
}

// bundleService returns the component sharing surface the service provides.
func (h *handler) bundleService() (core.BundleService, error) {
	svc, ok := h.svc.(core.BundleService)
	if !ok {
		return nil, core.Internal("this build does not provide component sharing")
	}
	return svc, nil
}

// showBundles renders the export and import forms.
func (h *handler) showBundles(w http.ResponseWriter, r *http.Request) error {
	return h.render(w, r, "bundle.html", "Sharing", newBundleView(nil))
}

// exportBundle sends the selected components to the browser as a download.
func (h *handler) exportBundle(w http.ResponseWriter, r *http.Request) error {
	in := core.BundleExportInput{
		Name:         field(r, "name"),
		Kinds:        selectedKinds(r),
		WorkflowKeys: fieldList(r, "workflow_keys"),
		ProjectRefs:  fieldList(r, "project_refs"),
		WebhookIDs:   fieldList(r, "webhook_ids"),
	}
	if err := in.Validate(); err != nil {
		return err
	}
	svc, err := h.bundleService()
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := svc.ExportBundle(r.Context(), in, &buf); err != nil {
		return err
	}
	w.Header().Set("Content-Type", bundleContentType)
	w.Header().Set("Content-Disposition", bundleAttachment(in.Name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(buf.Bytes())
	return err
}

// importBundle applies an uploaded or pasted bundle, or previews it. The
// tenant comes from the signed-in session, never from the form or the bundle.
func (h *handler) importBundle(w http.ResponseWriter, r *http.Request) error {
	reader, err := bundleReader(r)
	if err != nil {
		return err
	}
	in := core.BundleImportInput{
		OnCollision: core.CollisionPolicy(field(r, "on_collision")),
		Preview:     checked(r, "preview"),
		ProjectRef:  field(r, "project_ref"),
	}
	if err := in.Validate(); err != nil {
		return err
	}
	svc, err := h.bundleService()
	if err != nil {
		return err
	}
	result, err := svc.ImportBundle(r.Context(), reader, in)
	if err != nil {
		return err
	}
	return h.render(w, r, "bundle.html", "Sharing", newBundleView(result))
}

// selectedKinds reads the ticked component kind checkboxes.
func selectedKinds(r *http.Request) []core.ComponentKind {
	var out []core.ComponentKind
	for _, raw := range r.PostForm["kinds"] {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			out = append(out, core.ComponentKind(trimmed))
		}
	}
	return out
}

// bundleReader returns the uploaded bundle, or the pasted text when no file
// was chosen, so the screen works with or without a file picker.
func bundleReader(r *http.Request) (io.Reader, error) {
	if r.MultipartForm != nil {
		if files := r.MultipartForm.File["bundle"]; len(files) > 0 {
			file, err := files[0].Open()
			if err != nil {
				return nil, core.Invalid("the uploaded bundle could not be read")
			}
			return file, nil
		}
	}
	pasted := strings.TrimSpace(r.PostFormValue("bundle_text"))
	if pasted == "" {
		return nil, core.Invalid("choose a bundle file or paste a bundle")
	}
	return strings.NewReader(pasted), nil
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
