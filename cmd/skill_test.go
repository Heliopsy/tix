// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/testenv"
	"github.com/spf13/cobra"
)

// skillFile is the agent skill, which documents this command tree and records
// the commit it was checked against.
const skillFile = "../skills/tix/SKILL.md"

// verifiedAgainst matches the one version marker the skill is allowed to carry.
var verifiedAgainst = regexp.MustCompile(`(?m)^verified-against: tix ([0-9a-f]{7,40}) \([0-9]{4}-[0-9]{2}-[0-9]{2}\)$`)

// terminators end a command inside a line: what follows belongs to another
// process or to the shell.
var terminators = map[string]bool{"|": true, "||": true, "&&": true, ";": true, ">": true, ">>": true, "<": true, "#": true}

func readSkill(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("reading the skill: %v", err)
	}
	return string(b)
}

func TestSkillRecordsOneVersionMarker(t *testing.T) {
	skill := readSkill(t)

	if n := len(verifiedAgainst.FindAllString(skill, -1)); n != 1 {
		t.Fatalf("verified-against matched %d times, want exactly 1", n)
	}
	if n := strings.Count(skill, "verified-against:"); n != 1 {
		t.Errorf("verified-against: appears %d times, want 1; a second marker drifts", n)
	}
	if strings.Contains(skill, "written against tix") {
		t.Error("the body carries a second version marker; keep only the front matter one")
	}
	if strings.Contains(skill, "-dirty") {
		t.Error("the skill claims a dirty tree, which nobody can verify")
	}
}

func TestSkillVerifiedAgainstIsAnAncestorOfHead(t *testing.T) {
	m := verifiedAgainst.FindStringSubmatch(readSkill(t))
	if m == nil {
		t.Fatal("no verified-against marker")
	}
	if _, err := exec.LookPath("git"); err != nil {
		testenv.Skip(t, testenv.Capability{Name: "git", Why: "git is not on PATH", How: "install git"})
	}
	if out, err := exec.Command("git", "-C", "..", "rev-parse", "--git-dir").CombinedOutput(); err != nil {
		testenv.Skip(t, testenv.Capability{
			Name: "git-checkout",
			Why:  "the source is not a git checkout: " + strings.TrimSpace(string(out)),
			How:  "run the tests from a clone",
		})
	}
	// A shallow clone cannot answer this. CI checks out with depth 1 by
	// default, so the recorded commit is simply absent and merge-base reports
	// "not an ancestor" for a marker that is perfectly current. That made the
	// guard fail for the one reason it is not meant to catch. It announces the
	// gap rather than passing quietly, because a staleness check that silently
	// does nothing is the thing it exists to prevent.
	if out, err := exec.Command("git", "-C", "..", "rev-parse", "--is-shallow-repository").CombinedOutput(); err == nil &&
		strings.TrimSpace(string(out)) == "true" {
		testenv.Skip(t, testenv.Capability{
			Name: "git-full-history",
			Why:  "the checkout is shallow, so the recorded commit cannot be located",
			How:  "fetch the full history (actions/checkout with fetch-depth: 0)",
		})
	}
	out, err := exec.Command("git", "-C", "..", "merge-base", "--is-ancestor", m[1], "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("verified-against names %s, which is not an ancestor of HEAD: %v %s; "+
			"re-verify the skill against the current tree and update the marker",
			m[1], err, strings.TrimSpace(string(out)))
	}
}

func TestSkillNamesOnlyRealCommandsAndFlags(t *testing.T) {
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())

	lines := skillInvocations(readSkill(t))
	if len(lines) < 20 {
		t.Fatalf("found %d tix invocations in the skill, want the extractor to still work", len(lines))
	}
	for _, line := range lines {
		checkInvocation(t, root, line)
	}
}

// skillInvocations returns every line of the skill that runs tix: the prompt
// lines of console blocks, every line of sh blocks, and inline code spans that
// start with the binary. Console output is not a command, so it is skipped.
func skillInvocations(skill string) []string {
	var out []string
	var fence string

	lines := strings.Split(skill, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " ")
		for strings.HasSuffix(line, `\`) && i+1 < len(lines) {
			line = strings.TrimSuffix(line, `\`) + " " + strings.TrimRight(lines[i+1], " ")
			i++
		}
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			if fence == "" {
				fence = strings.TrimPrefix(trimmed, "```")
			} else {
				fence = ""
			}
			continue
		}

		switch fence {
		case "sh":
			out = append(out, line)
		case "console":
			if rest, ok := strings.CutPrefix(trimmed, "$ "); ok {
				out = append(out, rest)
			}
		case "":
			for _, span := range inlineCode(line) {
				if span == "tix" || strings.HasPrefix(span, "tix ") {
					out = append(out, span)
				}
			}
		}
	}
	return out
}

// inlineCode returns the contents of every backtick span on a line.
func inlineCode(line string) []string {
	parts := strings.Split(line, "`")
	var out []string
	for i := 1; i < len(parts); i += 2 {
		out = append(out, parts[i])
	}
	return out
}

// checkInvocation resolves every tix command on one line against the real
// command tree and asserts each flag it passes exists there.
func checkInvocation(t *testing.T, root *cobra.Command, line string) {
	t.Helper()

	words := strings.Fields(stripQuoted(line))
	for i, w := range words {
		if cleanWord(w) != "tix" {
			continue
		}
		cmd, flags, known := resolveInvocation(root, words[i+1:])
		if !known {
			continue
		}
		if cmd == nil {
			t.Errorf("skill names a command tix does not have: %s", line)
			continue
		}
		if cmd == root {
			continue
		}
		for _, flag := range flags {
			if !hasFlag(cmd, flag) {
				t.Errorf("%s has no flag %s: %s", cmd.CommandPath(), flag, line)
			}
		}
	}
}

// stripQuoted replaces every quoted span with a placeholder word. A quoted
// argument is opaque to the shell's own word splitting, so a filter expression
// such as '-tag:ops' is an argument and not a flag. Without this a leading dash
// inside quotes reaches pflag's shorthand lookup, which panics on a name longer
// than one character.
func stripQuoted(line string) string {
	var b strings.Builder
	var quote rune
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
			b.WriteString("ARG")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// cleanWord strips the shell and Markdown punctuation around a word, leaving
// the token a shell would see.
func cleanWord(w string) string {
	return strings.Trim(strings.TrimPrefix(w, "$("), "`'\"()[]$,.")
}

// resolveInvocation walks words down the command tree and reports the command
// reached with the flags passed to it. A nil command with known set means the
// first word names nothing in the tree; a placeholder such as <command> is not
// an invocation at all, and words after -- belong to a child process.
func resolveInvocation(root *cobra.Command, words []string) (*cobra.Command, []string, bool) {
	cmd := root
	var flags []string

	for i := 0; i < len(words); i++ {
		w := cleanWord(words[i])
		if w == "" {
			continue
		}
		if terminators[w] || w == "--" {
			break
		}
		if strings.HasPrefix(w, "<") {
			return nil, nil, false
		}
		if strings.HasPrefix(w, "-") && w != "-" {
			name, _, inline := strings.Cut(w, "=")
			flags = append(flags, name)
			if !inline && takesValue(cmd, name) {
				i++
			}
			continue
		}
		if next, _, err := cmd.Find([]string{w}); err == nil && next != cmd {
			cmd = next
			continue
		}
		if cmd == root {
			return nil, nil, true
		}
		break
	}
	return cmd, flags, true
}

// hasFlag reports whether a command defines a flag, by long name or shorthand,
// including the persistent flags it inherits.
func hasFlag(cmd *cobra.Command, name string) bool {
	if name == "--help" || name == "-h" {
		return true
	}
	if long, ok := strings.CutPrefix(name, "--"); ok {
		return cmd.Flags().Lookup(long) != nil || cmd.InheritedFlags().Lookup(long) != nil
	}
	short := strings.TrimPrefix(name, "-")
	if len(short) != 1 {
		return false
	}
	return cmd.Flags().ShorthandLookup(short) != nil || cmd.InheritedFlags().ShorthandLookup(short) != nil
}

// takesValue reports whether a flag consumes the word after it.
func takesValue(cmd *cobra.Command, name string) bool {
	if long, ok := strings.CutPrefix(name, "--"); ok {
		if f := cmd.Flags().Lookup(long); f != nil {
			return f.NoOptDefVal == ""
		}
		if f := cmd.InheritedFlags().Lookup(long); f != nil {
			return f.NoOptDefVal == ""
		}
		return false
	}
	short := strings.TrimPrefix(name, "-")
	if len(short) != 1 {
		return false
	}
	if f := cmd.Flags().ShorthandLookup(short); f != nil {
		return f.NoOptDefVal == ""
	}
	if f := cmd.InheritedFlags().ShorthandLookup(short); f != nil {
		return f.NoOptDefVal == ""
	}
	return false
}

// skillOmits names the "work" group commands the skill deliberately does not
// cover, each with the reason. An entry here is a decision; a command that is
// neither documented nor listed is the accident this guard exists to catch.
var skillOmits = map[string]string{
	"tag": "agents attach tags with task add -l and filter with task ls -l; tag add/rm/ls is tenant upkeep a person does",
}

// TestSkillCoversEveryWorkCommand fails when a command an agent is meant to
// drive lands without reaching the skill. The existing guards prove the skill
// tells no lies; this one is the only guard that notices an omission, which is
// how tix stats, tix actor and a rewritten task filter shipped unmentioned
// while every test stayed green.
func TestSkillCoversEveryWorkCommand(t *testing.T) {
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())
	skill := readSkill(t)

	var covered int
	for _, cmd := range root.Commands() {
		if cmd.GroupID != "work" || !cmd.IsAvailableCommand() {
			continue
		}
		name := cmd.Name()
		if why, omitted := skillOmits[name]; omitted {
			if why == "" {
				t.Errorf("skillOmits[%q] carries no reason", name)
			}
			if strings.Contains(skill, "tix "+name) {
				t.Errorf("skillOmits lists %q but the skill covers it; drop the entry", name)
			}
			continue
		}
		if !strings.Contains(skill, "tix "+name) {
			t.Errorf("the skill never names %q, a command in the work group; "+
				"document it, or add it to skillOmits with the reason", "tix "+name)
		}
		covered++
	}
	if covered < 5 {
		t.Fatalf("checked only %d work commands, want the group lookup to still work", covered)
	}
}
