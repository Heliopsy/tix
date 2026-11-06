// SPDX-License-Identifier: AGPL-3.0-or-later

package handoff_test

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/api"
	"github.com/heliopsy/tix/internal/handoff"
	"github.com/heliopsy/tix/internal/mcp"
	"github.com/heliopsy/tix/internal/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }

var hostileTitles = []string{
	`plain title`,
	`it's here`,
	`say "hi"`,
	`$(touch /tmp/pwned) and ${HOME}`,
	"`whoami` ran",
	"two\nlines\r\nand\ta tab",
	`back\slash \' and '\''`,
	`!event #comment ; & | > < * ? ~`,
	"\x00nul\x1bescape separator",
	`'`,
	``,
}

// TestCommandsReachTheClientAsOneWord runs every generated command through a
// real POSIX shell with the client's name swapped for a printer, and checks
// the argument after the launch words is the prompt, byte for byte.
func TestCommandsReachTheClientAsOneWord(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		testenv.Skip(t, testenv.Capability{Name: "sh", Why: "sh is not on PATH", How: "install a POSIX shell"})
	}
	for _, title := range hostileTitles {
		task := handoff.Task{Ref: "INFRA-12", Title: title, Server: "https://tix.example.com"}
		for _, s := range handoff.For(task) {
			if s.Command == "" {
				continue
			}
			rest, ok := strings.CutPrefix(s.Command, strings.Join(s.Launch, " ")+" ")
			if !ok {
				t.Fatalf("%s: %q does not start with its launch words", s.Name, s.Command)
			}
			cmd := exec.Command(sh, "-c", `f() { printf '%s\n' "$#"; printf '%s' "$1"; }; f `+rest)
			got, err := cmd.Output()
			if err != nil {
				t.Fatalf("%s, title %q: %v", s.Name, title, err)
			}
			count, arg, _ := strings.Cut(string(got), "\n")
			if count != "1" || arg != s.Prompt {
				t.Errorf("%s, title %q: shell saw %s words, first %q, want 1 word %q", s.Name, title, count, arg, s.Prompt)
			}
			if strings.ContainsAny(s.Command, "\n\r") {
				t.Errorf("%s, title %q: the command runs over more than one line", s.Name, title)
			}
		}
	}
}

func TestThePromptNamesTheTaskAndTheServer(t *testing.T) {
	task := handoff.Task{Ref: "INFRA-12", Title: "Fix the\nbuild", Server: "https://tix.example.com"}
	for name, p := range map[string]string{"cli": handoff.Prompt(task), "mcp": handoff.MCPPrompt(task)} {
		for _, want := range []string{"INFRA-12", `"Fix the build"`, "https://tix.example.com"} {
			if !strings.Contains(p, want) {
				t.Errorf("%s prompt %q does not name %q", name, p, want)
			}
		}
	}
	if !strings.Contains(handoff.Prompt(task), "`tix claim task INFRA-12 -o json`") {
		t.Error("the CLI prompt does not claim the task by its reference")
	}
}

func TestEveryDocumentedClientHasATab(t *testing.T) {
	raw, err := os.ReadFile("../../docs/mcp.md")
	if err != nil {
		t.Fatal(err)
	}
	var documented []string
	inWiring := false
	for line := range strings.SplitSeq(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			inWiring = line == "## Wiring it into a client"
		case inWiring && strings.HasPrefix(line, "### "):
			documented = append(documented, strings.TrimPrefix(line, "### "))
		}
	}
	if len(documented) < 5 {
		t.Fatalf("read %d clients from docs/mcp.md, want the reader to still work", len(documented))
	}
	var names []string
	for _, c := range handoff.Clients {
		names = append(names, c.Name)
		if len(c.Launch) > 0 && !strings.HasPrefix(c.Doc, "https://") {
			t.Errorf("%s has a command and no documentation for it", c.Name)
		}
	}
	slices.Sort(documented)
	slices.Sort(names)
	if !slices.Equal(documented, names) {
		t.Errorf("clients %v, docs/mcp.md documents %v", names, documented)
	}
}

func TestTheMCPPromptNamesOnlyExposedTools(t *testing.T) {
	raw, err := api.JSON()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := mcp.LoadDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	ov, err := mcp.BuiltinOverlay()
	if err != nil {
		t.Fatal(err)
	}
	tools, err := mcp.Plan(doc, ov, mcp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	exposed := map[string]bool{}
	for _, tool := range mcp.Exposed(tools) {
		exposed[tool.Name] = true
	}
	p := handoff.MCPPrompt(handoff.Task{Ref: "A-1", Title: "t", Server: "http://x"})
	for _, name := range handoff.MCPTools {
		if !exposed[name] {
			t.Errorf("the MCP prompt names %s, which tix mcp does not expose", name)
		}
		if !strings.Contains(p, name) {
			t.Errorf("the MCP prompt does not name %s", name)
		}
	}
}

func TestNoHandoffCarriesAToken(t *testing.T) {
	for _, s := range handoff.For(handoff.Task{Ref: "A-1", Title: "t", Server: "http://x"}) {
		for _, text := range []string{s.Command, s.Prompt} {
			if strings.Contains(text, "TIX_TOKEN") || strings.Contains(text, "--token ") && !strings.Contains(text, "--token <lease_token>") {
				t.Errorf("%s: %q passes a token", s.Name, text)
			}
		}
	}
}
