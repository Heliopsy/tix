// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"github.com/heliopsy/tix/api"
	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/mcp"
	"github.com/heliopsy/tix/internal/version"
	"github.com/spf13/cobra"
)

// mcpOptions are the flags only the mcp command takes.
type mcpOptions struct {
	overlay          string
	allowDestructive bool
	list             bool
}

// newMCPCmd builds the command that serves the API as MCP tools over stdio.
func newMCPCmd(g *globals) *cobra.Command {
	var o mcpOptions
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve the tix API as MCP tools over stdio",
		Long: "Serve the tix HTTP API to a Model Context Protocol client over standard input and output. " +
			"Every JSON operation of the API is one tool, generated from the OpenAPI document this " +
			"binary carries, and each call is one request to the configured server with the configured token.\n\n" +
			"Destructive tools are left out unless --allow-destructive is given or an overlay enables them, " +
			"and tools whose scope the token lacks are not listed. --list prints every operation and what " +
			"became of it without contacting a server. See docs/mcp.md.\n\n" +
			"Exit codes: 2 no server is configured or the overlay is invalid, 5 the server refused the token.",
		Example: "  tix mcp --server https://tix.example.com --token \"$TIX_TOKEN\"\n" +
			"  tix mcp --list\n" +
			"  tix mcp --allow-destructive --overlay ~/.config/tix/mcp.yaml",
		GroupID: "setup",
		Args:    noArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return runMCP(cmd, g, o) },
	}
	f := cmd.Flags()
	f.StringVar(&o.overlay, "overlay", "", "overlay file laid over the shipped one")
	f.BoolVar(&o.allowDestructive, "allow-destructive", false, "also offer destructive tools")
	f.BoolVar(&o.list, "list", false, "print every operation and its tool state, then exit")
	return cmd
}

func runMCP(cmd *cobra.Command, g *globals, o mcpOptions) error {
	raw, err := api.JSON()
	if err != nil {
		return core.Internal("reading the embedded openapi document").Wrap(err)
	}
	doc, err := mcp.LoadDocument(raw)
	if err != nil {
		return core.Internal("reading the embedded openapi document").Wrap(err)
	}
	ov, err := mcp.LoadOverlay(o.overlay)
	if err != nil {
		return core.Invalid("%v", err)
	}
	opts := mcp.Options{AllowDestructive: o.allowDestructive}
	if o.list {
		tools, err := mcp.Plan(doc, ov, opts)
		if err != nil {
			return core.Invalid("%v", err)
		}
		return g.render(cmd, mcp.Rows(tools))
	}

	resolved, err := g.resolve()
	if err != nil {
		return err
	}
	target, err := connect.Resolve(resolved, g.overrides())
	if err != nil {
		return err
	}
	if target.Mode != connect.ModeRemote {
		return usagef(cmd, "tix mcp talks to a tix server: pass --server, or select a context that names one")
	}
	g.clientHint = core.ClientHintMCP
	conn, ctx, err := g.dial(cmd)
	if err != nil {
		return err
	}
	remote, ok := conn.Service.(*client.Client)
	if !ok {
		return core.Internal("the remote target opened no http client")
	}
	opts.HasScope = func(s string) bool { return conn.Actor.HasScope(core.Scope(s)) }
	tools, err := mcp.Plan(doc, ov, opts)
	if err != nil {
		return core.Invalid("%v", err)
	}
	g.diag(cmd, "tix mcp: %d tools for %s on %s", len(mcp.Exposed(tools)), conn.Actor.Handle, target.URL)
	return mcp.Serve(ctx, mcp.NewServer(tools, remote, version.Version), cmd.InOrStdin(), cmd.OutOrStdout())
}

func init() { builders = append(builders, newMCPCmd) }
