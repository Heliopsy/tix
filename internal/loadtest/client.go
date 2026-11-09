// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// ErrWritesRefused is returned for a mutating request on a client that may
// only read: every remote run without --allow-writes.
var ErrWritesRefused = errors.New("write refused: this run is read-only, pass --allow-writes and a scratch project to write")

// Account is one set of credentials in one tenant.
type Account struct {
	Tenant   string
	Email    string
	Password string
	Token    string
}

// Target is the server a run drives.
type Target struct {
	Base   string
	Remote bool
	// AllowWrites lets agents create, update, transition and comment.
	AllowWrites bool
	// Project confines writes to one project, by key. A remote run that writes
	// always has one; a local run writes across the seeded projects.
	Project  string
	Accounts []Account
}

// client is one virtual user's connection to the server.
type client struct {
	base   string
	http   *http.Client
	token  string
	tenant string
	rec    *Recorder
	writes bool

	mu    sync.Mutex
	etags map[string]string
	mods  map[string]string
}

func newClient(t Target, acct Account, rec *Recorder, timeout time.Duration) *client {
	jar, _ := cookiejar.New(nil)
	return &client{
		base:   strings.TrimRight(t.Base, "/"),
		http:   &http.Client{Jar: jar, Timeout: timeout, CheckRedirect: noRedirect},
		tenant: acct.Tenant,
		rec:    rec,
		writes: t.AllowWrites,
		etags:  map[string]string{},
		mods:   map[string]string{},
	}
}

func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// response is what a request returned.
type response struct {
	status int
	body   []byte
	header http.Header
}

// mutating reports whether a method changes state on the server.
func mutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// guard refuses a mutating request on a read-only client. The sign-in is the
// one exception: it creates a session, which a browser user cannot do without.
func (c *client) guard(method, path string) error {
	if !mutating(method) || c.writes || path == wire.RouteLogin {
		return nil
	}
	return ErrWritesRefused
}

// do sends one request, records it under route, and returns the response.
// A status of 400 or above is returned as a response, not an error.
func (c *client) do(ctx context.Context, route, method, path string, body any, header http.Header) (*response, error) {
	if err := c.guard(method, path); err != nil {
		c.rec.Note("writes refused")
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding %s body: %w", route, err)
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return nil, fmt.Errorf("building %s: %w", route, err)
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set(wire.HeaderAuth, "Bearer "+c.token)
	}
	req.Header.Set("User-Agent", "tixload")
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			c.rec.Add(Sample{Route: route, Latency: time.Since(start), Err: err.Error()})
		}
		return nil, fmt.Errorf("%s: %w", route, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	lat := time.Since(start)
	if err != nil {
		if ctx.Err() == nil {
			c.rec.Add(Sample{Route: route, Latency: lat, Err: err.Error()})
		}
		return nil, fmt.Errorf("%s: reading the body: %w", route, err)
	}
	c.rec.Add(Sample{Route: route, Status: resp.StatusCode, Latency: lat})
	return &response{status: resp.StatusCode, body: data, header: resp.Header}, nil
}

// api calls a JSON route under /api/v1 and decodes a 2xx answer into out.
func (c *client) api(ctx context.Context, route, method, path string, in, out any) (int, error) {
	r, err := c.do(ctx, route, method, wire.APIPrefix+path, in, nil)
	if err != nil {
		return 0, err
	}
	if r.status >= 300 {
		return r.status, fmt.Errorf("%s answered %d: %s", route, r.status, snippet(r.body))
	}
	if out != nil && len(r.body) > 0 {
		if err := json.Unmarshal(r.body, out); err != nil {
			return r.status, fmt.Errorf("decoding %s: %w", route, err)
		}
	}
	return r.status, nil
}

// login signs in with the account's password and keeps the session cookie.
func (c *client) login(ctx context.Context, a Account) error {
	h := http.Header{}
	if a.Tenant != "" {
		h.Set(core.HeaderTenant, a.Tenant)
	}
	r, err := c.do(ctx, "POST /auth/login", http.MethodPost, wire.RouteLogin,
		map[string]string{"email": a.Email, "password": a.Password, "surface": "web"}, h)
	if err != nil {
		return err
	}
	if r.status != http.StatusOK {
		return fmt.Errorf("signing in as %s answered %d: %s", a.Email, r.status, snippet(r.body))
	}
	return nil
}

// page reads a web page with the session cookie, plus any extra cookies.
func (c *client) page(ctx context.Context, route, path string, cookies ...*http.Cookie) (*response, error) {
	h := http.Header{"Accept": {"text/html"}}
	for _, ck := range cookies {
		h.Add("Cookie", ck.String())
	}
	return c.do(ctx, route, http.MethodGet, path, nil, h)
}

// asset reads a static file the way a browser with a warm cache does: with
// the validators the last answer carried, so a repeat is a 304.
func (c *client) asset(ctx context.Context, path string) error {
	c.mu.Lock()
	h := http.Header{}
	if v := c.etags[path]; v != "" {
		h.Set("If-None-Match", v)
	}
	if v := c.mods[path]; v != "" {
		h.Set("If-Modified-Since", v)
	}
	c.mu.Unlock()
	r, err := c.do(ctx, "GET /assets/*", http.MethodGet, path, nil, h)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if v := r.header.Get("ETag"); v != "" {
		c.etags[path] = v
	}
	if v := r.header.Get("Last-Modified"); v != "" {
		c.mods[path] = v
	}
	return nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
