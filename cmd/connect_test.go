// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/onboard"
)

// connectTarget fills every section, SSH included, so none is skipped.
var connectTarget = onboard.Target{URL: "https://tix.example.com", Tenant: "acme",
	Token: "TOKEN", Version: "0.41.0", SSH: "tix.example.com:2020"}

// connectSnippets is every snippet the Connect an agent screen can show.
func connectSnippets() []onboard.Snippet {
	t := connectTarget
	out := append([]onboard.Snippet{}, onboard.Install(t)...)
	out = append(out, onboard.Snippet{ID: "token", Lang: "sh", Text: onboard.TokenCommand()})
	out = append(out, onboard.CLI(t)...)
	out = append(out, onboard.MCPPreview())
	for _, c := range onboard.MCPClients(t) {
		out = append(out, c.Snippets...)
	}
	out = append(out, onboard.Skill()...)
	out = append(out, onboard.SSH(t)...)
	return append(out, onboard.REST(t)...)
}

// imageRef is the container image a snippet runs, which stands for tix.
var imageRef = regexp.MustCompile(`ghcr\.io/heliopsy/tix:\S+`)

// tixInvocations returns every tix command a snippet runs: a shell segment
// starting with tix, the command after -- that a client launches, and the
// command and arguments of a JSON or TOML server entry.
func tixInvocations(t *testing.T, s onboard.Snippet) []string {
	t.Helper()
	switch s.Lang {
	case "json":
		var doc any
		if err := json.Unmarshal([]byte(s.Text), &doc); err != nil {
			t.Fatalf("%s: %v", s.ID, err)
		}
		return jsonCommands(doc)
	case "toml":
		cmd := regexp.MustCompile(`(?m)^command = "(\S+)"$`).FindStringSubmatch(s.Text)
		args := regexp.MustCompile(`(?m)^args = \[(.*)\]$`).FindStringSubmatch(s.Text)
		if cmd == nil || args == nil {
			t.Fatalf("%s: no command and args in the TOML", s.ID)
		}
		return []string{cmd[1] + " " + strings.NewReplacer(`"`, "", ",", "").Replace(args[1])}
	}
	var out []string
	text := imageRef.ReplaceAllString(s.Text, "tix")
	for _, line := range strings.Split(text, "\n") {
		if at := strings.Index(line, "'{"); at >= 0 {
			var doc any
			if err := json.Unmarshal([]byte(line[at+1:strings.LastIndex(line, "}")+1]), &doc); err == nil {
				out = append(out, jsonCommands(doc)...)
			}
			continue
		}
		for _, seg := range regexp.MustCompile(`\|\||&&|[|;]`).Split(line, -1) {
			words := strings.Fields(seg)
			for i, w := range words {
				if w == "tix" && (i == 0 || words[i-1] == "--" || strings.HasPrefix(words[0], "podman")) {
					out = append(out, strings.Join(words[i:], " "))
					break
				}
			}
		}
	}
	return out
}

// jsonCommands finds every server entry in a client's JSON and returns its
// command line.
func jsonCommands(v any) []string {
	obj, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	var out []string
	switch cmd := obj["command"].(type) {
	case string:
		line := cmd
		for _, a := range asList(obj["args"]) {
			line += " " + a
		}
		out = append(out, line)
	case []any:
		out = append(out, strings.Join(asList(cmd), " "))
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, jsonCommands(obj[k])...)
	}
	return out
}

func asList(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// TestConnectScreenNamesOnlyRealCommandsAndFlags fails when a tix command the
// Connect an agent screen hands out is not in the command tree, or passes a
// flag its command does not have. The screen is copied and pasted, so a
// command that rotted would fail on somebody else's machine.
func TestConnectScreenNamesOnlyRealCommandsAndFlags(t *testing.T) {
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())

	var total int
	for _, s := range connectSnippets() {
		for _, line := range tixInvocations(t, s) {
			total++
			if !strings.HasPrefix(line, "tix ") {
				t.Errorf("%s runs %q, which is not tix", s.ID, line)
				continue
			}
			words := strings.Fields(line)
			cmd, _, _, known := resolveInvocation(root, words[1:])
			if !known || cmd == nil || cmd == root {
				t.Errorf("%s names a command tix does not have: %s", s.ID, line)
				continue
			}
			checkInvocation(t, root, s.ID, line)
		}
	}
	if total < 17 {
		t.Fatalf("found %d tix invocations on the connect screen, want the extractor to still work", total)
	}
}

// TestConnectSkillFetchesEveryReferenceFile fails when the skill gains or
// loses a reference file the install commands do not fetch. SKILL.md links
// them relatively, so a missing one is a dead link in the agent's copy.
func TestConnectSkillFetchesEveryReferenceFile(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(skillDir, "reference", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	var have []string
	for _, f := range files {
		have = append(have, strings.TrimSuffix(filepath.Base(f), ".md"))
	}
	want := slices.Clone(onboard.SkillReferences)
	sort.Strings(have)
	sort.Strings(want)
	if !slices.Equal(have, want) {
		t.Fatalf("skills/tix/reference holds %v, the connect screen fetches %v", have, want)
	}
	if _, err := os.Stat(skillFile); err != nil {
		t.Fatal(err)
	}
}
