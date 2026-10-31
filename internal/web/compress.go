// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// A listing of 25 tasks is about 185KB of HTML, nearly all of it markup that
// repeats row after row, and every screen went out uncompressed: so did the
// 200KB stylesheet and the 50KB of htmx under it. Over a link slower than
// loopback the bytes were most of what a click cost. gzip takes that listing
// to about 12KB.
//
// Screens are compressed as they are written, at the fastest level, since a
// screen is rendered once and sent once. Assets never change for the life of
// the process, so they are compressed once, at start, at the best level.

// acceptsGzip reports whether the request will take a gzip body: it names
// gzip, and not with a weight of zero.
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(name), "gzip") {
			continue
		}
		_, weight, found := strings.Cut(strings.ReplaceAll(params, " ", ""), "q=")
		if !found {
			return true
		}
		q, err := strconv.ParseFloat(weight, 64)
		return err == nil && q > 0
	}
	return false
}

var gzipWriters = sync.Pool{New: func() any {
	w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	return w
}}

// compressed wraps next so an HTML response goes out gzipped to a browser
// that takes it. Anything else passes through untouched.
func compressed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if !acceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipResponse{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

// gzipResponse decides at WriteHeader whether this response is one to
// compress, from the content type the handler set.
type gzipResponse struct {
	http.ResponseWriter
	gz      *gzip.Writer
	decided bool
}

func (g *gzipResponse) WriteHeader(status int) {
	if !g.decided {
		g.decided = true
		h := g.Header()
		if status != http.StatusNoContent && status != http.StatusNotModified &&
			h.Get("Content-Encoding") == "" &&
			strings.HasPrefix(h.Get("Content-Type"), "text/html") {
			h.Set("Content-Encoding", "gzip")
			h.Del("Content-Length")
			g.gz = gzipWriters.Get().(*gzip.Writer)
			g.gz.Reset(g.ResponseWriter)
		}
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipResponse) Write(p []byte) (int, error) {
	if !g.decided {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(p))
		}
		g.WriteHeader(http.StatusOK)
	}
	if g.gz != nil {
		return g.gz.Write(p)
	}
	return g.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the connection underneath.
func (g *gzipResponse) Unwrap() http.ResponseWriter { return g.ResponseWriter }

func (g *gzipResponse) close() {
	if g.gz == nil {
		return
	}
	_ = g.gz.Close()
	g.gz.Reset(io.Discard)
	gzipWriters.Put(g.gz)
	g.gz = nil
}

// gzipBytes compresses one asset at the best level.
func gzipBytes(body []byte) []byte {
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = w.Write(body)
	_ = w.Close()
	return buf.Bytes()
}
