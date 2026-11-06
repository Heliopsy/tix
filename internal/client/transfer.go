// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// ExportTo streams a snapshot to w as the server produces it.
func (c *Client) ExportTo(ctx context.Context, in core.ExportInput, w io.Writer) error {
	body, contentType, err := encodeBody(in)
	if err != nil {
		return err
	}

	req, err := c.newRequest(ctx, http.MethodPost, wire.RouteExport, nil, body)
	if err != nil {
		return err
	}
	req.Header.Set(wire.HeaderContentType, contentType)
	req.Header.Set(wire.HeaderAccept, wire.ContentNDJSON)

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

	req, err := c.newRequest(ctx, http.MethodPost, wire.RouteImport, q, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set(wire.HeaderContentType, wire.ContentNDJSON)

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

// ExportTasks streams one project's tasks to w as the server writes them.
func (c *Client) ExportTasks(ctx context.Context, in core.TaskExportInput, w io.Writer) error {
	body, contentType, err := encodeBody(in)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, http.MethodPost, routePath(wire.RouteProjectTaskExport, "ref", in.ProjectRef), nil, body)
	if err != nil {
		return err
	}
	req.Header.Set(wire.HeaderContentType, contentType)
	resp, err := c.send(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.Copy(w, resp.Body); err != nil {
		return core.Internal("streaming task export: %v", err).Wrap(err)
	}
	return nil
}

// ImportTasks uploads a file of tasks as a multipart form and returns the
// server's report.
func (c *Client) ImportTasks(ctx context.Context, r io.Reader, in core.TaskImportInput) (*core.TaskImportResult, error) {
	pr, pw := io.Pipe()
	form := multipart.NewWriter(pw)
	go func() { _ = pw.CloseWithError(writeImportForm(form, r, in)) }()

	req, err := c.newRequest(ctx, http.MethodPost, routePath(wire.RouteProjectTaskImport, "ref", in.ProjectRef), nil, pr)
	if err != nil {
		_ = pr.Close()
		return nil, err
	}
	req.Header.Set(wire.HeaderContentType, form.FormDataContentType())
	resp, err := c.send(req)
	if err != nil {
		_ = pr.Close()
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	out := new(core.TaskImportResult)
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, core.Internal("decoding import result: %v", err).Wrap(err)
	}
	return out, nil
}

// writeImportForm writes the choices as fields and the file as its part.
func writeImportForm(form *multipart.Writer, r io.Reader, in core.TaskImportInput) error {
	fields := map[string]string{
		"format": string(in.Format), "mode": string(in.Mode), "mapping": in.Mapping,
		"preview": strconv.Itoa(in.Preview), "dry_run": strconv.FormatBool(in.DryRun),
	}
	for _, k := range []string{"format", "mode", "mapping", "preview", "dry_run"} {
		if err := form.WriteField(k, fields[k]); err != nil {
			return err
		}
	}
	name := in.Filename
	if name == "" {
		name = "tasks"
	}
	part, err := form.CreateFormFile("file", name)
	if err != nil {
		return err
	}
	if r == nil {
		r = strings.NewReader(in.Content)
	}
	if _, err := io.Copy(part, r); err != nil {
		return err
	}
	return form.Close()
}
