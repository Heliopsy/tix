// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
)

// This used to be a second filter parser, two hundred lines of it, written
// before internal/query existed and never told when the language grew.
//
// The cost was not the duplication. It was that the two parsers disagreed
// quietly. When the language gained negation and weak matching, "-tag:ops"
// failed here with a clear error, which is survivable, but "title~api" was
// swallowed as free text and matched nothing: an empty board, no error, and a
// user who reasonably concludes their tasks are gone. A filter that silently
// answers a different question than the one asked is worse than one that
// refuses.
//
// So the web bar, the TUI bar and `tix task ls --filter` now run the same
// parser and cannot drift again. internal/tui/filter.go is the same shim.

// FilterKeys are the term prefixes the filter bar accepts.
var FilterKeys = query.Keys

// FilterSyntaxHint names the operators a key list alone does not reveal.
var FilterSyntaxHint = query.SyntaxHint()

// ParseFilter turns a filter expression into the TaskFilter the CLI builds.
func ParseFilter(expression string) (core.TaskFilter, error) { return query.Parse(expression) }
