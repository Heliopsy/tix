// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// statusDB is a disposable database of this test's own, never the zero-config
// store, which is somebody's real work.
func statusDB(c *cli) string { return filepath.Join(c.home, "status.db") }

// TestStatusAgainstADatabaseWithNoServer is the ordinary single-user case: no
// server has ever run, and saying so is the right answer rather than a failure.
func TestStatusAgainstADatabaseWithNoServer(t *testing.T) {
	c := newCLI(t)
	db := statusDB(c)

	got := c.mustRun("--db", db, "status")
	for _, want := range []string{"INSTALLATION", "SERVERS (0 attached)", "WORK", "none registered"} {
		if !strings.Contains(got.out, want) {
			t.Errorf("the report never says %q:\n%s", want, got.out)
		}
	}
	if strings.Contains(got.out, "NOT HEARTBEATING") {
		t.Errorf("a report with no servers claims one stopped answering:\n%s", got.out)
	}
}

// TestStatusJSONCarriesTheContract holds the shape a consumer depends on. The
// fields are read out of the document rather than off a Go struct, because the
// document is what a script sees.
func TestStatusJSONCarriesTheContract(t *testing.T) {
	c := newCLI(t)
	db := statusDB(c)

	got := c.mustRun("--db", db, "status", "-o", "json")
	var doc map[string]any
	if err := json.Unmarshal([]byte(got.out), &doc); err != nil {
		t.Fatalf("the report is not json: %v\n%s", err, got.out)
	}
	for _, key := range []string{"installation", "servers", "work"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("the document carries no %q:\n%s", key, got.out)
		}
	}
	servers, ok := doc["servers"].([]any)
	if !ok {
		t.Fatalf("servers is %T, want an array even when empty: %s", doc["servers"], got.out)
	}
	if len(servers) != 0 {
		t.Errorf("servers = %v, want empty", servers)
	}

	inst, ok := doc["installation"].(map[string]any)
	if !ok {
		t.Fatalf("installation is %T", doc["installation"])
	}
	for _, key := range []string{"version", "schema_version", "engine", "observed_at", "store"} {
		if _, ok := inst[key]; !ok {
			t.Errorf("installation carries no %q: %v", key, inst)
		}
	}
	if inst["engine"] != "sqlite" {
		t.Errorf("engine = %v, want sqlite", inst["engine"])
	}

	work, ok := doc["work"].(map[string]any)
	if !ok {
		t.Fatalf("work is %T", doc["work"])
	}
	for _, key := range []string{
		"tenants", "projects", "tasks", "claimed",
		"leases_expired_unswept", "webhooks_pending", "webhooks_failed",
	} {
		if _, ok := work[key]; !ok {
			t.Errorf("work carries no %q: %v", key, work)
		}
	}
}

// TestStatusRendersEveryFormat checks the three formats a reader can ask for,
// since a format that renders the raw Go struct is the failure this catches.
func TestStatusRendersEveryFormat(t *testing.T) {
	c := newCLI(t)
	db := statusDB(c)

	tests := []struct {
		format string
		want   string
	}{
		{"table", "INSTALLATION"},
		{"json", `"installation"`},
		{"yaml", "installation:"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := c.mustRun("--db", db, "status", "-o", tt.format)
			if !strings.Contains(got.out, tt.want) {
				t.Errorf("-o %s never says %q:\n%s", tt.format, tt.want, got.out)
			}
		})
	}
}

// TestStatusIsNotDoctor states the difference the two commands exist to keep.
// Doctor decides whether this installation is sound and fails when it is not;
// status reports what exists and succeeds whenever it could produce the report.
func TestStatusIsNotDoctor(t *testing.T) {
	c := newCLI(t)
	db := statusDB(c)

	status := c.mustRun("--db", db, "status")
	doctor := c.mustRun("--db", db, "doctor")
	if strings.Contains(status.out, "identity") || strings.Contains(status.out, "conflict-files") {
		t.Errorf("status has started reporting doctor's checks:\n%s", status.out)
	}
	if strings.Contains(doctor.out, "SERVERS") {
		t.Errorf("doctor has started reporting the server list:\n%s", doctor.out)
	}
}
