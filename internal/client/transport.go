// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// maxErrorBody bounds how much of a failing response is read before decoding.
const maxErrorBody = 1 << 20

// routePath substitutes {name} placeholders in a wire route pattern,
// percent-encoding every value.
func routePath(pattern string, params ...string) string {
	out := pattern
	for i := 0; i+1 < len(params); i += 2 {
		out = strings.Replace(out, "{"+params[i]+"}", url.PathEscape(params[i+1]), 1)
	}
	return out
}

func (c *Client) url(path string, q url.Values) string {
	u := *c.base
	escaped := strings.TrimRight(u.EscapedPath(), "/") + path
	decoded, err := url.PathUnescape(escaped)
	if err != nil {
		decoded = escaped
	}
	u.Path, u.RawPath = decoded, escaped
	if len(q) > 0 {
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func (c *Client) newRequest(ctx context.Context, method, path string, q url.Values, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.url(path, q), body)
	if err != nil {
		return nil, core.Internal("building request: %v", err).Wrap(err)
	}
	req.Header.Set(wire.HeaderAccept, wire.ContentJSON)
	req.Header.Set("User-Agent", c.userAgent)
	if c.token != "" {
		req.Header.Set(wire.HeaderAuth, "Bearer "+c.token)
	}
	if c.cookie != "" {
		// #nosec G124 -- an outgoing request cookie carries only a name and a
		// value; Secure, HttpOnly and SameSite are server response directives
		// and have no meaning here.
		req.AddCookie(&http.Cookie{Name: wire.SessionCookieName, Value: c.cookie})
	}
	return req, nil
}

func (c *Client) send(req *http.Request) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, transportError(req.Method, req.URL.String(), err)
	}
	if resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		return nil, responseError(resp)
	}
	return resp, nil
}

// transportError reports a connectivity failure, distinguishable from any
// error the server itself produced.
func transportError(method, target string, err error) error {
	return core.Internal("%s %s: contacting server: %v", method, target, err).Wrap(err)
}

// responseError rebuilds the server's domain error from the wire envelope, and
// falls back to the HTTP status when no envelope is present.
func responseError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))

	var env wire.ErrorEnvelope
	if err := json.Unmarshal(raw, &env); err == nil && env.Error.Code != "" {
		return &core.Error{
			Kind:    env.Error.Code,
			Message: env.Error.Message,
			Details: env.Error.Details,
		}
	}
	return statusError(resp.StatusCode, raw)
}

func statusError(status int, raw []byte) error {
	message := strings.TrimSpace(string(raw))
	if message == "" {
		message = http.StatusText(status)
	}
	if len(message) > 512 {
		message = message[:512]
	}
	return &core.Error{Kind: kindForStatus(status), Message: message}
}

func kindForStatus(status int) core.Kind {
	switch status {
	case http.StatusBadRequest:
		return core.KindInvalid
	case http.StatusUnauthorized:
		return core.KindUnauthenticated
	case http.StatusForbidden:
		return core.KindForbidden
	case http.StatusNotFound:
		return core.KindNotFound
	case http.StatusConflict:
		return core.KindConflict
	case http.StatusUnprocessableEntity:
		return core.KindPrecondition
	default:
		return core.KindInternal
	}
}

func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.timeout)
}

func encodeBody(body any) (io.Reader, string, error) {
	if body == nil {
		return nil, "", nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, "", core.Internal("encoding request body: %v", err).Wrap(err)
	}
	return bytes.NewReader(raw), wire.ContentJSON, nil
}

// call issues a request and decodes the response into a new Out.
func call[Out any](ctx context.Context, c *Client, method, path string, q url.Values, body any) (*Out, error) {
	reader, contentType, err := encodeBody(body)
	if err != nil {
		return nil, err
	}

	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	req, err := c.newRequest(ctx, method, path, q, reader)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set(wire.HeaderContentType, contentType)
	}

	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	out := new(Out)
	if resp.StatusCode == http.StatusNoContent {
		return out, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		if err == io.EOF {
			return out, nil
		}
		return nil, core.Internal("decoding response from %s: %v", req.URL.String(), err).Wrap(err)
	}
	return out, nil
}

// callVoid issues a request whose response body carries nothing useful.
func callVoid(ctx context.Context, c *Client, method, path string, q url.Values, body any) error {
	_, err := call[struct{}](ctx, c, method, path, q, body)
	return err
}

// list issues a request and unwraps the paged envelope.
func list[T any](ctx context.Context, c *Client, path string, q url.Values) ([]T, string, error) {
	page, err := call[wire.Page[T]](ctx, c, http.MethodGet, path, q, nil)
	if err != nil {
		return nil, "", err
	}
	return page.Items, page.NextCursor, nil
}

// listAll issues a request for an unpaged collection.
func listAll[T any](ctx context.Context, c *Client, path string, q url.Values) ([]T, error) {
	items, _, err := list[T](ctx, c, path, q)
	return items, err
}

func pageQuery(p core.Page) url.Values {
	q := url.Values{}
	if p.Limit != 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	if p.Sort != "" {
		q.Set("sort", p.Sort)
	}
	if p.Direction != "" {
		q.Set("direction", string(p.Direction))
	}
	return q
}

func setStrings(q url.Values, key string, values []string) {
	for _, v := range values {
		q.Add(key, v)
	}
}

func setBool(q url.Values, key string, v bool) {
	if v {
		q.Set(key, "true")
	}
}

func setTime(q url.Values, key string, t *time.Time) {
	if t != nil {
		q.Set(key, t.Format(time.RFC3339Nano))
	}
}

func setJSON(q url.Values, key string, v map[string]any) {
	if len(v) == 0 {
		return
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	q.Set(key, string(raw))
}

func setTriState(q url.Values, key string, t core.TriState) {
	switch t {
	case core.Yes:
		q.Set(key, "true")
	case core.No:
		q.Set(key, "false")
	}
}
