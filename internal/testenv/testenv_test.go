package testenv

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func reset() {
	mu.Lock()
	defer mu.Unlock()
	missing = map[string]Capability{}
}

func TestReportIsSilentWhenNothingWasSkipped(t *testing.T) {
	reset()
	t.Cleanup(reset)

	var buf bytes.Buffer
	Report(&buf)
	if buf.Len() != 0 {
		t.Fatalf("a complete run reported %q, want nothing", buf.String())
	}
}

func TestReportNamesEveryMissingCapabilityOnce(t *testing.T) {
	reset()
	t.Cleanup(reset)

	Record(Capability{Name: "postgres", Why: "no dsn", How: "just pg-up"})
	Record(Capability{Name: "postgres", Why: "a later, different reason", How: "ignored"})
	Record(Capability{Name: "symlinks", Why: "not permitted", How: "run on a filesystem that allows them"})

	var buf bytes.Buffer
	Report(&buf)
	out := buf.String()

	for _, want := range []string{
		"PARTIAL RUN", "2 capabilities were",
		"postgres: no dsn", "enable with: just pg-up",
		"symlinks: not permitted",
		Marker + " SKIPPED | postgres | no dsn | just pg-up",
		Marker + " SKIPPED | symlinks | not permitted | run on a filesystem that allows them",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report does not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "a later, different reason") {
		t.Errorf("the second recording of postgres overwrote the first:\n%s", out)
	}
	if n := strings.Count(out, Marker); n != 2 {
		t.Errorf("report carries %d marker lines, want 2:\n%s", n, out)
	}
}

func TestMissingIsOrderedByName(t *testing.T) {
	reset()
	t.Cleanup(reset)

	Record(Capability{Name: "symlinks"})
	Record(Capability{Name: "postgres"})
	Record(Capability{Name: "character-device"})

	got := Missing()
	want := []string{"character-device", "postgres", "symlinks"}
	if len(got) != len(want) {
		t.Fatalf("Missing() = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("Missing() = %v, want %v", got, want)
		}
	}
}

func TestReportUsesTheSingularForOneCapability(t *testing.T) {
	reset()
	t.Cleanup(reset)

	Record(Capability{Name: "postgres", Why: "no dsn", How: "just pg-up"})
	var buf bytes.Buffer
	Report(&buf)
	if !strings.Contains(buf.String(), "1 capability was") {
		t.Errorf("report does not use the singular:\n%s", buf.String())
	}
}

func TestPostgresDSNSkipsAndRecordsWhenUnset(t *testing.T) {
	reset()
	t.Cleanup(reset)
	t.Setenv(PostgresEnv, "")

	fake := &recordingTB{TB: t}
	func() {
		defer func() { _ = recover() }()
		PostgresDSN(fake)
	}()

	if !fake.skipped {
		t.Fatal("PostgresDSN did not skip when the dsn is unset")
	}
	gaps := Missing()
	if len(gaps) != 1 || gaps[0].Name != "postgres" {
		t.Fatalf("recorded %v, want the postgres capability", gaps)
	}
	if gaps[0].How != PostgresHow {
		t.Errorf("the skip does not say how to enable postgres: %q", gaps[0].How)
	}
}

func TestPostgresDSNReturnsTheTrimmedValue(t *testing.T) {
	reset()
	t.Cleanup(reset)
	t.Setenv(PostgresEnv, "  postgres://example  ")

	if got := PostgresDSN(t); got != "postgres://example" {
		t.Errorf("PostgresDSN() = %q, want %q", got, "postgres://example")
	}
	if gaps := Missing(); len(gaps) != 0 {
		t.Errorf("a configured dsn recorded %v, want nothing", gaps)
	}
}

// recordingTB captures a skip instead of ending the test that triggered it.
type recordingTB struct {
	testing.TB
	skipped bool
}

func (r *recordingTB) Skipf(string, ...any) {
	r.skipped = true
	panic("skip")
}

func (r *recordingTB) Helper() {}

func TestAppendLogWritesOneMarkedLinePerCapability(t *testing.T) {
	reset()
	t.Cleanup(reset)
	path := filepath.Join(t.TempDir(), "env.log")
	t.Setenv(LogEnv, path)

	Record(Capability{Name: "postgres", Why: "no dsn", How: "just pg-up"})
	AppendLog()
	Record(Capability{Name: "symlinks", Why: "not permitted", How: "another filesystem"})
	AppendLog()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("log holds %d lines, want 3 (one, then both):\n%s", len(lines), raw)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, Marker+" SKIPPED | ") {
			t.Errorf("line %q is not machine-readable", line)
		}
	}
}

func TestAppendLogIsSilentWithoutAPath(t *testing.T) {
	reset()
	t.Cleanup(reset)
	t.Setenv(LogEnv, "")
	Record(Capability{Name: "postgres", Why: "no dsn", How: "just pg-up"})

	AppendLog() // must not panic, and must not fail the run
}

func TestAppendLogSurvivesAnUnwritablePath(t *testing.T) {
	reset()
	t.Cleanup(reset)
	t.Setenv(LogEnv, filepath.Join(t.TempDir(), "missing-dir", "env.log"))
	Record(Capability{Name: "postgres", Why: "no dsn", How: "just pg-up"})

	AppendLog() // a report that cannot be written must not break the suite
}
