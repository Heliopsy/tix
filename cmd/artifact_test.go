package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// artifactsOf reads a task's artifacts back as json.
func artifactsOf(t *testing.T, c *cli, ref string) []core.Artifact {
	t.Helper()
	out := c.mustRun("artifact", "ls", ref, "-o", "json").out
	var artifacts []core.Artifact
	if err := json.Unmarshal([]byte(out), &artifacts); err != nil {
		t.Fatalf("artifact ls is not json: %v\n%s", err, out)
	}
	return artifacts
}

// An artifact can be attached from a flag, from standard input and from a
// file, and every source survives the round trip.
func TestArtifactPutAcceptsEveryInputSource(t *testing.T) {
	blobPath := filepath.Join(t.TempDir(), "build.log")
	if err := os.WriteFile(blobPath, []byte("compiled\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	tests := []struct {
		name     string
		args     []string
		stdin    string
		wantKind core.ArtifactKind
		wantBlob string
		wantKey  string
	}{
		{
			name:     "payload flag",
			args:     []string{"--name", "result", "--payload", `{"passed":true}`},
			wantKind: core.ArtifactResult,
			wantKey:  "passed",
		},
		{
			name:     "payload from standard input",
			args:     []string{"--kind", "metric", "--payload", "-"},
			stdin:    `{"duration_ms":12}`,
			wantKind: core.ArtifactMetric,
			wantKey:  "duration_ms",
		},
		{
			name:     "blob from a file",
			args:     []string{"--kind", "log", "--file", blobPath, "--content-type", "text/plain"},
			wantKind: core.ArtifactLog,
			wantBlob: "compiled\n",
		},
		{
			name:     "blob from standard input",
			args:     []string{"--kind", "file", "--file", "-"},
			stdin:    "raw bytes",
			wantKind: core.ArtifactFile,
			wantBlob: "raw bytes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			c.mustRun("task", "add", "ship it")
			args := append([]string{"artifact", "put", "default-1"}, tc.args...)
			got := c.runIn(tc.stdin, append(args, "-o", "json")...)
			if got.code != core.ExitOK {
				t.Fatalf("exit = %d\n%s", got.code, got.err)
			}

			artifacts := artifactsOf(t, c, "default-1")
			if len(artifacts) != 1 {
				t.Fatalf("artifact ls returned %d artifacts", len(artifacts))
			}
			a := artifacts[0]
			if a.Kind != tc.wantKind {
				t.Errorf("kind = %q, want %q", a.Kind, tc.wantKind)
			}
			if tc.wantKey != "" {
				if _, ok := a.Payload[tc.wantKey]; !ok {
					t.Errorf("payload = %v, want key %q", a.Payload, tc.wantKey)
				}
			}
			if tc.wantBlob != "" && string(a.Blob) != tc.wantBlob {
				t.Errorf("blob = %q, want %q", a.Blob, tc.wantBlob)
			}
		})
	}
}

// Input the service would refuse, and input the command itself cannot make
// sense of, both fail before anything is written.
func TestArtifactPutRejectsBadInput(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantErr  string
	}{
		{
			name:     "unknown task",
			args:     []string{"default-99", "--payload", "{}"},
			wantCode: core.ExitNotFound,
		},
		{
			name:     "payload that is not an object",
			args:     []string{"default-1", "--payload", "not json"},
			wantCode: core.ExitUsage,
			wantErr:  "json object",
		},
		{
			name:     "empty kind",
			args:     []string{"default-1", "--kind", "", "--payload", "{}"},
			wantCode: core.ExitUsage,
			wantErr:  "kind is required",
		},
		{
			name:     "two readers of standard input",
			args:     []string{"default-1", "--payload", "-", "--file", "-"},
			wantCode: core.ExitUsage,
			wantErr:  "standard input",
		},
		{
			name:     "missing file",
			args:     []string{"default-1", "--file", "no-such-file"},
			wantCode: core.ExitUsage,
			wantErr:  "reading artifact file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			c.mustRun("task", "add", "ship it")
			got := c.run(append([]string{"artifact", "put"}, tc.args...)...)
			if got.code != tc.wantCode {
				t.Fatalf("exit = %d, want %d\n%s", got.code, tc.wantCode, got.err)
			}
			if tc.wantErr != "" && !strings.Contains(got.err, tc.wantErr) {
				t.Fatalf("stderr = %q, want it to mention %q", got.err, tc.wantErr)
			}
			if len(artifactsOf(t, c, "default-1")) != 0 {
				t.Fatal("a rejected artifact was written anyway")
			}
		})
	}
}

// A dry run reports the attachment it would make and writes nothing.
func TestArtifactPutDryRunWritesNothing(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "ship it")
	got := c.mustRun("artifact", "put", "default-1", "--payload", "{}", "--dry-run", "-o", "json")
	if !strings.Contains(got.out, "artifact.put") || !strings.Contains(got.out, statusPlanned) {
		t.Fatalf("dry run output = %q", got.out)
	}
	if len(artifactsOf(t, c, "default-1")) != 0 {
		t.Fatal("a dry run attached an artifact")
	}
}

// Listing renders in every format the rest of the tree supports.
func TestArtifactListInEveryFormat(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "ship it")
	c.mustRun("artifact", "put", "default-1", "--name", "report", "--payload", `{"passed":true}`)

	for _, format := range []string{"table", "json", "yaml", "ndjson"} {
		t.Run(format, func(t *testing.T) {
			got := c.mustRun("artifact", "ls", "default-1", "-o", format)
			if !strings.Contains(got.out, "report") {
				t.Fatalf("%s output = %q", format, got.out)
			}
		})
	}
}
