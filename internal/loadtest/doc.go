// SPDX-License-Identifier: AGPL-3.0-or-later

// Package loadtest drives a tix server with simulated browsers, phones and
// agents over plain HTTP, and reports latency, errors and the server's memory.
//
// It is a development tool, run by `just load` through internal/loadtest/tixload.
// It is not part of the tix binary and goreleaser never builds it.
package loadtest
