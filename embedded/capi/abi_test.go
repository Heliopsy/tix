// SPDX-License-Identifier: AGPL-3.0-or-later

package capi_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/testenv"
	"github.com/heliopsy/tix/internal/wire"
)

// cToolchain is what driving the library from C needs beyond Go.
func cToolchain(why string) testenv.Capability {
	return testenv.Capability{
		Name: "c-toolchain",
		Why:  why + ", so the C ABI was not driven from C",
		How:  "install gcc or clang and run go test ./embedded/capi/",
	}
}

// buildHarness builds the shared library and the C program that calls it,
// returning the program's path and the directory holding the library.
func buildHarness(t *testing.T) (string, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("building the shared library is slow")
	}
	if runtime.GOOS == "windows" {
		testenv.Skip(t, cToolchain("the harness links with -rpath, which Windows has no use for"))
	}
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	if _, err := exec.LookPath(cc); err != nil {
		testenv.Skip(t, cToolchain("no C compiler named "+cc))
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		testenv.Skip(t, cToolchain("no go tool on PATH"))
	}
	dir := t.TempDir()
	lib := "libtix.so"
	if runtime.GOOS == "darwin" {
		lib = "libtix.dylib"
	}
	build := exec.Command(goTool, "build", "-buildmode=c-shared", "-o", filepath.Join(dir, lib),
		"github.com/heliopsy/tix/embedded/libtix")
	build.Env = append(os.Environ(), "CGO_ENABLED=1")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the shared library: %v\n%s", err, out)
	}
	harness := filepath.Join(dir, "harness")
	compile := exec.Command(cc, "-o", harness, filepath.Join("testdata", "harness.c"),
		"-I", dir, "-L", dir, "-ltix", "-lpthread", "-Wl,-rpath,"+dir)
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compiling the harness: %v\n%s", err, out)
	}
	return harness, dir
}

// errorCode is the code of an error object, or "" for any other answer.
func errorCode(t *testing.T, answer string) string {
	t.Helper()
	var body struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(answer), &body); err != nil {
		t.Fatalf("answer %q is not json: %v", answer, err)
	}
	if body.Error == nil {
		return ""
	}
	if body.Error.Message == "" {
		t.Errorf("error object %s has no message", answer)
	}
	return body.Error.Code
}

func TestTheCABIFromC(t *testing.T) {
	harness, dir := buildHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, harness, filepath.Join(dir, "first.db"), filepath.Join(dir, "second.db"))
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+dir, "DYLD_LIBRARY_PATH="+dir)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	answers := map[string]string{}
	lines := bufio.NewScanner(stdout)
	read := func(until string) {
		for lines.Scan() {
			label, answer, _ := strings.Cut(lines.Text(), " ")
			answers[label] = answer
			if label == until {
				return
			}
		}
		t.Fatalf("the harness ended before %q: %v (answers so far: %v)", until, lines.Err(), answers)
	}

	read("concurrent-bad")
	var start struct {
		Handle int64  `json:"handle"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal([]byte(answers["start"]), &start); err != nil || start.Handle <= 0 ||
		!strings.HasPrefix(start.URL, "http://127.0.0.1:") {
		t.Fatalf("start answered %s", answers["start"])
	}
	var token struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(answers["token"]), &token); err != nil || !strings.HasPrefix(token.Token, "tix_") {
		t.Fatalf("token answered %s", answers["token"])
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, start.URL+wire.RouteWhoAmI, nil)
	req.Header.Set("Authorization", "Bearer "+token.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("whoami on the instance the C program started: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"handle":"app"`) {
		t.Fatalf("whoami answered %d %s", resp.StatusCode, body)
	}

	if _, err := io.WriteString(stdin, "stop\n"); err != nil {
		t.Fatal(err)
	}
	read("stop-again")
	if err := cmd.Wait(); err != nil {
		t.Fatalf("the harness failed: %v", err)
	}

	for label, want := range map[string]string{
		"bad-json": "invalid", "public": "invalid", "bad-ttl": "invalid",
		"unknown": "not_found", "stop-again": "not_found",
		"start": "", "create": "", "tenants": "", "token": "", "version": "",
	} {
		if got := errorCode(t, answers[label]); got != want {
			t.Errorf("%s answered %s, want error code %q", label, answers[label], want)
		}
	}
	if answers["stop"] != "{}" {
		t.Errorf("stop answered %s, want {}", answers["stop"])
	}
	if answers["create"] != `{"key":"work","name":"Work"}` {
		t.Errorf("create answered %s", answers["create"])
	}
	if answers["tenants"] != `{"tenants":[{"key":"default","name":"Default"},{"key":"work","name":"Work"}]}` {
		t.Errorf("tenants answered %s", answers["tenants"])
	}
	if answers["concurrent-bad"] != "0" {
		t.Errorf("%s concurrent calls answered badly", answers["concurrent-bad"])
	}
	if !strings.Contains(answers["version"], `"version":`) || !strings.Contains(answers["version"], `"commit":`) {
		t.Errorf("version answered %s", answers["version"])
	}
}
