// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// TestAFailedListingWritesNoDocument is the guard for an answer-shaped output
// that is not an answer: a failed `task ls -o json` printed "[]" on standard
// output, its error on standard error and exited non-zero, so a pipeline
// reading only stdout read a valid empty array and concluded there were no
// tasks.
//
// The listing is failed by an unreadable page cursor, which fails inside the
// listing loop with the writer already open and depends on no other filter's
// resolution rules, so this guard rests on nothing but the writer it guards.
func TestAFailedListingWritesNoDocument(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("task", "add", "-p", "infra", "a real task")

	for _, format := range []string{output.FormatJSON, output.FormatNDJSON, output.FormatYAML, output.FormatTable} {
		got := c.run("task", "ls", "--cursor", "garbage", "-o", format)
		if got.code != core.KindInvalid.ExitCode() {
			t.Fatalf("-o %s exited %d, want %d for an unreadable cursor",
				format, got.code, core.KindInvalid.ExitCode())
		}
		if got.out != "" {
			t.Errorf("-o %s wrote %q to stdout; a failed listing has no answer to give", format, got.out)
		}
		if !strings.Contains(got.err, "cursor") {
			t.Errorf("-o %s did not name the failure on stderr: %q", format, got.err)
		}
	}
}

// TestAPartialJSONListingIsLeftUnterminated pins the half of the decision that
// cannot be undone: records already on the wire stay there, but the array
// bracket is withheld, so a consumer parsing stdout gets a syntax error rather
// than a short array it would believe.
func TestAPartialJSONListingIsLeftUnterminated(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	l := newList[map[string]string](&globals{format: output.FormatJSON}, cmd)
	if err := l.Write(map[string]string{"ref": "infra-1"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := l.fail(errors.New("upstream went away")); err == nil {
		t.Fatal("fail returned no error")
	}
	if !strings.Contains(buf.String(), "infra-1") {
		t.Fatalf("stdout = %q, want the record that was already streamed", buf.String())
	}
	var decoded []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &decoded); err == nil {
		t.Fatalf("stdout %q parsed as a complete array of %d records", buf.String(), len(decoded))
	}
}

// TestAPartialNDJSONListingKeepsItsLines states the other half: a line-oriented
// format has no terminator to withhold, so the lines already written stand and
// the exit status carries the failure.
func TestAPartialNDJSONListingKeepsItsLines(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	l := newList[map[string]string](&globals{format: output.FormatNDJSON}, cmd)
	if err := l.Write(map[string]string{"ref": "infra-1"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := l.fail(errors.New("upstream went away")); err == nil {
		t.Fatal("fail returned no error")
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout = %q, want exactly the one record already written", buf.String())
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil || decoded["ref"] != "infra-1" {
		t.Fatalf("line %q is not the record that was written: %v", lines[0], err)
	}
}
