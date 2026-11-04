// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"bytes"
	"context"
	"io"
	"net/url"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// maxRawBody bounds a response Raw reads, so one listing cannot exhaust memory.
const maxRawBody = 32 << 20

// Raw sends one request built from the API's own description and returns the
// response body undecoded. A failing response comes back as the server's
// domain error, exactly as every typed method reports it.
func (c *Client) Raw(ctx context.Context, method, path string, q url.Values, body []byte) ([]byte, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := c.newRequest(ctx, method, path, q, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set(wire.HeaderContentType, wire.ContentJSON)
	}
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRawBody))
	if err != nil {
		return nil, core.Internal("reading response from %s: %v", req.URL.String(), err).Wrap(err)
	}
	return raw, nil
}
