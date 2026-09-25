// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// assetCacheControl lets a browser reuse an asset for a minute and then
// revalidate it.
//
// The asset URLs are stable across releases, and the responses used to carry
// no validator at all, so a browser cached them heuristically: after an
// upgrade a reader got the new markup with the previous release's stylesheet
// and reported the interface as broken. A strong ETag makes that revalidation
// a 304, so the window in which a stale sheet can be served is bounded by
// max-age rather than by the browser's guess.
const assetCacheControl = "public, max-age=60, must-revalidate"

// assetContentTypes names the media type of every extension the embedded
// asset set contains. The set is served from memory rather than from disk, so
// nothing else is going to sniff these for us.
var assetContentTypes = map[string]string{
	".css":  "text/css; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".svg":  "image/svg+xml",
	".json": "application/json",
}

// asset is one embedded file with the validator computed for it at startup.
type asset struct {
	body        []byte
	etag        string
	contentType string
}

// assetSet serves the embedded stylesheet, scripts and icons.
type assetSet struct {
	files map[string]asset
}

// assetHandler serves the embedded stylesheet and scripts with a strong
// validator per file, computed once because the bytes are compiled in.
func assetHandler() http.Handler {
	sub, err := fs.Sub(assetFS, "assets")
	if err != nil {
		panic(err)
	}
	set := &assetSet{files: make(map[string]asset)}
	err = fs.WalkDir(sub, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(sub, name)
		if err != nil {
			return err
		}
		set.files[name] = asset{
			body:        body,
			etag:        assetETag(body),
			contentType: assetContentType(name),
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/assets/", set)
}

// assetETag fingerprints one asset as a strong entity tag.
func assetETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// assetContentType names the media type an asset is served as.
func assetContentType(name string) string {
	if ct, ok := assetContentTypes[strings.ToLower(path.Ext(name))]; ok {
		return ct
	}
	return "application/octet-stream"
}

// ServeHTTP serves one embedded asset, answering conditional requests.
func (s *assetSet) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	a, ok := s.files[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", assetCacheControl)
	w.Header().Set("ETag", a.etag)
	w.Header().Set("Content-Type", a.contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// A zero modtime keeps ServeContent from emitting a Last-Modified an
	// embedded file cannot honestly claim; the ETag is the validator.
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(a.body))
}
