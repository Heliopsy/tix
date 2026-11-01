// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

var webPNG = []byte("\x89PNG\r\n\x1a\nweb icon")

// postIcon submits a project form as the browser does once it carries the
// picker: multipart, with the icon file under icon_file.
func (b *browser) postIcon(path string, fields map[string]string, file []byte) *http.Response {
	b.t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fields["csrf_token"] = b.csrf()
	for name, value := range fields {
		if err := w.WriteField(name, value); err != nil {
			b.t.Fatal(err)
		}
	}
	part, err := w.CreateFormFile("icon_file", "logo.png")
	if err != nil {
		b.t.Fatal(err)
	}
	if _, err := part.Write(file); err != nil {
		b.t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		b.t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, b.fix.server.URL+path, &body)
	if err != nil {
		b.t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	return resp
}

func settingsFields(kind string) map[string]string {
	return map[string]string{"name": "infra", "workflow_key": "default", "icon_kind": kind}
}

// boardHeading is the board's h1, where the project's icon leads its name.
func boardHeading(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<div class="headtip"><h1>`, "</h1>")
}

// projectRef is the project reference on the first task row of a listing.
func projectRef(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<span class="tm tm-proj"><a class="projref`, "</a>")
}

// iconPicker is the picker fieldset of the one form on the page posting to action.
func iconPicker(t *testing.T, page, action string) string {
	t.Helper()
	form := between(t, page, `action="`+action+`" enctype="multipart/form-data">`, "</form>")
	return between(t, form, `<fieldset class="iconpick"`, "</fieldset>")
}

func TestProjectFormsOfferTheIconPicker(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for path, action := range map[string]string{"/projects": "/projects", "/projects/infra": "/projects/infra/update"} {
		picker := iconPicker(t, b.page(path), action)
		for _, kind := range []string{"text", "builtin", "image", "none"} {
			if !strings.Contains(picker, `name="icon_kind" value="`+kind+`"`) {
				t.Errorf("%s: the picker offers no %s kind", path, kind)
			}
		}
		for _, icon := range core.BuiltinIcons() {
			cell := `<input type="radio" name="icon_name" value="` + icon.Name + `"`
			if !strings.Contains(picker, cell) || !strings.Contains(picker, `icons.svg#pi-`+icon.Name+`"`) {
				t.Errorf("%s: the grid has no %s cell drawn from the sprite", path, icon.Name)
			}
		}
		for _, want := range []string{`name="icon_file" type="file"`, `name="icon" maxlength="2"`,
			`data-iconpick-preview`, `class="field-info tip"`} {
			if !strings.Contains(picker, want) {
				t.Errorf("%s: the picker carries no %s", path, want)
			}
		}
	}
}

func TestEveryBuiltinIconIsInTheSprite(t *testing.T) {
	t.Parallel()
	sprite, err := os.ReadFile("assets/icons.svg")
	if err != nil {
		t.Fatal(err)
	}
	var drawn []string
	for _, m := range regexp.MustCompile(`<symbol id="pi-([a-z]+)"`).FindAllStringSubmatch(string(sprite), -1) {
		drawn = append(drawn, m[1])
	}
	var named []string
	for _, b := range core.BuiltinIcons() {
		named = append(named, b.Name)
	}
	if !slices.Equal(drawn, named) {
		t.Fatalf("the sprite draws %v, the built-in icons are %v", drawn, named)
	}
}

func TestChoosingIconKindsFromTheSettingsForm(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "a task to carry the icon")

	fields := settingsFields("builtin")
	fields["icon_name"] = "rocket"
	resp := b.postIcon("/projects/infra/update", fields, nil)
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	sprite := `<use href="/assets/icons.svg#pi-rocket"/>`
	if h := boardHeading(t, b.page("/projects/infra")); !strings.Contains(h, sprite) {
		t.Fatalf("the board heading does not draw the rocket: %s", h)
	}
	if ref := projectRef(t, b.page("/tasks")); !strings.Contains(ref, `<svg class="proj-icon proj-icon-svg" width="16" height="16" aria-hidden="true">`+sprite) {
		t.Fatalf("the task row's project reference does not draw the built-in icon: %s", ref)
	}

	resp = b.postIcon("/projects/infra/update", settingsFields("image"), webPNG)
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	digest := core.ProjectIconDigest(webPNG)
	img := `<img class="proj-icon proj-icon-img" src="/projects/infra/icon?v=` + digest + `"`
	if h := boardHeading(t, b.page("/projects/infra")); !strings.Contains(h, img) || strings.Contains(h, sprite) {
		t.Fatalf("the board heading does not draw the uploaded image alone: %s", h)
	}
	if ref := projectRef(t, b.page("/tasks")); !strings.Contains(ref, img) {
		t.Fatalf("the task row's project reference does not draw the uploaded image: %s", ref)
	}
	if picker := iconPicker(t, b.page("/projects/infra"), "/projects/infra/update"); !strings.Contains(picker, `value="image" checked`) ||
		!strings.Contains(between(t, picker, "data-iconpick-preview", "</span>"), img) {
		t.Fatal("the settings form does not open on the image it previews")
	}

	fields = settingsFields("image")
	fields["name"] = "Infrastructure"
	resp = b.postIcon("/projects/infra/update", fields, nil)
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	if h := boardHeading(t, b.page("/projects/infra")); !strings.Contains(h, img) || !strings.Contains(h, "Infrastructure") {
		t.Fatalf("saving without a new file lost the image: %s", h)
	}

	resp = b.postIcon("/projects/infra/update", settingsFields("none"), nil)
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	if h := boardHeading(t, b.page("/projects/infra")); strings.Contains(h, "proj-icon") {
		t.Fatalf("removing the icon left one: %s", h)
	}
	gone := b.get("/projects/infra/icon")
	_ = gone.Body.Close()
	if gone.StatusCode != http.StatusNotFound {
		t.Fatalf("a removed image answered %d", gone.StatusCode)
	}

	resp = b.postIcon("/projects/infra/update", settingsFields("image"), nil)
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusSeeOther {
		t.Fatal("Upload with no file and no image to keep was accepted")
	}
}

func TestCreatingAProjectWithAnUploadedIcon(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.postIcon("/projects", map[string]string{"key": "web", "name": "Web", "icon_kind": "image"}, webPNG)
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	mark := between(t, b.page("/projects"), `<a class="projrow-main" href="/projects/web">`, "</a>")
	if !strings.Contains(mark, `src="/projects/web/icon?v=`+core.ProjectIconDigest(webPNG)+`"`) {
		t.Fatalf("the listing does not draw the uploaded icon: %s", mark)
	}
	resp = b.postIcon("/projects", map[string]string{"key": "bad", "name": "Bad", "icon_kind": "image"},
		[]byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`))
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusSeeOther {
		t.Fatal("a scripted SVG was accepted")
	}
}

func TestTheIconRouteServesTheImageSafely(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.postIcon("/projects/infra/update", settingsFields("image"), webPNG)
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	digest := core.ProjectIconDigest(webPNG)

	got := b.get("/projects/infra/icon?v=" + digest)
	body, _ := io.ReadAll(got.Body)
	_ = got.Body.Close()
	wantStatus(t, got, http.StatusOK)
	if !bytes.Equal(body, webPNG) {
		t.Fatalf("icon body = %q", body)
	}
	want := map[string]string{
		"Content-Type":            "image/png",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": wire.IconPolicy,
		"ETag":                    `"` + digest + `"`,
		"Cache-Control":           "private, max-age=31536000, immutable",
	}
	for name, value := range want {
		if v := got.Header.Get(name); v != value {
			t.Errorf("%s = %q, want %q", name, v, value)
		}
	}

	stale := b.get("/projects/infra/icon?v=older")
	_ = stale.Body.Close()
	if cc := stale.Header.Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Errorf("an address naming another digest is cached for good: %q", cc)
	}

	req, err := http.NewRequest(http.MethodGet, f.server.URL+"/projects/infra/icon?v="+digest, nil)
	if err != nil {
		t.Fatal(err)
	}
	b.stamp(req)
	req.Header.Set("If-None-Match", `"`+digest+`"`)
	cached, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	rest, _ := io.ReadAll(cached.Body)
	_ = cached.Body.Close()
	if cached.StatusCode != http.StatusNotModified || len(rest) != 0 {
		t.Fatalf("revalidation answered %d with %d bytes", cached.StatusCode, len(rest))
	}
}
