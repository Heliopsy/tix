package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
)

// ExportBundle streams a component bundle to w as the server produces it.
func (c *Client) ExportBundle(ctx context.Context, in core.BundleExportInput, w io.Writer) error {
	body, contentType, err := encodeBody(in)
	if err != nil {
		return err
	}

	req, err := c.newRequest(ctx, http.MethodPost, httpapi.RouteBundleExport, nil, body)
	if err != nil {
		return err
	}
	req.Header.Set(httpapi.HeaderContentType, contentType)
	req.Header.Set(httpapi.HeaderAccept, httpapi.ContentBundle)

	resp, err := c.send(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if _, err := io.Copy(w, resp.Body); err != nil {
		return core.Internal("streaming bundle export: %v", err).Wrap(err)
	}
	return nil
}

// ImportBundle streams a bundle from r to the server and returns its plan.
func (c *Client) ImportBundle(ctx context.Context, r io.Reader, in core.BundleImportInput) (*core.BundleResult, error) {
	q := url.Values{}
	if in.OnCollision != "" {
		q.Set("on_collision", string(in.OnCollision))
	}
	if in.Preview {
		q.Set("preview", strconv.FormatBool(true))
	}
	if in.ProjectRef != "" {
		q.Set("project_ref", in.ProjectRef)
	}

	req, err := c.newRequest(ctx, http.MethodPost, httpapi.RouteBundleImport, q, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set(httpapi.HeaderContentType, httpapi.ContentBundle)

	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	out := new(core.BundleResult)
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
		return nil, core.Internal("decoding bundle result: %v", err).Wrap(err)
	}
	return out, nil
}
