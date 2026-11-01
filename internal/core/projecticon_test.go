// SPDX-License-Identifier: AGPL-3.0-or-later

package core_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

const svgHead = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">`

func TestSniffProjectIcon(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		data []byte
		want string
		err  string
	}{
		{"png", []byte("\x89PNG\r\n\x1a\nrest"), "image/png", ""},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0}, "image/jpeg", ""},
		{"webp", []byte("RIFF\x10\x00\x00\x00WEBPVP8 "), "image/webp", ""},
		{"svg", []byte(svgHead + `<circle cx="12" cy="12" r="9"/></svg>`), "image/svg+xml", ""},
		{"svg with prolog and doctype", []byte(`<?xml version="1.0"?>` + "\n" +
			`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` +
			svgHead + `</svg>`), "image/svg+xml", ""},
		{"svg using its own fragment", []byte(svgHead + `<defs><path id="a" d="M0 0h1"/></defs><use href="#a"/></svg>`), "image/svg+xml", ""},
		{"svg embedding a png", []byte(svgHead + `<image href="data:image/png;base64,AAAA"/></svg>`), "image/svg+xml", ""},
		{"empty", nil, "", "empty"},
		{"too large", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, core.MaxProjectIconBytes)...), "", "limit"},
		{"plain text", []byte("hello, I am a png"), "", "not a PNG"},
		{"html", []byte("<html><body>hi</body></html>"), "", "not a PNG"},
		{"gif", []byte("GIF89a......"), "", "not a PNG"},
		{"script element", []byte(svgHead + `<script>alert(1)</script></svg>`), "", "script"},
		{"script in a namespace", []byte(svgHead + `<x:script xmlns:x="http://www.w3.org/2000/svg">alert(1)</x:script></svg>`), "", "script"},
		{"event handler", []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`), "", "event handler"},
		{"event handler on a child", []byte(svgHead + `<rect onClick="x()"/></svg>`), "", "event handler"},
		{"foreign object", []byte(svgHead + `<foreignObject><div/></foreignObject></svg>`), "", "foreignObject"},
		{"javascript href", []byte(svgHead + `<a href=" java	script:alert(1)"><rect/></a></svg>`), "", "javascript"},
		{"external href", []byte(svgHead + `<image href="https://example.com/x.png"/></svg>`), "", "refers to"},
		{"external xlink", []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="other.svg#a"/></svg>`), "", "refers to"},
		{"embedded svg data", []byte(svgHead + `<image href="data:image/svg+xml;base64,AAAA"/></svg>`), "", "refers to"},
		{"entity declaration", []byte(`<!DOCTYPE svg [<!ENTITY a "aaaa">]>` + svgHead + `&a;</svg>`), "", "entities"},
		{"stylesheet instruction", []byte(`<?xml-stylesheet href="x.css"?>` + svgHead + `</svg>`), "", "processing instruction"},
		{"truncated svg", []byte(svgHead + `<rect`), "", "could not be read"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := core.SniffProjectIcon(tc.data)
			if tc.err == "" {
				if err != nil || got != tc.want {
					t.Fatalf("SniffProjectIcon = %q, %v; want %q", got, err, tc.want)
				}
				return
			}
			if err == nil || !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("SniffProjectIcon = %q, %v; want an invalid error naming %q", got, err, tc.err)
			}
		})
	}
}

func TestNewProjectIconImageIgnoresTheDeclaredType(t *testing.T) {
	t.Parallel()
	png := []byte("\x89PNG\r\n\x1a\nbody")
	img, err := core.NewProjectIconImage(core.ProjectIconFile{ContentType: "text/html", Data: png})
	if err != nil {
		t.Fatal(err)
	}
	if img.ContentType != "image/png" || img.Digest != core.ProjectIconDigest(png) || !bytes.Equal(img.Data, png) {
		t.Fatalf("image = %+v", img)
	}
	if _, err := core.NewProjectIconImage(core.ProjectIconFile{ContentType: "image/png", Data: []byte("not one")}); err == nil {
		t.Fatal("text declared as a png was accepted")
	}
	if core.ProjectIconDigest(png) == core.ProjectIconDigest(append(png, 'x')) {
		t.Fatal("two images share a digest")
	}
}

func TestNormalizeProjectIconName(t *testing.T) {
	t.Parallel()
	for _, b := range core.BuiltinIcons() {
		got, err := core.NormalizeProjectIconName(" " + strings.ToUpper(b.Name) + " ")
		if err != nil || got != b.Name {
			t.Errorf("NormalizeProjectIconName(%q) = %q, %v", b.Name, got, err)
		}
		if b.Glyph == "" {
			t.Errorf("%s has no fallback glyph", b.Name)
		}
	}
	if got, err := core.NormalizeProjectIconName("  "); err != nil || got != "" {
		t.Fatalf("blank = %q, %v", got, err)
	}
	_, err := core.NormalizeProjectIconName("unicorn")
	if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "rocket") {
		t.Fatalf("unknown name = %v, want an invalid error listing the icons", err)
	}
}

func TestValidateProjectIconChoice(t *testing.T) {
	t.Parallel()
	png := &core.ProjectIconFile{Data: []byte("\x89PNG\r\n\x1a\n")}
	cases := []struct {
		name       string
		text, icon string
		file       *core.ProjectIconFile
		ok         bool
	}{
		{"none", "", "", nil, true},
		{"text", "W", "", nil, true},
		{"builtin", "", "rocket", nil, true},
		{"image", "", "", png, true},
		{"empty file asks for none", "", "", &core.ProjectIconFile{}, true},
		{"text and builtin", "W", "rocket", nil, false},
		{"text and image", "W", "", png, false},
		{"builtin and image", "", "rocket", png, false},
		{"long text", "WWW", "", nil, false},
		{"unknown builtin", "", "unicorn", nil, false},
		{"bad image", "", "", &core.ProjectIconFile{Data: []byte("x")}, false},
	}
	for _, tc := range cases {
		err := core.ValidateProjectIconChoice(tc.text, tc.icon, tc.file)
		if (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok %v", tc.name, err, tc.ok)
		}
	}
	in := core.CreateProjectInput{Key: "web", Name: "Web", Icon: "W", IconName: "globe"}
	if err := in.Validate(); err == nil {
		t.Fatal("a create naming two icon kinds validated")
	}
}

func TestProjectIconKindAndText(t *testing.T) {
	t.Parallel()
	glyph, _ := core.BuiltinIconGlyph("rocket")
	cases := []struct {
		p    core.Project
		kind core.ProjectIconKind
		text string
	}{
		{core.Project{}, core.IconKindNone, ""},
		{core.Project{Icon: "W"}, core.IconKindText, "W"},
		{core.Project{IconName: "rocket"}, core.IconKindBuiltin, glyph},
		{core.Project{IconImage: "abc"}, core.IconKindImage, core.ProjectImageGlyph},
	}
	for _, tc := range cases {
		if got := tc.p.IconKind(); got != tc.kind {
			t.Errorf("%+v: kind %q, want %q", tc.p, got, tc.kind)
		}
		if got := tc.p.IconText(); got != tc.text {
			t.Errorf("%+v: text %q, want %q", tc.p, got, tc.text)
		}
	}
}
