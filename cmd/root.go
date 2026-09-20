// Package cmd contains all Cobra CLI commands for tix.
//
// This layer holds flag definitions and wiring only. All business logic lives
// in internal/; no package under internal/ may import Cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/tix/internal/version"
)

var rootCmd = &cobra.Command{
	Use:           "tix",
	Short:         "Task management for humans and AI agents",
	Long:          "tix manages tasks for humans and AI agents over one shared store, via CLI, TUI, HTTP API, WebSocket, and web UI.",
	SilenceErrors: true,
	SilenceUsage:  true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), version.String())
		return err
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command and exits non-zero on failure.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
