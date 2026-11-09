// SPDX-License-Identifier: AGPL-3.0-or-later

// Package onboard builds the commands and configuration snippets that connect
// a CLI, an MCP client or an agent to one tix server. The browser's Connect an
// agent screen renders them, and tests hold them to docs/mcp.md and to the
// real command tree, so the screen and the docs cannot drift apart.
package onboard

import (
	"encoding/json"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// Target is what every snippet is filled in with.
type Target struct {
	// URL is the server's base address, such as https://tix.example.com.
	URL string
	// Tenant is the tenant key the reader works in. Empty when the reader
	// may not read it; a token already names its tenant, and an SSH key
	// enrolled in one tenant connects as tix.
	Tenant string
	// Token is written wherever a command needs the API token.
	Token string
	// Version is the running version; a release pins the install commands to it.
	Version string
	// SSH is where people reach the SSH listener, host or host:port. Empty
	// means the server runs none.
	SSH string
}

// Snippet is one copyable block.
type Snippet struct {
	// ID is unique on the screen and safe as an HTML id.
	ID string
	// Title names the snippet: a tab label or a heading.
	Title string
	// Path is the file the snippet goes in, named instead of a title.
	Path string
	// Note is one sentence about where it goes or what it does.
	Note string
	// Lang is the fence language docs/mcp.md writes it under: sh, json,
	// toml or powershell.
	Lang string
	// Text is the snippet itself.
	Text string
}

// Client is one MCP client and the ways to register tix with it.
type Client struct {
	ID string
	// Name is the client's name, which is also its heading in docs/mcp.md.
	Name     string
	Snippets []Snippet
}

// Preset is a named scope bundle the token form offers.
type Preset struct {
	ID     string
	Name   string
	Note   string
	Scopes []core.Scope
}

// TokenName is the name the docs and the token form give an agent's token.
const TokenName = "mcp-agent"

// SkillURL is where the agent skill is published.
const SkillURL = "https://tix.red/skill.md"

// SkillReferences are the reference files beside SKILL.md, published under
// https://tix.red/reference/. A test checks them against skills/tix/reference.
var SkillReferences = []string{"queries", "recovery", "setup", "ssh"}

// Presets are the scope bundles the token form offers besides a custom set.
var Presets = []Preset{
	{ID: "agent", Name: "Agent", Note: "Read, create and edit tasks, claim them, move them and comment.",
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeTaskWrite, core.ScopeTaskTransition,
			core.ScopeTaskClaim, core.ScopeCommentWrite}},
	{ID: "read", Name: "Read only", Note: "Read tasks, projects and workflows, and follow the event stream.",
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead, core.ScopeWorkflowRead,
			core.ScopeEventSubscribe}},
}

// AgentScopes are the agent preset's scopes.
func AgentScopes() []core.Scope { return Presets[0].Scopes }

// TokenCommand mints the agent token from the command line.
func TokenCommand() string {
	var b strings.Builder
	b.WriteString("tix token create " + TokenName)
	for _, s := range AgentScopes() {
		b.WriteString(" --scope " + string(s))
	}
	return b.String()
}

// Rotate replaces the agent token, keeping the old value working for ten
// minutes so a running agent can pick up the new one.
func Rotate() Snippet {
	return Snippet{ID: "token-rotate", Title: "Rotate it later", Lang: "sh",
		Text: "tix token rotate " + TokenName + " --grace 10m",
		Note: "Issues a new value with the same name and scopes, and keeps the old one working for ten minutes so a running agent is not cut off."}
}

// Login signs in with a password and stores a token for the tenant on a
// context named after it, or nil when the tenant or the address is unknown.
func Login(t Target, email string) *Snippet {
	if t.Tenant == "" || email == "" {
		return nil
	}
	return &Snippet{ID: "cli-login", Title: "Or sign in with your password", Lang: "sh",
		Text: "tix --server " + shellWord(t.URL) + " login --tenant " + shellWord(t.Tenant) +
			" --email " + shellWord(email) + " --password -",
		Note: "Asks for your password, then mints a token with every scope you hold and saves it on a context named after the tenant. It needs the admin role here; add --ssh-key in place of --email and --password to sign in with an enrolled key."}
}

// release matches a version that names a published release.
var release = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)

// tag is the release tag of the running version, or "" for a development build.
func (t Target) tag() string {
	if !release.MatchString(t.Version) {
		return ""
	}
	return "v" + strings.TrimPrefix(t.Version, "v")
}

// Install lists the ways to put the CLI on a machine, pinned to the running
// release where there is one, so the client matches the server.
func Install(t Target) []Snippet {
	tag := t.tag()
	unix, windows, goGet, image := "curl -fsSL https://tix.red/install.sh | sh",
		"irm https://tix.red/install.ps1 | iex", "go install github.com/heliopsy/tix@latest",
		"ghcr.io/heliopsy/tix:latest"
	if tag != "" {
		unix = "curl -fsSL https://tix.red/install.sh | TIX_VERSION=" + tag + " sh"
		windows = `$env:TIX_VERSION = "` + tag + `"; ` + windows
		goGet = "go install github.com/heliopsy/tix@" + tag
		image = "ghcr.io/heliopsy/tix:" + strings.TrimPrefix(tag, "v")
	}
	return []Snippet{
		{ID: "install-unix", Title: "Linux and macOS", Lang: "sh", Text: unix,
			Note: "Downloads the release for this OS and CPU, checks its checksum and puts tix on your PATH."},
		{ID: "install-windows", Title: "Windows", Lang: "powershell", Text: windows,
			Note: "The same install in PowerShell."},
		{ID: "install-go", Title: "Go", Lang: "sh", Text: goGet,
			Note: "Builds tix with your Go toolchain."},
		{ID: "install-container", Title: "Container", Lang: "sh",
			Text: "podman run --rm -e TIX_SERVER_URL=" + shellWord(t.URL) + " -e TIX_TOKEN=" + t.Token + " " + image + " task ls",
			Note: "Runs one command from the published image, with nothing installed. Docker takes the same arguments."},
	}
}

// Verify is the command that proves the install and the connection work.
func Verify() Snippet {
	return Snippet{ID: "cli-verify", Title: "Check the connection", Lang: "sh", Text: "tix task ls --limit 5",
		Note: "Lists five tasks from this server. An error names what is wrong: the address, the token or a scope."}
}

// CLI points the command line at the server with a named context.
func CLI(t Target) []Snippet {
	return []Snippet{
		{ID: "cli-ctx", Title: "Save a context", Lang: "sh",
			Text: t.ctxAdd(),
			Note: "Stores the server, token and tenant under one name and makes it the default, so later commands need no flags."},
		Verify(),
	}
}

// ctxAdd names the context after the tenant, and names the tenant when it is known.
func (t Target) ctxAdd() string {
	name, tenant := "tix", ""
	if t.Tenant != "" {
		name, tenant = shellWord(t.Tenant), " --tenant "+shellWord(t.Tenant)
	}
	return "tix ctx add " + name + " --server " + shellWord(t.URL) + " --token " + t.Token + tenant + " --use"
}

// MCPPreview lists the tools tix mcp would serve, without contacting a server.
func MCPPreview() Snippet {
	return Snippet{ID: "mcp-list", Title: "See the tools", Lang: "sh", Text: "tix mcp --list",
		Note: "Prints every tool, the scope it needs and whether it is offered. It needs no server."}
}

// MCPClients are the clients docs/mcp.md names, each with its snippets in
// the order the docs give them.
func MCPClients(t Target) []Client {
	env := `--env TIX_SERVER_URL=` + shellWord(t.URL) + ` --env TIX_TOKEN=` + t.Token + ` -- tix mcp`
	clients := []Client{
		{ID: "claude-code", Name: "Claude Code", Snippets: []Snippet{
			{Title: "Command line", Lang: "sh", Text: "claude mcp add tix " + env},
			{Path: ".mcp.json", Note: "At the root of a project.", Lang: "json",
				Text: serversJSON("mcpServers", t, nil)},
		}},
		{ID: "codex", Name: "Codex", Snippets: []Snippet{
			{Title: "Command line", Lang: "sh", Text: "codex mcp add tix " + env},
			{Path: "~/.codex/config.toml", Note: "Shared by the Codex CLI and IDE extension.", Lang: "toml",
				Text: "[mcp_servers.tix]\ncommand = \"tix\"\nargs = [\"mcp\"]\n\n[mcp_servers.tix.env]\n" +
					"TIX_SERVER_URL = " + quote(t.URL) + "\nTIX_TOKEN = " + quote(t.Token)},
		}},
		{ID: "opencode", Name: "OpenCode", Snippets: []Snippet{
			{Path: "opencode.json", Lang: "json", Text: openCodeJSON(t),
				Note: "The command and its arguments are one array, and the variables go under environment."},
		}},
		{ID: "gemini", Name: "Gemini CLI", Snippets: []Snippet{
			{Path: ".gemini/settings.json", Lang: "json", Text: serversJSON("mcpServers", t, nil),
				Note: "In a project, or ~/.gemini/settings.json for every project. Gemini CLI starts it only in a trusted folder."},
		}},
		{ID: "claude-desktop", Name: "Claude Desktop", Snippets: []Snippet{
			{Path: "claude_desktop_config.json", Lang: "json", Text: serversJSON("mcpServers", t, nil),
				Note: "Quit and reopen the app after editing it."},
		}},
		{ID: "cursor", Name: "Cursor", Snippets: []Snippet{
			{Path: ".cursor/mcp.json", Lang: "json", Text: serversJSON("mcpServers", t, []string{`"type": "stdio"`}),
				Note: "In a project, or ~/.cursor/mcp.json for every project."},
		}},
		{ID: "vscode", Name: "VS Code", Snippets: []Snippet{
			{Path: ".vscode/mcp.json", Lang: "json", Text: serversJSON("servers", t, []string{`"type": "stdio"`}),
				Note: "In a workspace, for GitHub Copilot's agent mode."},
			{Title: "Command line", Lang: "sh", Note: "Adds the same server to your user profile.",
				Text: `code --add-mcp '{"name":"tix","type":"stdio","command":"tix","args":["mcp"],"env":{"TIX_SERVER_URL":` +
					quote(t.URL) + `,"TIX_TOKEN":` + quote(t.Token) + `}}'`},
		}},
		{ID: "zed", Name: "Zed", Snippets: []Snippet{
			{Path: "settings.json", Lang: "json", Text: serversJSON("context_servers", t, nil),
				Note: "Zed's settings file, opened with zed: open settings file."},
		}},
	}
	for _, c := range clients {
		for i := range c.Snippets {
			c.Snippets[i].ID = "mcp-" + c.ID + "-" + strconv.Itoa(i+1)
		}
	}
	return clients
}

// serversJSON is the shape most clients share: a map of servers under top,
// each a command, its arguments and an environment.
func serversJSON(top string, t Target, lead []string) string {
	var b strings.Builder
	b.WriteString("{\n  \"" + top + "\": {\n    \"tix\": {\n")
	for _, l := range lead {
		b.WriteString("      " + l + ",\n")
	}
	b.WriteString("      \"command\": \"tix\",\n      \"args\": [\"mcp\"],\n      \"env\": {\n")
	b.WriteString("        \"TIX_SERVER_URL\": " + quote(t.URL) + ",\n")
	b.WriteString("        \"TIX_TOKEN\": " + quote(t.Token) + "\n")
	b.WriteString("      }\n    }\n  }\n}")
	return b.String()
}

// openCodeJSON is OpenCode's shape, which differs in every key.
func openCodeJSON(t Target) string {
	return "{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"mcp\": {\n    \"tix\": {\n" +
		"      \"type\": \"local\",\n      \"command\": [\"tix\", \"mcp\"],\n      \"enabled\": true,\n" +
		"      \"environment\": {\n        \"TIX_SERVER_URL\": " + quote(t.URL) + ",\n" +
		"        \"TIX_TOKEN\": " + quote(t.Token) + "\n      }\n    }\n  }\n}"
}

// Skill installs the agent skill.
func Skill() []Snippet {
	fetch := func(dir string) string {
		return "mkdir -p " + dir + "/reference\n" +
			"curl -fsSL " + SkillURL + " -o " + dir + "/SKILL.md\n" +
			"for f in " + strings.Join(SkillReferences, " ") + "; do\n" +
			"  curl -fsSL https://tix.red/reference/$f.md -o " + dir + "/reference/$f.md\n" +
			"done"
	}
	return []Snippet{
		{ID: "skill-project", Title: "Claude Code, this project", Lang: "sh", Text: fetch(".claude/skills/tix"),
			Note: "Run it at the root of the repository. Commit the folder so everyone working there gets it."},
		{ID: "skill-user", Title: "Claude Code, every project", Lang: "sh", Text: fetch("~/.claude/skills/tix"),
			Note: "Installs it for you alone, in every project."},
		{ID: "skill-other", Title: "Other agents", Lang: "sh",
			Text: "printf '%s\\n' 'Track work in tix. Read " + SkillURL + " before you use it.' >> AGENTS.md",
			Note: "Codex, OpenCode, Gemini CLI and most other agents read AGENTS.md. Any agent that can fetch a URL can follow the skill."},
	}
}

// SSH lists the terminal interface commands, or nothing when the server runs
// no SSH listener.
func SSH(t Target) []Snippet {
	if t.SSH == "" {
		return nil
	}
	host, port := splitHostPort(t.SSH)
	dest := "ssh "
	if port != "" && port != "22" {
		dest += "-p " + port + " "
	}
	user := t.Tenant
	if user == "" {
		user = "tix"
	}
	dest += user + "@" + host
	return []Snippet{
		{ID: "ssh-key", Title: "Enrol your key", Lang: "sh", Text: "tix user key add --file ~/.ssh/id_ed25519.pub",
			Note: "Once, from a configured CLI. Add --all-tenants to enrol it in every tenant you belong to. The SSH keys screen does the same in the browser."},
		{ID: "ssh-tui", Title: "Open the terminal interface", Lang: "sh", Text: dest},
		{ID: "ssh-cmd", Title: "Run a tix command", Lang: "sh", Text: dest + " task ls -o json",
			Note: "Runs on the server as you, with nothing installed or configured here. The server reads no file of yours: redirect with < and > instead."},
		{ID: "ssh-export", Title: "Export over SSH", Lang: "sh", Text: dest + " export > snapshot.ndjson",
			Note: "Streams the tenant's snapshot to your machine. It needs the export scope."},
		{ID: "ssh-watch", Title: "Stream events", Lang: "sh", Text: dest + " watch -o ndjson",
			Note: "One JSON event per line, for a script. Add -t after ssh for the coloured view a person reads."},
	}
}

// splitHostPort reads host or host:port, dropping IPv6 brackets.
func splitHostPort(addr string) (string, string) {
	if host, port, err := net.SplitHostPort(addr); err == nil {
		return host, port
	}
	return strings.Trim(addr, "[]"), ""
}

// REST calls the HTTP API with the token.
func REST(t Target) []Snippet {
	auth := `curl -fsS -H "Authorization: Bearer ` + t.Token + `" `
	return []Snippet{
		{ID: "rest-whoami", Title: "Who the token is", Lang: "sh", Text: auth + shellWord(t.URL+"/api/v1/whoami")},
		{ID: "rest-tasks", Title: "List tasks", Lang: "sh", Text: auth + `"` + t.URL + `/api/v1/tasks?limit=5"`},
	}
}

// quote renders s as a JSON string, which is also a valid TOML basic string
// for the values written here.
func quote(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

// shellSafe matches a word the shell reads as itself.
var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,~-]+$`)

// shellWord quotes s for a POSIX shell when it needs quoting.
func shellWord(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
