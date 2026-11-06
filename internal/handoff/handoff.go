// SPDX-License-Identifier: AGPL-3.0-or-later

// Package handoff writes the text that hands one task to a coding agent: a
// prompt for an agent driving the tix CLI, the same prompt for one holding the
// tix MCP tools, and the command that starts each documented client on it.
//
// None of it carries a token. The agent brings its own, through its tix
// configuration or the environment its MCP client starts `tix mcp` with.
package handoff

import (
	"strings"
	"unicode"
)

// Task is what a hand-off names: the task's reference as a reader sees it,
// its title, and the address of the server it lives on.
type Task struct {
	Ref    string
	Title  string
	Server string
}

// Prompt is the instruction for an agent that runs the tix CLI.
func Prompt(t Task) string {
	ref := t.Ref
	return intro(t) + " " +
		"Claim it first with `tix claim task " + ref + " -o json` and keep the lease_token it prints. " +
		"Read it with `tix task show " + ref + "`, post progress as comments with `tix comment add " + ref + " \"...\"`, " +
		"and when it is finished close it with `tix claim release " + ref + " --token <lease_token> --status done --comment \"what you did\"`."
}

// MCPTools are the tools the MCP prompt names, in the order it names them.
var MCPTools = []string{"claim_task", "task_show", "comment_add", "claim_release"}

// MCPPrompt is the instruction for an agent that holds the tix MCP tools.
func MCPPrompt(t Task) string {
	return intro(t) + " Use the tix MCP tools. " +
		"Claim it first with " + MCPTools[0] + " and keep the lease_token it returns. " +
		"Read it with " + MCPTools[1] + ", post progress as comments with " + MCPTools[2] + ", " +
		"and when it is finished close it with " + MCPTools[3] + ", passing the lease token, status done and a comment saying what you did."
}

func intro(t Task) string {
	return "Work on tix task " + t.Ref + " (\"" + OneLine(t.Title) + "\") on the tix server at " + t.Server + "."
}

// OneLine folds every run of whitespace and control characters into one
// space, so a title cannot break a one-line command over two lines.
func OneLine(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}), " ")
}

// Quote makes s one word a POSIX shell passes on unexpanded: single quotes,
// inside which nothing is special, with each single quote closed, escaped
// and reopened.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Client is one agent a task can be handed to: its name, the words that
// start it with a prompt when it has such a command, the page documenting
// that command, and whether it takes the MCP prompt rather than the CLI one.
type Client struct {
	ID     string
	Name   string
	Launch []string
	Doc    string
	MCP    bool
}

// Clients are the clients docs/mcp.md documents, in its order.
var Clients = []Client{
	{ID: "claude-code", Name: "Claude Code", Launch: []string{"claude"}, Doc: "https://code.claude.com/docs/en/cli-reference"},
	{ID: "codex", Name: "Codex", Launch: []string{"codex"}, Doc: "https://developers.openai.com/codex/cli/reference"},
	{ID: "opencode", Name: "OpenCode", Launch: []string{"opencode", "--prompt"}, Doc: "https://opencode.ai/docs/cli/"},
	{ID: "gemini", Name: "Gemini CLI", Launch: []string{"gemini", "-i"}, Doc: "https://geminicli.com/docs/cli/cli-reference/"},
	{ID: "claude-desktop", Name: "Claude Desktop", MCP: true},
	{ID: "cursor", Name: "Cursor", Launch: []string{"agent"}, Doc: "https://cursor.com/docs/cli/overview"},
	{ID: "vscode", Name: "VS Code", Launch: []string{"code", "chat"}, Doc: "https://code.visualstudio.com/docs/configure/command-line"},
	{ID: "zed", Name: "Zed", MCP: true},
}

// Snippet is one client's hand-off: the command to run, or empty when the
// client has none, and the prompt it carries.
type Snippet struct {
	Client
	Command string
	Prompt  string
}

// For is the hand-off of one task to every client.
func For(t Task) []Snippet {
	cli, mcp := Prompt(t), MCPPrompt(t)
	out := make([]Snippet, 0, len(Clients))
	for _, c := range Clients {
		s := Snippet{Client: c, Prompt: cli}
		if c.MCP {
			s.Prompt = mcp
		}
		if len(c.Launch) > 0 {
			s.Command = strings.Join(c.Launch, " ") + " " + Quote(s.Prompt)
		}
		out = append(out, s)
	}
	return out
}
