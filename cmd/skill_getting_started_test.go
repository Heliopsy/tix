// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// gettingStartedDir is the skill for a person new to tix. It is published
// beside the main skill and guarded the same way: every command it names is
// checked against the real tree, and it carries one dated version marker.
const (
	gettingStartedDir  = "../skills/tix-getting-started"
	gettingStartedFile = gettingStartedDir + "/SKILL.md"
	skillsRoot         = "../skills"
)

// frontMatter matches the block between the opening and closing --- lines.
var frontMatter = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n`)

// frontMatterField reads one top-level key out of a front matter block.
func frontMatterField(block, key string) string {
	for _, line := range strings.Split(block, "\n") {
		if v, ok := strings.CutPrefix(line, key+":"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// TestEverySkillHasFrontMatter fails when a skill under skills/ is missing the
// fields an agent picks it by, or the version the site's mirror reads.
func TestEverySkillHasFrontMatter(t *testing.T) {
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		t.Fatal(err)
	}
	var checked int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(skillsRoot, e.Name(), "SKILL.md")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("skills/%s holds no SKILL.md: %v", e.Name(), err)
			continue
		}
		checked++
		m := frontMatter.FindStringSubmatch(string(b))
		if m == nil {
			t.Errorf("%s does not open with a --- front matter block", path)
			continue
		}
		if name := frontMatterField(m[1], "name"); name != e.Name() {
			t.Errorf("%s names itself %q, want its directory name %q", path, name, e.Name())
		}
		if frontMatterField(m[1], "description") == "" {
			t.Errorf("%s has no description, which is what an agent chooses a skill by", path)
		}
		if v, err := strconv.Atoi(frontMatterField(m[1], "version")); err != nil || v < 1 {
			t.Errorf("%s has version %q, want a positive integer", path, frontMatterField(m[1], "version"))
		}
		if !verifiedAgainst.MatchString(m[1]) {
			t.Errorf("%s carries no verified-against marker in its front matter", path)
		}
	}
	if checked < 2 {
		t.Fatalf("checked %d skills, want both the main and the getting-started skill", checked)
	}
}

func TestGettingStartedSkillRecordsOneVersionMarker(t *testing.T) {
	sources := skillSourcesIn(t, gettingStartedDir)
	var all strings.Builder
	for _, body := range sources {
		all.WriteString(body)
	}
	if n := strings.Count(all.String(), "verified-against:"); n != 1 {
		t.Errorf("verified-against: appears %d times across the skill, want 1", n)
	}
	if strings.Contains(all.String(), "-dirty") {
		t.Error("the skill claims a dirty tree, which nobody can verify")
	}
}

func TestGettingStartedSkillVerifiedAgainstIsAnAncestorOfHead(t *testing.T) {
	assertVerifiedAgainstIsAnAncestor(t, skillSourcesIn(t, gettingStartedDir)["SKILL.md"])
}

func TestGettingStartedSkillNamesOnlyRealCommandsAndFlags(t *testing.T) {
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())

	var total int
	for name, body := range skillSourcesIn(t, gettingStartedDir) {
		lines := skillInvocations(body)
		total += len(lines)
		for _, line := range lines {
			checkInvocation(t, root, "tix-getting-started/"+name, line)
		}
	}
	if total < 20 {
		t.Fatalf("found %d tix invocations in the getting-started skill, want the extractor to still work", total)
	}
}

// TestGettingStartedSkillLinksOnlyFilesItShips fails on a relative link the
// skill cannot follow once installed or mirrored alone. A link out of the skill
// directory resolves in the repository and nowhere else.
func TestGettingStartedSkillLinksOnlyFilesItShips(t *testing.T) {
	sources := skillSourcesIn(t, gettingStartedDir)
	for name, body := range sources {
		for _, m := range skillLink.FindAllStringSubmatch(body, -1) {
			target := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(name), m[1])))
			if _, ok := sources[target]; !ok {
				t.Errorf("%s links %s, which the getting-started skill does not ship; "+
					"link an absolute URL instead", name, m[1])
			}
		}
	}
	for name := range sources {
		if name != "SKILL.md" && !strings.Contains(sources["SKILL.md"], "]("+name) {
			t.Errorf("the skill ships %s but SKILL.md links to no such file", name)
		}
	}
}

// TestGettingStartedSkillAsksFirstAndHandsOver pins the parts of the skill a
// reword could drop without any command going stale: it asks the person how
// they want to use tix before anything else, keeps the safety rules, and hands
// over to the main skill.
func TestGettingStartedSkillAsksFirstAndHandsOver(t *testing.T) {
	body := skillSourcesIn(t, gettingStartedDir)["SKILL.md"]

	ask := strings.Index(body, "## Step 1: ask how they want to use tix")
	concepts := strings.Index(body, "## Step 2: explain the concepts")
	setup := strings.Index(body, "## Step 3: set up the interface they chose")
	if ask < 0 || concepts < 0 || setup < 0 || !(ask < concepts && concepts < setup) {
		t.Errorf("the steps are missing or out of order: ask %d, concepts %d, setup %d", ask, concepts, setup)
	}
	for _, want := range []string{
		"Never touch a real store without asking",
		"Never print secrets",
		"--db",
		"https://tix.red/skill.md",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the getting-started skill no longer says %q", want)
		}
	}
	if !strings.Contains(body, "\nThe commit this file was checked against is") {
		t.Error("the paragraph the site mirror replaces is gone; tix.red's sync matches its first sentence")
	}
}

// TestNewUsersArePointedAtTheGettingStartedSkill fails when the README or the
// docs index stops sending a newcomer, or the agent helping one, to the skill.
func TestNewUsersArePointedAtTheGettingStartedSkill(t *testing.T) {
	if _, err := os.Stat(gettingStartedFile); err != nil {
		t.Fatal(err)
	}
	for file, link := range map[string]string{
		"../README.md":      "skills/tix-getting-started/SKILL.md",
		"../docs/README.md": "../skills/tix-getting-started/SKILL.md",
	} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		at := strings.Index(body, "New to tix, or helping someone who is?")
		if at < 0 {
			t.Errorf("%s has no pointer to the getting-started skill", file)
			continue
		}
		if !strings.Contains(body[at:], "]("+link+")") {
			t.Errorf("%s points new users at the skill without linking %s", file, link)
		}
	}
}
