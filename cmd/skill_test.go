// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sshd"
	"github.com/heliopsy/tix/internal/testenv"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

// ------------------------------------------------------------------ prose
//
// Everything above bounds the skill's *invocations*: each `tix …` line names a
// real command and passes real flags. None of it reads a sentence, which is how
// a breaking change to the SSH authentication model passed through this file
// untouched. Every command the SSH section named stayed real while what it said
// about them became the opposite of the truth: "any public key is accepted" long
// after the open sandbox had moved behind --demo.
//
// No test can decide whether prose is true. The four below assert the parts of
// it that are mechanically decidable, which is where that drift was visible:
// a transcript the skill quotes must be one the binary can print, a default it
// states must be the flag's real default, a mode switch must be mentioned at
// all, and the username it tells an agent to type must be the constant the
// listener compares against.

// skillFailureLine matches a quoted failure: the CLI's own `error: kind:
// message` or the SSH listener's `tix: message`, which carries no kind because
// a stranger is owed the reason and not the shape behind it.
var skillFailureLine = regexp.MustCompile(`^(?:error: ([a-z_]+): (.+)|tix: (.+))$`)

// sourceVerb matches a formatting verb inside a message, which stands for a
// value the message did not contain until it was printed.
var sourceVerb = regexp.MustCompile(`%[a-z]`)

// skipDirs are the trees that hold no message this repository prints.
var skipDirs = map[string]bool{
	".git": true, ".github": true, "bin": true, "dist": true,
	"node_modules": true, "openspec": true, "screenshots": true, "LICENSES": true,
}

// TestSkillQuotedFailuresAreOnesTheBinaryCanPrint ties every failure the skill
// quotes to a message that exists in the source.
//
// The skill is published and followed literally, so a transcript it shows is
// read as what the reader will see. A quoted failure that no message can
// produce is either a wording that drifted or a case that no longer exists, and
// the second is the dangerous one: the skill showed `tix ssh` refusing the
// zero-configuration store in a mode that does not refuse it.
func TestSkillQuotedFailuresAreOnesTheBinaryCanPrint(t *testing.T) {
	messages := sourceMessages(t)
	if len(messages) < 200 {
		t.Fatalf("collected %d messages from the source, want the collector to still work", len(messages))
	}

	kinds := map[string]bool{}
	for _, k := range core.Kinds {
		kinds[string(k)] = true
	}

	quoted := skillQuotedFailures(readSkill(t))
	if len(quoted) < 4 {
		t.Fatalf("found %d quoted failures in the skill, want the extractor to still work", len(quoted))
	}
	for _, line := range quoted {
		m := skillFailureLine.FindStringSubmatch(line)
		text := m[2] + m[3]
		if kind := m[1]; kind != "" && !kinds[kind] {
			t.Errorf("the skill quotes error kind %q, which core.Kinds does not list: %s", kind, line)
		}
		if !producible(messages, text) {
			t.Errorf("the skill quotes a failure no message in the source can produce:\n  %s\n"+
				"requote it from a real run, or drop it if the case no longer exists", line)
		}
	}
}

// skillQuotedFailures returns every failure line inside a console block. A
// prompt line is a command, not output, so it is skipped; so is a line in any
// other fence, where a `tix:` prefix would be prose about a target.
func skillQuotedFailures(skill string) []string {
	var out []string
	var fence string
	for _, raw := range strings.Split(skill, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			if fence == "" {
				fence = strings.TrimPrefix(line, "```")
			} else {
				fence = ""
			}
			continue
		}
		if fence != "console" || strings.HasPrefix(line, "$ ") {
			continue
		}
		if skillFailureLine.MatchString(line) {
			out = append(out, line)
		}
	}
	return out
}

// producible reports whether any message in the source renders to text.
func producible(messages []*regexp.Regexp, text string) bool {
	for _, m := range messages {
		if m.MatchString(text) {
			return true
		}
	}
	return false
}

// sourceMessages compiles every string the source could print as a message
// into a pattern that matches what it renders to, with each formatting verb
// standing for the value it carries.
//
// Concatenation is folded, because a message too long for one line is written
// as several literals and is one sentence by the time anybody reads it.
func sourceMessages(t *testing.T) []*regexp.Regexp {
	t.Helper()

	var out []*regexp.Regexp
	seen := map[string]bool{}
	add := func(s string) {
		if len(s) < 12 || !strings.Contains(s, " ") || seen[s] {
			return
		}
		seen[s] = true
		pattern := regexp.QuoteMeta(s)
		pattern = strings.ReplaceAll(pattern, "%q", `"[^"]*"`)
		pattern = sourceVerb.ReplaceAllString(pattern, ".*")
		re, err := regexp.Compile("^" + pattern + "$")
		if err != nil {
			return
		}
		out = append(out, re)
	}

	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		// Test files are excluded: a message asserted in a test and printed
		// nowhere is not a message the binary can print.
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return nil //nolint:nilerr // a file that does not parse is the compiler's complaint, not this guard's
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch e := n.(type) {
			case *ast.BasicLit:
				if e.Kind == token.STRING {
					if s, ok := unquote(e); ok {
						add(s)
					}
				}
			case *ast.BinaryExpr:
				if s, ok := foldString(e); ok {
					add(s)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking the source: %v", err)
	}
	return out
}

// unquote reads a string literal's value, whatever quoting it used.
func unquote(lit *ast.BasicLit) (string, bool) {
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// foldString joins an expression built only from string literals, which is how
// a message longer than one line is written.
func foldString(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		return unquote(v)
	case *ast.ParenExpr:
		return foldString(v.X)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		left, ok := foldString(v.X)
		if !ok {
			return "", false
		}
		right, ok := foldString(v.Y)
		if !ok {
			return "", false
		}
		return left + right, true
	}
	return "", false
}

// skillStatedDefault matches a flag the skill names with its default in
// parentheses straight after it: `--max-sessions` (100), `--tenant-ttl`
// (default 6h). The value has to start with a digit, which is what separates a
// stated default from a parenthetical about the flag.
var skillStatedDefault = regexp.MustCompile("`(--[a-z0-9-]+)` \\((?:default )?([0-9][^)]*)\\)")

// TestSkillStatedDefaultsAreTheRealDefaults ties every default the skill states
// to the value the flag actually carries.
//
// A default is the behaviour a reader gets by typing nothing, so a stated one
// is a claim about what happens without them. The skill's SSH limits were
// written from a listener's flags and nothing rechecked them afterwards.
func TestSkillStatedDefaultsAreTheRealDefaults(t *testing.T) {
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())
	defaults := flagDefaults(root)

	stated := skillStatedDefault.FindAllStringSubmatch(readSkill(t), -1)
	if len(stated) < 8 {
		t.Fatalf("found %d stated defaults in the skill, want the extractor to still work", len(stated))
	}
	for _, m := range stated {
		name, want := m[1], strings.TrimSpace(m[2])
		real, ok := defaults[strings.TrimPrefix(name, "--")]
		if !ok {
			t.Errorf("the skill states a default for %s, which no command registers", name)
			continue
		}
		if anyDefaultEquals(real, want) {
			continue
		}
		// A flag registered at its zero value declares no default of its own:
		// the value is a sentinel and the real default is applied downstream,
		// from a named constant. --top and --oldest are both of that shape, so
		// the claim is checked against the constant rather than called wrong.
		if zeroDefault(real) && declaredDefault(defaultConstants(t), name, want) {
			continue
		}
		t.Errorf("the skill says %s defaults to %q; the flag defaults to %v and no Default-prefixed "+
			"constant in internal/core names that value for it", name, want, real)
	}
}

// flagDefaults maps every flag name in the tree to the default values it
// carries, which is a set because one name may appear on several commands.
func flagDefaults(root *cobra.Command) map[string][]string {
	out := map[string][]string{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.Flags().VisitAll(func(f *pflag.Flag) {
			for _, seen := range out[f.Name] {
				if seen == f.DefValue {
					return
				}
			}
			out[f.Name] = append(out[f.Name], f.DefValue)
		})
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	return out
}

// anyDefaultEquals reports whether a stated default is one of the real ones.
// A duration is compared as a duration, because pflag prints 30m0s where a
// person writes 30m and both are the same deadline.
func anyDefaultEquals(real []string, stated string) bool {
	want, wantErr := time.ParseDuration(stated)
	for _, got := range real {
		if got == stated {
			return true
		}
		if wantErr == nil {
			if d, err := time.ParseDuration(got); err == nil && d == want {
				return true
			}
		}
	}
	return false
}

// zeroDefault reports whether every default a flag name carries is the zero
// value of its type, which is pflag's way of saying nothing was declared.
func zeroDefault(real []string) bool {
	for _, got := range real {
		switch got {
		case "", "0", "0s", "false":
		default:
			return false
		}
	}
	return len(real) > 0
}

// declaredDefault reports whether a Default-prefixed constant both carries the stated
// value and names the flag, so --top is answered by DefaultStatsTopActors and
// not by any other constant that happens to be 5.
func declaredDefault(constants map[string]string, flag, stated string) bool {
	want := strings.ReplaceAll(strings.TrimPrefix(flag, "--"), "-", "")
	for name, value := range constants {
		if value == stated && strings.Contains(strings.ToLower(name), want) {
			return true
		}
	}
	return false
}

// defaultConstants reads the literal Default-prefixed constants out of internal/core,
// which is where a default applied below the flag layer is written down.
func defaultConstants(t *testing.T) map[string]string {
	t.Helper()

	out := map[string]string{}
	entries, err := os.ReadDir(filepath.Join("..", "internal", "core"))
	if err != nil {
		t.Fatalf("reading internal/core: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join("..", "internal", "core", e.Name())
		file, perr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if perr != nil {
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, ident := range value.Names {
					if !strings.HasPrefix(ident.Name, "Default") || i >= len(value.Values) {
						continue
					}
					if lit, ok := value.Values[i].(*ast.BasicLit); ok && lit.Kind == token.INT {
						out[ident.Name] = lit.Value
					}
				}
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("internal/core declares no literal Default-prefixed constants; the reader changed shape")
	}
	return out
}

// skillFlagOmits names the boolean flags the skill deliberately leaves out, on
// commands it does name. An entry here is a decision; a switch that is neither
// mentioned nor listed is the accident the guard below exists to catch.
var skillFlagOmits = map[string]string{
	"tix config show --sources":         "the skill sends a reader to config show -v, which is the whole resolution; --sources is a narrower view of the same thing",
	"tix project ls --include-archived": "the skill uses project ls only to find project keys, and an archived project is not one an agent should claim into",
	"tix task ls --claimed":             "the skill drives the empty side with --unclaimed and the held side through --filter 'claimed:', which is the form it teaches",
	"tix task ls --include-deleted":     "a deleted task is not work; the skill names the deleted: filter term instead",
	"tix serve --insecure-no-tls":       "tix serve is an operator command the skill names only to say where the sweeper and the event stream run; deployment.md owns its flags",
	"tix serve --no-lease-sweeper":      "as above: the skill says the sweeper runs inside tix serve, not how to turn it off",
	"tix serve --no-retention-pruner":   "as above: operator tuning, not agent behaviour",
	"tix serve --no-webhook-dispatcher": "as above: operator tuning, not agent behaviour",
}

// TestSkillNamesEveryModeSwitch fails when a command the skill documents grows
// a boolean flag the skill never mentions.
//
// A boolean flag is a mode switch, and prose that does not mention one is
// asserting its default without saying so. That is exactly what happened here:
// --demo arrived, the open sandbox stopped being the default, and the SSH
// section went on describing the sandbox as what any key gets, because nothing
// required the switch to be named. The scope is boolean flags alone, on
// commands the skill already names, which keeps the omission list a set of
// decisions rather than an inventory.
func TestSkillNamesEveryModeSwitch(t *testing.T) {
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())
	skill := readSkill(t)

	var checked int
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		path := c.CommandPath()
		if path != "tix" && strings.Contains(skill, path) {
			c.Flags().VisitAll(func(f *pflag.Flag) {
				if f.Value.Type() != "bool" {
					return
				}
				checked++
				key := path + " --" + f.Name
				if why, omitted := skillFlagOmits[key]; omitted {
					if why == "" {
						t.Errorf("skillFlagOmits[%q] carries no reason", key)
					}
					return
				}
				if !strings.Contains(skill, "--"+f.Name) {
					t.Errorf("the skill documents %s but never names its switch --%s, "+
						"so it asserts that flag's default in silence; "+
						"document it, or add %q to skillFlagOmits with the reason",
						path, f.Name, key)
				}
			})
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	if checked < 15 {
		t.Fatalf("checked only %d boolean flags, want the walk to still work", checked)
	}
	for key := range skillFlagOmits {
		name := key[strings.LastIndex(key, " ")+1:]
		if strings.Contains(skill, name) {
			t.Errorf("skillFlagOmits lists %q but the skill names %s; drop the entry", key, name)
		}
	}
}

// TestSkillTeachesTheRealSSHUsername ties the username in the skill's ssh
// examples to the constant the listener compares against.
//
// The username stopped being decoration and became the tenant selector, and the
// skill went on telling agents to connect as the sandbox's actor handle, which
// selects a tenant of that name and is refused. It asserts the neutral user is
// shown, not that nothing else is: a second example naming a tenant key is
// correct and this cannot tell it from a stale one.
func TestSkillTeachesTheRealSSHUsername(t *testing.T) {
	if skill := readSkill(t); !strings.Contains(skill, "ssh -p 2222 "+sshd.NeutralUser+"@") {
		t.Errorf("the skill shows no `ssh -p 2222 %s@host` example; %q is the username meaning "+
			"\"the one tenant this key is enrolled in\", and any other name selects a tenant by that key",
			sshd.NeutralUser, sshd.NeutralUser)
	}
}
