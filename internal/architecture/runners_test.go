// SPDX-License-Identifier: AGPL-3.0-or-later

package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// selfHosted are the runner labels that mean our own hardware rather than a
// throwaway machine GitHub discards afterwards.
var selfHosted = []string{"ho-z10"}

// forkGuard is the condition a self-hosted job must carry. It is compared
// literally: a job that expresses the same idea differently still has to be
// read by a person, and a guard nobody can recognise at a glance is a guard
// that gets copied wrong.
const forkGuard = "github.event_name != 'pull_request' || " +
	"github.event.pull_request.head.repo.full_name == github.repository"

type workflow struct {
	On   map[string]any `yaml:"on"`
	Jobs map[string]struct {
		RunsOn any    `yaml:"runs-on"`
		If     string `yaml:"if"`
		Steps  []struct {
			Uses string `yaml:"uses"`
			Run  string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

// TestNoForkPullRequestRunsOnOurHardware is the one that matters.
//
// This repository is public and its CI runs on a self-hosted runner. Without a
// guard, a pull request from a fork builds and tests a stranger's code on our
// own machine. GitHub's fork-approval setting is the other half of this, but
// that is a person clicking a button, and it should not be the only thing in
// the way.
//
// The check reads the workflows rather than taking a list of jobs, so the
// seventh job somebody adds next month is covered without anybody remembering
// this file exists. That is the whole point: the guard was added when there
// were six jobs, and six is exactly the number of things a person can add a
// seventh to without noticing.
func TestNoForkPullRequestRunsOnOurHardware(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", ".github", "workflows", "*.y*ml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no workflows found: %v", err)
	}

	checked := 0
	for _, path := range paths {
		name := filepath.Base(path)

		raw, err := os.ReadFile(path) //nolint:gosec // a fixed path inside the repository
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var wf workflow
		if err := yaml.Unmarshal(raw, &wf); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		// A workflow no pull request can start cannot be reached by a fork.
		if !triggeredByPullRequest(wf.On) {
			continue
		}

		for job, spec := range wf.Jobs {
			if !runsSelfHosted(spec.RunsOn) {
				continue
			}
			// A job that never puts the pull request's code on the machine
			// cannot run it either. The CLA check and the title check are
			// both of this kind: they read what GitHub tells them about the
			// pull request and touch nothing from the branch. They have to
			// run for forks, since refusing to check an outside
			// contributor's CLA defeats the point of having one.
			//
			// This is checked rather than declared, so a checkout added to
			// one of them later stops being exempt on its own.
			if !fetchesCode(spec.Steps) {
				continue
			}
			checked++
			if normalise(spec.If) != normalise(forkGuard) {
				t.Errorf("%s: job %q runs on our own hardware, a pull request can start it, and it "+
					"checks that pull request's code out, but it does not carry the fork guard.\n"+
					"  want if: %s\n"+
					"  got  if: %s\n"+
					"Either add that condition, or move the job to ubuntu-latest the way fork-pr, "+
					"codeql and dependency-review do.",
					name, job, forkGuard, orNone(spec.If))
			}
		}
	}

	// If the scan stops finding jobs it is not proving anything any more.
	if checked == 0 {
		t.Fatal("no self-hosted job that checks code out and is reachable from a pull request was " +
			"found. That is the state we want, but it also means this check no longer proves anything, " +
			"so make it fail loudly rather than pass quietly on nothing.")
	}
}

// fetchesCode reports whether any step brings the branch onto the runner.
func fetchesCode(steps []struct {
	Uses string `yaml:"uses"`
	Run  string `yaml:"run"`
}) bool {
	for _, st := range steps {
		if strings.HasPrefix(st.Uses, "actions/checkout") {
			return true
		}
		if strings.Contains(st.Run, "git clone") || strings.Contains(st.Run, "git fetch") {
			return true
		}
	}
	return false
}

func triggeredByPullRequest(on map[string]any) bool {
	for k := range on {
		if strings.HasPrefix(k, "pull_request") {
			return true
		}
	}
	return false
}

func runsSelfHosted(v any) bool {
	switch t := v.(type) {
	case string:
		for _, label := range selfHosted {
			if strings.Contains(t, label) {
				return true
			}
		}
	case []any:
		for _, e := range t {
			if runsSelfHosted(e) {
				return true
			}
		}
	}
	return false
}

// normalise flattens the whitespace a multi-line YAML scalar leaves behind, so
// the comparison is about the condition and not about how it was wrapped.
func normalise(s string) string { return strings.Join(strings.Fields(s), " ") }

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(none)"
	}
	return s
}
