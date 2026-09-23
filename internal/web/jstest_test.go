// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/testenv"
)

// jsSuite is the node test runner invocation the containerised gate also runs.
// It is a glob, not a directory: node resolves a directory argument as a
// module rather than as a set of test files.
var jsSuite = []string{"--test", "--test-reporter=tap", "jstest/*.test.mjs"}

// nodeCapability names the interpreter this wrapper needs, so a machine
// without it reports the gap instead of skipping into a green run.
var nodeCapability = testenv.Capability{
	Name: "node",
	Why:  "node is not on PATH",
	How:  "just jstest",
}

// TestJavaScriptAssets runs the suite covering assets/*.js. The authoritative
// gate is `just jstest`, which runs the same command inside the pinned CI
// image; this wrapper is so the plain `go test ./...` loop catches a
// regression too, on a machine that happens to have node.
func TestJavaScriptAssets(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		testenv.Skip(t, nodeCapability)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// #nosec G204 -- node is resolved from PATH and the arguments are constant.
	out, err := exec.CommandContext(ctx, node, jsSuite...).CombinedOutput()
	if err != nil {
		t.Fatalf("the JavaScript suite failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "# fail 0") {
		t.Fatalf("the JavaScript suite reported no clean run:\n%s", out)
	}
}

// TestDecideLoadsBeforeTheScriptsThatNeedIt guards the load order the assets
// depend on: each one stands down without window.tix, so a decide.js that
// arrives late would silently disable the shortcut layer, the live feed and
// the copy buttons at once.
func TestDecideLoadsBeforeTheScriptsThatNeedIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	page := b.page("/tasks")
	decide := strings.Index(page, `src="/assets/decide.js"`)
	if decide < 0 {
		t.Fatal("the layout does not load /assets/decide.js")
	}
	for _, script := range []string{"live.js", "shortcuts.js", "copy.js"} {
		at := strings.Index(page, `src="/assets/`+script+`"`)
		if at < 0 {
			t.Errorf("the layout does not load /assets/%s", script)
			continue
		}
		if at < decide {
			t.Errorf("/assets/%s loads before decide.js, so window.tix is not there yet", script)
		}
	}

	resp := f.as("alice").get("/assets/decide.js")
	wantStatus(t, resp, http.StatusOK)
	if !strings.Contains(body(t, resp), "isTypingTarget") {
		t.Error("the served decide.js is not the asset the suite covers")
	}
}
