// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/wire"
)

// tix serve answers other people, so a source it is sent must not name a
// server file; the same request from a command against the database directly
// is the operator's own and is stored. Only the running server can show that
// runServe marks its service as shared.
func TestServeRefusesASyncPathOutsideTheTenantDirectory(t *testing.T) {
	c := newCLI(t)
	outside := filepath.Join(t.TempDir(), "mapping.yaml")
	c.mustRun("sync", "source", "add", "generic", "direct", "--mapping", outside)

	var created struct {
		Token string `json:"token"`
	}
	out := c.mustRun("token", "create", "syncer", "--scope", "sync:admin", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("token create: %v (%s)", err, out)
	}
	addr := freePort(t)
	done := make(chan result, 1)
	go func() { done <- c.run("serve", "--listen", addr) }()
	waitFor(t, "http://"+addr+"/healthz")
	defer stopServe(t, done)

	body := `{"system":"generic","name":"served","mapping_path":` + strconvQuote(outside) + `}`
	req, err := http.NewRequest(http.MethodPut, "http://"+addr+wire.RouteSyncSources, strings.NewReader(body))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+created.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	reply, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(reply), "mapping_path") {
		t.Fatalf("tix serve answered %d %s, want the server path refused", resp.StatusCode, reply)
	}
}

func strconvQuote(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}
