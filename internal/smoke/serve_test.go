//go:build smoke

package smoke

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"syscall"
	"testing"
)

// TestServeAnswersAndShutsDownCleanly drives the server the way a deployment
// does: start it, check it is healthy, write through the API, read it back,
// then stop it and confirm it let go of the port.
func TestServeAnswersAndShutsDownCleanly(t *testing.T) {
	s := newScratch(t)
	db := s.path("serve.db")
	s.mustRun("--db", db, "task", "add", "already here")
	token := mintToken(t, s, db)

	addr := freePort(t)
	base := "http://" + addr
	server := s.start("--db", db, "serve", "--listen", addr, "--insecure-no-tls")
	waitForHTTP(t, base+"/healthz", server)

	if body, code := get(t, base+"/healthz", ""); code != http.StatusOK || !strings.Contains(body, `"ok"`) {
		t.Fatalf("GET /healthz = %d %s", code, body)
	}

	created := postJSON(t, base+"/api/v1/tasks", token, `{"title":"over the api"}`)
	var task struct {
		Ref   string `json:"ref"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(created), &task); err != nil {
		t.Fatalf("decoding the created task: %v\n%s", err, created)
	}
	if task.Title != "over the api" || task.Ref == "" {
		t.Fatalf("the api created %+v", task)
	}

	body, code := get(t, base+"/api/v1/tasks", token)
	if code != http.StatusOK || !strings.Contains(body, "over the api") {
		t.Fatalf("GET /api/v1/tasks = %d, the task did not come back:\n%s", code, body)
	}

	// A live connection is the only way connection ls has a row to render, and
	// the registry is one process's, so the question has to be put to the
	// server rather than to the database.
	watch := s.start("--server", base, "--token", token, "watch")
	t.Cleanup(func() { _ = watch.cmd.Process.Kill() })
	assertConnectionListing(t, s, base, token)

	server.signal(syscall.SIGINT)
	if got := server.waitExit(); got != 0 {
		t.Fatalf("serve exited %d\nlog:\n%s", got, server.log.String())
	}
	waitForPortFree(t, addr)
}

// assertConnectionListing waits for the stream to register and then holds the
// listing to the same standard as every other one.
func assertConnectionListing(t *testing.T, s *scratch, base, token string) {
	t.Helper()
	var out string
	for range 200 {
		out = s.mustRun("--server", base, "--token", token, "connection", "ls").out
		if strings.Contains(out, "events") {
			break
		}
		sleepPoll()
	}
	if !strings.Contains(out, "events") {
		t.Fatalf("no live connection was ever listed:\n%s", out)
	}
	assertRendered(t, "connection ls", out, "ID", "SURFACE", "ACTOR", "FROM", "SINCE", "KEY")
}

// mintToken creates a token that may do everything, which is what a smoke run
// needs and what no deployment should hand out.
func mintToken(t *testing.T, s *scratch, db string) string {
	t.Helper()
	out := s.mustRun("--db", db, "token", "create", "smoke", "--scope", "*", "-o", "json").out
	var minted struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(out), &minted); err != nil {
		t.Fatalf("decoding the minted token: %v\n%s", err, out)
	}
	if minted.Token == "" {
		t.Fatalf("no token in %s", out)
	}
	return minted.Token
}

func get(t *testing.T, url, token string) (string, int) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	return send(t, req, token)
}

func postJSON(t *testing.T, url, token, body string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	got, code := send(t, req, token)
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("POST %s = %d:\n%s", url, code, got)
	}
	return got
}

func send(t *testing.T, req *http.Request, token string) (string, int) {
	t.Helper()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the response: %v", err)
	}
	return string(body), resp.StatusCode
}
