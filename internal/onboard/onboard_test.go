// SPDX-License-Identifier: AGPL-3.0-or-later

package onboard

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/docsmd"
)

// docsTarget fills the snippets the way docs/mcp.md writes them.
var docsTarget = Target{URL: "URL", Tenant: "acme", Token: "TOKEN", Version: "dev"}

func mcpDoc(t *testing.T) string {
	t.Helper()
	md, err := docsmd.Read("docs/mcp.md")
	if err != nil {
		t.Fatal(err)
	}
	return md
}

// TestMCPSnippetsAreTheDocs fails when the connect screen and docs/mcp.md
// disagree about a client: each snippet must appear, fenced in its language,
// under the heading that names its client, and every client heading of the
// docs must be one the screen offers.
func TestMCPSnippetsAreTheDocs(t *testing.T) {
	md := mcpDoc(t)
	clients := MCPClients(docsTarget)
	wiring, err := docsmd.Section(md, "## Wiring it into a client")
	if err != nil {
		t.Fatal(err)
	}
	offered := map[string]bool{}
	for _, c := range clients {
		offered[c.Name] = true
		section, err := docsmd.Section(md, "### "+c.Name)
		if err != nil {
			t.Errorf("docs/mcp.md has no section for %s: %v", c.Name, err)
			continue
		}
		for _, s := range c.Snippets {
			fenced := "```" + s.Lang + "\n" + s.Text + "\n```"
			if !strings.Contains(section, fenced) {
				t.Errorf("docs/mcp.md, %s, has no %s block reading:\n%s", c.Name, s.Lang, s.Text)
			}
		}
	}
	for _, line := range strings.Split(wiring, "\n") {
		if name, ok := strings.CutPrefix(line, "### "); ok && !offered[name] {
			t.Errorf("docs/mcp.md documents %s, which the connect screen does not offer", name)
		}
	}
	if !strings.Contains(md, "```sh\n"+TokenCommand()+"\n```") {
		t.Errorf("docs/mcp.md does not mint the agent token as the screen does: %s", TokenCommand())
	}
}

// TestJSONSnippetsParse fails on a JSON snippet a client would refuse,
// including the one the VS Code command line carries in quotes.
func TestJSONSnippetsParse(t *testing.T) {
	for _, target := range []Target{docsTarget, {URL: `https://a"b`, Tenant: "t", Token: `x\y`}} {
		var n int
		for _, c := range MCPClients(target) {
			for _, s := range c.Snippets {
				text := s.Text
				if s.Lang == "sh" {
					start, end := strings.Index(text, "'{"), strings.LastIndex(text, "}'")
					if start < 0 {
						continue
					}
					text = text[start+1 : end+1]
				} else if s.Lang != "json" {
					continue
				}
				n++
				var v map[string]any
				if err := json.Unmarshal([]byte(text), &v); err != nil {
					t.Errorf("%s %s does not parse: %v\n%s", c.Name, s.Title, err, text)
				}
			}
		}
		if n < 8 {
			t.Fatalf("parsed %d JSON snippets, want every client's", n)
		}
	}
}

func TestInstallPinsTheRunningRelease(t *testing.T) {
	cases := []struct {
		version, want, absent string
	}{
		{"0.41.0", "TIX_VERSION=v0.41.0", "@latest"},
		{"v0.41.0", "ghcr.io/heliopsy/tix:0.41.0", ":latest"},
		{"dev", "go install github.com/heliopsy/tix@latest", "TIX_VERSION"},
	}
	for _, tc := range cases {
		var all []string
		for _, s := range Install(Target{URL: "u", Token: "t", Version: tc.version}) {
			all = append(all, s.Text)
		}
		joined := strings.Join(all, "\n")
		if !strings.Contains(joined, tc.want) || strings.Contains(joined, tc.absent) {
			t.Errorf("version %s: install reads\n%s\nwant %q and no %q", tc.version, joined, tc.want, tc.absent)
		}
	}
}

func TestSSHAddress(t *testing.T) {
	cases := []struct{ addr, want string }{
		{"tix.example.com:2020", "ssh -p 2020 acme@tix.example.com"},
		{"tix.example.com:22", "ssh acme@tix.example.com"},
		{"tix.example.com", "ssh acme@tix.example.com"},
		{"[::1]:2222", "ssh -p 2222 acme@::1"},
	}
	for _, tc := range cases {
		got := SSH(Target{Tenant: "acme", SSH: tc.addr})
		if len(got) < 2 || got[1].Text != tc.want {
			t.Errorf("SSH(%q) = %+v, want %q", tc.addr, got, tc.want)
		}
	}
	if got := SSH(Target{SSH: "h:2020"}); got[1].Text != "ssh -p 2020 tix@h" {
		t.Errorf("SSH with no tenant key = %q, want the neutral user", got[1].Text)
	}
	if got := SSH(Target{Tenant: "acme"}); got != nil {
		t.Errorf("SSH with no listener = %+v, want nothing", got)
	}
}

func TestShellWordQuotesOnlyWhatNeedsIt(t *testing.T) {
	for in, want := range map[string]string{
		"https://tix.example.com:8443": "https://tix.example.com:8443",
		"it's":                         `'it'\''s'`,
		"a b":                          "'a b'",
	} {
		if got := shellWord(in); got != want {
			t.Errorf("shellWord(%q) = %q, want %q", in, got, want)
		}
	}
}
