// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestRawSendsTheRequestAsGivenAndReturnsTheBody(t *testing.T) {
	var got struct{ method, path, query, auth, ctype, body string }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got.method, got.path, got.query = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		got.auth, got.ctype, got.body = r.Header.Get("Authorization"), r.Header.Get("Content-Type"), string(raw)
		if r.URL.Query().Get("fail") != "" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"code":"not_found","message":"task x"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"ref":"x"}`)
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, "tok")
	if err != nil {
		t.Fatal(err)
	}

	body, err := c.Raw(context.Background(), http.MethodPost, "/api/v1/tasks/a%2Fb", url.Values{"tag": {"x", "y"}}, []byte(`{"t":1}`))
	if err != nil || string(body) != `{"ref":"x"}` {
		t.Fatalf("Raw = %q, %v", body, err)
	}
	if got.method != http.MethodPost || got.path != "/api/v1/tasks/a%2Fb" || got.query != "tag=x&tag=y" ||
		got.auth != "Bearer tok" || got.ctype != "application/json" || got.body != `{"t":1}` {
		t.Errorf("server saw %+v", got)
	}

	if _, err := c.Raw(context.Background(), http.MethodGet, "/api/v1/tags", nil, nil); err != nil || got.ctype != "" {
		t.Errorf("a bodiless request: err %v, content type %q", err, got.ctype)
	}

	_, err = c.Raw(context.Background(), http.MethodGet, "/api/v1/tasks/x", url.Values{"fail": {"1"}}, nil)
	var de *core.Error
	if !errors.As(err, &de) || de.Kind != core.KindNotFound || de.Message != "task x" {
		t.Errorf("a failing response = %v, want the envelope's not_found", err)
	}
}
