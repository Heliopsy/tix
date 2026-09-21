package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// ExitInterrupt is the status returned when the interface is interrupted.
const ExitInterrupt = 130

// Options configure one run of the terminal interface.
type Options struct {
	Service core.Service
	Actor   *core.Actor
	Context context.Context
	Project string
	Filter  string
	// Scheme names the keybinding preset and Overrides rebinds single actions.
	Scheme    string
	Overrides map[string]string
	// TimeStyle renders every timestamp the interface draws. The zero value
	// still works: it renders the compact layout in the machine's local zone,
	// so a caller that has not wired configuration through yet is not broken.
	TimeStyle output.TimeStyle
	In        io.Reader
	Out       io.Writer
	Err       io.Writer
	Environ   []string
}

// Run starts the terminal interface and returns the process exit code.
func Run(o Options) int {
	ctx := o.Context
	if ctx == nil {
		ctx = context.Background()
	}
	errw := o.Err
	if errw == nil {
		errw = os.Stderr
	}
	model := New(Config{
		Service: o.Service, Context: ctx, Actor: o.Actor,
		Environ: o.Environ, Out: o.Out, Project: o.Project, Filter: o.Filter,
		Scheme: o.Scheme, Overrides: o.Overrides, TimeStyle: o.TimeStyle,
	})
	final, err := tea.NewProgram(model, programOptions(ctx, o)...).Run()
	return exitStatus(final, err, errw)
}

// programOptions builds the bubbletea options for a run.
func programOptions(ctx context.Context, o Options) []tea.ProgramOption {
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithContext(ctx)}
	if customInput(o.In) {
		opts = append(opts, tea.WithInput(o.In))
	}
	if o.Out != nil {
		opts = append(opts, tea.WithOutput(o.Out))
	}
	return opts
}

// customInput reports whether reading from r needs bubbletea's custom input
// path, which leaves the terminal alone rather than switching it to raw mode.
func customInput(r io.Reader) bool {
	return r != nil && r != io.Reader(os.Stdin)
}

// exitStatus maps the way the interface ended onto a process exit code.
func exitStatus(final tea.Model, err error, errw io.Writer) int {
	if err != nil {
		if errors.Is(err, tea.ErrProgramKilled) || errors.Is(err, context.Canceled) {
			return ExitInterrupt
		}
		_, _ = fmt.Fprintf(errw, "error: %v\n", err)
		return core.ExitError
	}
	m, ok := final.(Model)
	if !ok {
		return core.ExitOK
	}
	switch {
	case m.interrupted:
		return ExitInterrupt
	case m.fatal != nil:
		_, _ = fmt.Fprintf(errw, "error: %v\n", m.fatal)
		return core.KindOf(m.fatal).ExitCode()
	default:
		return core.ExitOK
	}
}
