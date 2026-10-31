// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"io"
	"net/http"
	"strconv"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// Icon kinds the project forms submit as icon_kind.
const (
	iconKindNone    = "none"
	iconKindText    = "text"
	iconKindBuiltin = "builtin"
	iconKindImage   = "image"
)

// builtinIconLabels names each built-in icon in English, the catalog key the
// picker translates.
var builtinIconLabels = map[string]string{
	"folder": "Folder", "briefcase": "Briefcase", "home": "Home", "server": "Server",
	"database": "Database", "cloud": "Cloud", "code": "Code", "terminal": "Terminal",
	"bug": "Bug", "rocket": "Rocket", "shield": "Shield", "lock": "Lock",
	"globe": "Globe", "chart": "Chart", "book": "Book", "users": "People",
	"package": "Package", "wrench": "Wrench", "flask": "Flask", "bolt": "Lightning",
}

// iconChoice is one built-in icon as the picker offers it.
type iconChoice struct {
	Name  string
	Label string
}

// iconPicker is what the icon picker on a project form renders.
type iconPicker struct {
	Project  core.Project
	Kind     string
	Builtins []iconChoice
}

// newIconPicker opens the picker on the kind the project already carries.
func newIconPicker(p core.Project) iconPicker {
	kind := iconKindText
	switch p.IconKind() {
	case core.IconKindBuiltin:
		kind = iconKindBuiltin
	case core.IconKindImage:
		kind = iconKindImage
	case core.IconKindNone:
		if p.ID != "" {
			kind = iconKindNone
		}
	}
	out := iconPicker{Project: p, Kind: kind}
	for _, b := range core.BuiltinIcons() {
		out.Builtins = append(out.Builtins, iconChoice{Name: b.Name, Label: builtinIconLabels[b.Name]})
	}
	return out
}

// iconMark is a project icon as a template draws it: exactly one of Text,
// Builtin and Image is set, or none.
type iconMark struct {
	Key     string
	Name    string
	Text    string
	Builtin string
	Image   string
	Hidden  bool
}

// Any reports whether there is an icon to draw.
func (m iconMark) Any() bool { return m.Text != "" || m.Builtin != "" || m.Image != "" }

// markOf reads the icon of anything carrying a project's appearance. Hidden
// marks the icon decorative, beside a text that already names the project.
func markOf(v any, hidden bool) iconMark {
	var m iconMark
	switch p := v.(type) {
	case core.Project:
		m = iconMark{Key: p.Key, Name: p.Name, Text: p.Icon, Builtin: p.IconName, Image: p.IconImage}
	case *core.Project:
		if p != nil {
			return markOf(*p, hidden)
		}
	case projectAccent:
		m = iconMark{Key: p.Key, Name: p.Name, Text: p.Icon, Builtin: p.IconName, Image: p.IconImage}
	case projectChoice:
		m = iconMark{Key: p.Key, Name: p.Name, Text: p.Icon, Builtin: p.IconName, Image: p.IconImage}
	}
	m.Hidden = hidden
	return m
}

// iconChoiceOf reads the icon a project form asks for into an update. A form
// that names no kind is an older one carrying only the text icon. Keep says
// the project already has an image, which an Upload with no new file keeps.
func iconChoiceOf(r *http.Request, keep bool) (core.UpdateProjectInput, error) {
	var in core.UpdateProjectInput
	kind, named := sent(r, "icon_kind")
	if !named {
		text := field(r, "icon")
		in.Icon = &text
		return in, nil
	}
	empty := ""
	switch kind {
	case iconKindText:
		text := field(r, "icon")
		in.Icon = &text
	case iconKindBuiltin:
		name := field(r, "icon_name")
		if name == "" {
			return in, core.Invalid("choose one of the built-in icons")
		}
		in.IconName = &name
	case iconKindImage:
		file, err := uploadedIcon(r)
		if err != nil {
			return in, err
		}
		switch {
		case file != nil:
			in.IconFile = file
		case !keep:
			return in, core.Invalid("choose an image file to upload")
		}
	default:
		in.Icon = &empty
	}
	return in, nil
}

// uploadedIcon reads the image a form uploaded, or nil when it carried none.
func uploadedIcon(r *http.Request) (*core.ProjectIconFile, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	file, header, err := r.FormFile("icon_file")
	if err != nil {
		return nil, nil
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, core.MaxProjectIconBytes+1))
	if err != nil {
		return nil, core.Invalid("the icon image could not be read")
	}
	if len(data) == 0 {
		return nil, nil
	}
	return &core.ProjectIconFile{ContentType: header.Header.Get("Content-Type"), Data: data}, nil
}

// iconCacheControl lets a browser keep an icon addressed by its digest for
// good, since a new image is a new address.
const iconCacheControl = "private, max-age=31536000, immutable"

// showProjectIcon answers with a project's uploaded icon image.
func (h *handler) showProjectIcon(w http.ResponseWriter, r *http.Request) error {
	img, err := h.svc.GetProjectIcon(r.Context(), r.PathValue("key"))
	if err != nil {
		return err
	}
	etag := wire.IconETag(img.Digest)
	hdr := w.Header()
	hdr.Set("ETag", etag)
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", wire.IconPolicy)
	if r.URL.Query().Get("v") == img.Digest {
		hdr.Set("Cache-Control", iconCacheControl)
	} else {
		hdr.Set("Cache-Control", "private, no-cache")
	}
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	hdr.Set("Content-Type", img.ContentType)
	hdr.Set("Content-Length", strconv.Itoa(len(img.Data)))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(img.Data) // #nosec G705 -- a checked image, served nosniff under a sandboxing CSP and drawn only through <img>
	return err
}
