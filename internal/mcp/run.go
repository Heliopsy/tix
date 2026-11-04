// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp

import (
	"context"
	"fmt"
	"io"
	"os"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// LoadOverlay returns the shipped overlay with the file at path, if any,
// laid over it.
func LoadOverlay(path string) (Overlay, error) {
	ov, err := BuiltinOverlay()
	if err != nil || path == "" {
		return ov, err
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- the operator names their own overlay file
	if err != nil {
		return Overlay{}, fmt.Errorf("reading overlay %q: %w", path, err)
	}
	user, err := ParseOverlay(raw)
	if err != nil {
		return Overlay{}, fmt.Errorf("overlay %q: %w", path, err)
	}
	return ov.Merge(user), nil
}

// Row is one operation as a listing shows it.
type Row struct {
	Tool      string `json:"tool" yaml:"tool"`
	Operation string `json:"operation" yaml:"operation"`
	Method    string `json:"method" yaml:"method"`
	Path      string `json:"path" yaml:"path"`
	Scope     string `json:"scope,omitempty" yaml:"scope,omitempty"`
	State     State  `json:"state" yaml:"state"`
	Reason    string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// Rows renders a plan for a listing, in operation order.
func Rows(tools []Tool) []Row {
	rows := make([]Row, 0, len(tools))
	for _, t := range tools {
		rows = append(rows, Row{Tool: t.Name, Operation: t.Op.ID, Method: t.Op.Method, Path: t.Op.Path,
			Scope: t.Op.Scope, State: t.State, Reason: t.Reason})
	}
	return rows
}

// Serve runs the server over a newline-delimited JSON-RPC stream until the
// client closes it or ctx ends.
func Serve(ctx context.Context, srv *sdk.Server, in io.Reader, out io.Writer) error {
	return srv.Run(ctx, &sdk.IOTransport{Reader: io.NopCloser(in), Writer: nopCloser{out}})
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
