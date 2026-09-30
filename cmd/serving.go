// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"io"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// boundServer is a server that binds its address before it accepts on it,
// which both the HTTP server and the SSH listener are.
type boundServer interface {
	Listen() error
	Serve(ctx context.Context) error
}

// serveUntilSignal installs the interrupt handler, binds, announces the bound
// address and serves until a signal arrives.
//
// The order is why this is one function rather than four lines at each call
// site. Between a bound port and a registered handler, Go's default action for
// SIGINT still terminates the process, so an operator interrupting a server
// that has already accepted connections would kill it instead of draining it.
func serveUntilSignal(cmd *cobra.Command, srv boundServer, announce func(io.Writer)) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := srv.Listen(); err != nil {
		return err
	}
	announce(cmd.OutOrStdout())
	return srv.Serve(ctx)
}
