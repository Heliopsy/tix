// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/heliopsy/tix/internal/testenv"
)

// TestMain keeps a stray interrupt from killing the test binary.
//
// Two tests prove a listener shuts down on a signal by sending SIGINT to this
// very process, because that is the only way to exercise the real handler the
// command installs. Between the signal being sent and that handler existing,
// and again after the command has deregistered it, Go's default action for
// SIGINT is to terminate. With -shuffle=on a test lands in that window
// eventually, and the whole package dies with "signal: interrupt" rather than
// failing an assertion, which says nothing about what broke.
//
// Registering a channel here disables the default action for the lifetime of
// the binary. The signal is still delivered to every other registered channel,
// so the command's own handler sees it exactly as before.
func TestMain(m *testing.M) {
	stray := make(chan os.Signal, 1)
	signal.Notify(stray, syscall.SIGINT)
	code := m.Run()
	// Not deferred: os.Exit does not run deferred calls, so a defer here would
	// read as cleanup while doing nothing.
	signal.Stop(stray)
	testenv.AppendLog()
	testenv.Report(os.Stderr)
	os.Exit(code)
}
