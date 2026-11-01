// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"time"
)

// ProjectIconKind names which of the three icon kinds a project carries.
type ProjectIconKind string

// The icon kinds. A project carries at most one.
const (
	IconKindNone    ProjectIconKind = ""
	IconKindText    ProjectIconKind = "text"
	IconKindBuiltin ProjectIconKind = "builtin"
	IconKindImage   ProjectIconKind = "image"
)

// BuiltinIcon is one symbol of the interface's own sprite, with the glyph a
// surface that cannot draw the sprite shows instead.
type BuiltinIcon struct {
	Name  string `json:"name" yaml:"name"`
	Glyph string `json:"glyph" yaml:"glyph"`
}

// builtinIcons is the curated set, in the order a picker offers it.
var builtinIcons = []BuiltinIcon{
	{"folder", "\U0001F4C1"},
	{"briefcase", "\U0001F4BC"},
	{"home", "\U0001F3E0"},
	{"server", "\U0001F5A5"},
	{"database", "\U0001F5C4"},
	{"cloud", "☁"},
	{"code", "{}"},
	{"terminal", ">_"},
	{"bug", "\U0001F41B"},
	{"rocket", "\U0001F680"},
	{"shield", "\U0001F6E1"},
	{"lock", "\U0001F512"},
	{"globe", "\U0001F310"},
	{"chart", "\U0001F4CA"},
	{"book", "\U0001F4D6"},
	{"users", "\U0001F465"},
	{"package", "\U0001F4E6"},
	{"wrench", "\U0001F527"},
	{"flask", "\U0001F9EA"},
	{"bolt", "⚡"},
}

// ProjectImageGlyph stands in for an uploaded image where none can be drawn.
const ProjectImageGlyph = "▣"

// MaxProjectIconBytes bounds an uploaded icon image.
const MaxProjectIconBytes = 64 << 10

// BuiltinIcons returns the built-in project icons in display order.
func BuiltinIcons() []BuiltinIcon {
	out := make([]BuiltinIcon, len(builtinIcons))
	copy(out, builtinIcons)
	return out
}

// BuiltinIconGlyph returns the fallback glyph of a built-in icon.
func BuiltinIconGlyph(name string) (string, bool) {
	for _, b := range builtinIcons {
		if b.Name == name {
			return b.Glyph, true
		}
	}
	return "", false
}

// NormalizeProjectIconName trims and lowercases a built-in icon name and
// checks it is one the system ships.
func NormalizeProjectIconName(s string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(s))
	if name == "" {
		return "", nil
	}
	if _, ok := BuiltinIconGlyph(name); ok {
		return name, nil
	}
	names := make([]string, 0, len(builtinIcons))
	for _, b := range builtinIcons {
		names = append(names, b.Name)
	}
	return "", Invalid("project icon name %q is not a built-in icon; choose one of %s",
		s, strings.Join(names, ", "))
}

// ProjectIconFile is an image a caller uploads as a project's icon. The
// declared content type is advisory: the bytes decide.
type ProjectIconFile struct {
	ContentType string `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	Data        []byte `json:"data" yaml:"data"`
}

// Empty reports whether the file carries no bytes, which asks for no image.
func (f *ProjectIconFile) Empty() bool { return f == nil || len(f.Data) == 0 }

// ProjectIconImage is a stored project icon image.
type ProjectIconImage struct {
	ProjectID   string    `json:"project_id" yaml:"project_id"`
	ContentType string    `json:"content_type" yaml:"content_type"`
	Digest      string    `json:"digest" yaml:"digest"`
	Data        []byte    `json:"data" yaml:"data"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"updated_at"`
}

// NewProjectIconImage checks an upload and returns the image to store, typed
// by its bytes and fingerprinted by its digest.
func NewProjectIconImage(f ProjectIconFile) (ProjectIconImage, error) {
	contentType, err := SniffProjectIcon(f.Data)
	if err != nil {
		return ProjectIconImage{}, err
	}
	return ProjectIconImage{ContentType: contentType, Digest: ProjectIconDigest(f.Data), Data: f.Data}, nil
}

// ProjectIconDigest fingerprints image bytes for the project row and the ETag.
func ProjectIconDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}

// SniffProjectIcon returns the content type of an icon image, refusing
// anything that is not a small PNG, JPEG, WebP or safe SVG.
func SniffProjectIcon(data []byte) (string, error) {
	switch {
	case len(data) == 0:
		return "", Invalid("project icon image is empty")
	case len(data) > MaxProjectIconBytes:
		return "", Invalid("project icon image is %d bytes; the limit is %d", len(data), MaxProjectIconBytes)
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", nil
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", nil
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp", nil
	}
	if err := checkSVG(data); err != nil {
		return "", err
	}
	return "image/svg+xml", nil
}

// errNotSVG marks bytes that are not an SVG document at all.
var errNotSVG = errors.New("not an svg document")

// checkSVG accepts an SVG document only when nothing in it can run or fetch.
func checkSVG(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	root := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if !root {
				return Invalid("project icon image is not a PNG, JPEG, WebP or SVG image")
			}
			return Invalid("project icon SVG could not be read: %v", err)
		}
		if err := checkSVGToken(tok, &root); err != nil {
			if errors.Is(err, errNotSVG) {
				return Invalid("project icon image is not a PNG, JPEG, WebP or SVG image")
			}
			return err
		}
	}
	if !root {
		return Invalid("project icon image is not a PNG, JPEG, WebP or SVG image")
	}
	return nil
}

// svgRefused are elements that run script or embed another document.
var svgRefused = map[string]bool{
	"script": true, "foreignobject": true, "iframe": true, "embed": true, "object": true,
}

func checkSVGToken(tok xml.Token, root *bool) error {
	switch t := tok.(type) {
	case xml.ProcInst:
		if t.Target != "xml" {
			return Invalid("project icon SVG carries a processing instruction %q", t.Target)
		}
	case xml.Directive:
		if bytes.Contains(t, []byte("[")) || bytes.Contains(bytes.ToUpper(t), []byte("ENTITY")) {
			return Invalid("project icon SVG declares entities, which are not accepted")
		}
	case xml.StartElement:
		name := strings.ToLower(t.Name.Local)
		if !*root {
			if name != "svg" {
				return errNotSVG
			}
			*root = true
		}
		if svgRefused[name] {
			return Invalid("project icon SVG carries a %s element, which is not accepted", t.Name.Local)
		}
		for _, a := range t.Attr {
			if err := checkSVGAttr(a); err != nil {
				return err
			}
		}
	case xml.CharData:
		if !*root && len(bytes.TrimSpace(t)) > 0 {
			return errNotSVG
		}
	}
	return nil
}

// svgSafeData are the embedded images an SVG reference may carry.
var svgSafeData = []string{"data:image/png", "data:image/jpeg", "data:image/gif", "data:image/webp"}

func checkSVGAttr(a xml.Attr) error {
	name := strings.ToLower(a.Name.Local)
	value := strings.ToLower(strings.Join(strings.Fields(a.Value), ""))
	if strings.HasPrefix(name, "on") {
		return Invalid("project icon SVG carries an event handler %q, which is not accepted", a.Name.Local)
	}
	if strings.Contains(value, "javascript:") {
		return Invalid("project icon SVG carries a javascript: address, which is not accepted")
	}
	if name != "href" && name != "src" {
		return nil
	}
	if strings.HasPrefix(value, "#") {
		return nil
	}
	for _, prefix := range svgSafeData {
		if strings.HasPrefix(value, prefix) {
			return nil
		}
	}
	return Invalid("project icon SVG refers to %q; only fragments of itself and embedded images are accepted", a.Value)
}

// IconKind reports which kind of icon the project carries.
func (p Project) IconKind() ProjectIconKind {
	switch {
	case p.IconImage != "":
		return IconKindImage
	case p.IconName != "":
		return IconKindBuiltin
	case p.Icon != "":
		return IconKindText
	default:
		return IconKindNone
	}
}

// IconText is the project's icon as text, for a surface that draws no sprite
// and no image: the text icon itself, a built-in icon's fallback glyph, or a
// fixed mark for an image.
func (p Project) IconText() string {
	switch p.IconKind() {
	case IconKindImage:
		return ProjectImageGlyph
	case IconKindBuiltin:
		glyph, _ := BuiltinIconGlyph(p.IconName)
		return glyph
	default:
		return p.Icon
	}
}

// ValidateProjectIconChoice checks each icon kind on its own and refuses a
// write naming more than one.
func ValidateProjectIconChoice(text, name string, file *ProjectIconFile) error {
	if _, err := NormalizeProjectIcon(text); err != nil {
		return err
	}
	if _, err := NormalizeProjectIconName(name); err != nil {
		return err
	}
	named := 0
	for _, set := range []bool{strings.TrimSpace(text) != "", strings.TrimSpace(name) != "", !file.Empty()} {
		if set {
			named++
		}
	}
	if named > 1 {
		return Invalid("a project icon is one of text, a built-in name or an image; name only one")
	}
	if !file.Empty() {
		if _, err := SniffProjectIcon(file.Data); err != nil {
			return err
		}
	}
	return nil
}
