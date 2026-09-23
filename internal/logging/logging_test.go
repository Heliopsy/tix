// SPDX-License-Identifier: AGPL-3.0-or-later

package logging_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/logging"
)

func TestNewRoutesToTheNamedDestination(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "tix.log")
	cases := []struct {
		name   string
		output string
		want   func(t *testing.T, out, errw *bytes.Buffer)
	}{
		{"stderr", logging.DestStderr, func(t *testing.T, out, errw *bytes.Buffer) {
			if !strings.Contains(errw.String(), "hello") {
				t.Fatalf("stderr holds %q, want the record", errw.String())
			}
			if out.Len() != 0 {
				t.Fatalf("stdout holds %q, want nothing", out.String())
			}
		}},
		{"stdout", logging.DestStdout, func(t *testing.T, out, errw *bytes.Buffer) {
			if !strings.Contains(out.String(), "hello") {
				t.Fatalf("stdout holds %q, want the record", out.String())
			}
			if errw.Len() != 0 {
				t.Fatalf("stderr holds %q, want nothing", errw.String())
			}
		}},
		{"file", path, func(t *testing.T, out, errw *bytes.Buffer) {
			body, err := os.ReadFile(path) // #nosec G304 -- a path this test created
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			if !strings.Contains(string(body), "hello") {
				t.Fatalf("%s holds %q, want the record", path, body)
			}
			if out.Len() != 0 || errw.Len() != 0 {
				t.Fatalf("a file destination also wrote to a stream: out=%q err=%q", out, errw)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errw bytes.Buffer
			log, closer, err := logging.New(logging.Options{
				Level: "info", Format: logging.FormatText, Output: tc.output,
				Stdout: &out, Stderr: &errw,
				File:  logging.FileOptions{MaxSizeMB: 1},
				Clock: clock.NewFakeAt(),
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			log.Info("hello")
			if err := closer.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
			tc.want(t, &out, &errw)
		})
	}
}

func TestNewFiltersByLevel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level     string
		wantDebug bool
		wantWarn  bool
	}{
		{"debug", true, true},
		{"info", false, true},
		{"warn", false, true},
		{"error", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			t.Parallel()
			var errw bytes.Buffer
			log, closer, err := logging.New(logging.Options{
				Level: tc.level, Format: logging.FormatText, Output: logging.DestStderr, Stderr: &errw,
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer func() { _ = closer.Close() }()
			log.Debug("a-debug-record")
			log.Warn("a-warn-record")
			if got := strings.Contains(errw.String(), "a-debug-record"); got != tc.wantDebug {
				t.Fatalf("at level %q a debug record present = %v, want %v", tc.level, got, tc.wantDebug)
			}
			if got := strings.Contains(errw.String(), "a-warn-record"); got != tc.wantWarn {
				t.Fatalf("at level %q a warn record present = %v, want %v", tc.level, got, tc.wantWarn)
			}
		})
	}
}

func TestNewJSONFormatEmitsParseableRecords(t *testing.T) {
	t.Parallel()
	var errw bytes.Buffer
	log, closer, err := logging.New(logging.Options{
		Level: "info", Format: logging.FormatJSON, Output: logging.DestStderr, Stderr: &errw,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = closer.Close() }()
	log.Info("request", "status", 200)

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(errw.Bytes()), &record); err != nil {
		t.Fatalf("the json handler emitted %q, which does not parse: %v", errw.String(), err)
	}
	if record["msg"] != "request" {
		t.Fatalf("record msg = %v, want \"request\"", record["msg"])
	}
	if record["status"] != float64(200) {
		t.Fatalf("record status = %v, want 200 as its own field", record["status"])
	}
}

func TestNewRefusesUnusableOptions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		opts logging.Options
		want string
	}{
		{"unknown level", logging.Options{Level: "trace", Format: logging.FormatText, Output: logging.DestStderr}, "log level"},
		{"unknown format", logging.Options{Level: "info", Format: "logfmt", Output: logging.DestStderr}, "log format"},
		{"empty output", logging.Options{Level: "info", Format: logging.FormatText}, "must name stderr"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := logging.New(tc.opts)
			if err == nil {
				t.Fatalf("New(%+v) succeeded, want an error about %q", tc.opts, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("New error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestStreamDistinguishesAPathFromAStream(t *testing.T) {
	t.Parallel()
	cases := []struct {
		output string
		want   bool
	}{
		{logging.DestStderr, true},
		{logging.DestStdout, true},
		{"  stderr  ", true},
		{"/var/log/tix.log", false},
		{"stderr.log", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.output, func(t *testing.T) {
			t.Parallel()
			if got := logging.Stream(tc.output); got != tc.want {
				t.Fatalf("Stream(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}
