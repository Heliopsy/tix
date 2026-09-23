// SPDX-License-Identifier: AGPL-3.0-or-later

// Package logging builds the process logger from resolved configuration: the
// level it filters at, the handler it formats with, and the destination it
// writes to, which may be a rotating file.
package logging

import (
	"io"
	"log/slog"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// Destinations that name a stream rather than a file.
const (
	DestStderr = "stderr"
	DestStdout = "stdout"
)

// Output formats the handlers implement.
const (
	FormatText = "text"
	FormatJSON = "json"
)

// Levels lists the accepted level names, ordered from most to least verbose.
var Levels = []string{"debug", "info", "warn", "error"}

// Formats lists the accepted handler names.
var Formats = []string{FormatText, FormatJSON}

// Destinations lists the stream names an output setting may take. Any other
// value is a file path.
var Destinations = []string{DestStderr, DestStdout}

// ParseLevel resolves a level name.
func ParseLevel(name string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return 0, false
	}
}

// KnownFormat reports whether name is a handler this build implements.
func KnownFormat(name string) bool { return slices.Contains(Formats, name) }

// Stream reports whether an output setting names a stream rather than a file.
func Stream(output string) bool {
	return slices.Contains(Destinations, strings.TrimSpace(output))
}

// Options describes the logger to build.
type Options struct {
	Level  string
	Format string
	// Output is "stderr", "stdout", or the path of a file to rotate.
	Output string
	// Stderr and Stdout are the streams the named destinations resolve to.
	// They are injected so a test never has to capture the process streams.
	Stderr io.Writer
	Stdout io.Writer
	// File governs rotation and is read only for a file destination.
	File FileOptions
	// Clock dates rotated files and decides which are past MaxAge.
	Clock clock.Clock
}

// FileOptions bounds what a file destination may leave on disk.
type FileOptions struct {
	MaxSizeMB  int
	MaxAge     core.Duration
	MaxBackups int
	Compress   bool
}

// New builds a logger and the closer that releases its destination. The closer
// is never nil; for a stream destination it is a no-op, so a caller may always
// defer it without asking what the destination was.
func New(o Options) (*slog.Logger, io.Closer, error) {
	level, ok := ParseLevel(o.Level)
	if !ok {
		return nil, nil, core.Invalid("log level %q is not one of %s", o.Level, strings.Join(Levels, ", "))
	}
	if !KnownFormat(o.Format) {
		return nil, nil, core.Invalid("log format %q is not one of %s", o.Format, strings.Join(Formats, ", "))
	}

	w, closer, err := destination(o)
	if err != nil {
		return nil, nil, err
	}

	handlerOpts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if o.Format == FormatJSON {
		handler = slog.NewJSONHandler(w, handlerOpts)
	} else {
		handler = slog.NewTextHandler(w, handlerOpts)
	}
	return slog.New(handler), closer, nil
}

func destination(o Options) (io.Writer, io.Closer, error) {
	switch strings.TrimSpace(o.Output) {
	case "":
		return nil, nil, core.Invalid("log output must name stderr, stdout or a file path")
	case DestStderr:
		return orDiscard(o.Stderr), nopCloser{}, nil
	case DestStdout:
		return orDiscard(o.Stdout), nopCloser{}, nil
	}
	f, err := NewFile(strings.TrimSpace(o.Output), o.File, o.Clock)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

func orDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
