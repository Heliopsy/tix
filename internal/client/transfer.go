package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
)

// ExportTo streams a snapshot to w as the server produces it.
func (c *Client) ExportTo(ctx context.Context, in core.ExportInput, w io.Writer) error {
	body, contentType, err := encodeBody(in)
	if err != nil {
		return err
	}

	req, err := c.newRequest(ctx, http.MethodPost, httpapi.RouteExport, nil, body)
	if err != nil {
		return err
	}
	req.Header.Set(httpapi.HeaderContentType, contentType)
	req.Header.Set(httpapi.HeaderAccept, httpapi.ContentNDJSON)

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if _, err := io.Copy(w, resp.Body); err != nil {
		return core.Internal("streaming export: %v", err).Wrap(err)
	}
	return nil
}

// ImportFrom streams a snapshot from r to the server.
func (c *Client) ImportFrom(ctx context.Context, r io.Reader, in core.ImportInput) (*core.ImportResult, error) {
	q := url.Values{}
	if in.Mode != "" {
		q.Set("mode", string(in.Mode))
	}
	setBool(q, "dry_run", in.DryRun)

	req, err := c.newRequest(ctx, http.MethodPost, httpapi.RouteImport, q, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set(httpapi.HeaderContentType, httpapi.ContentNDJSON)

	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	out := new(core.ImportResult)
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
		return nil, core.Internal("decoding import result: %v", err).Wrap(err)
	}
	return out, nil
}
