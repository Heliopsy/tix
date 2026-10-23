// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/qr"
)

// terminalReader is a standard input that knows for itself whether it is a
// terminal. The real check reads a file descriptor, which the test harness
// cannot produce: it drives the whole command tree through an in-memory
// reader, and a prompt is not testable at all unless that reader can say so.
type terminalReader interface{ IsTerminal() bool }

// onATerminal reports whether this invocation can ask a person a question.
func onATerminal(cmd *cobra.Command) bool {
	switch in := cmd.InOrStdin().(type) {
	case terminalReader:
		return in.IsTerminal()
	case *os.File:
		return term.IsTerminal(in.Fd())
	default:
		return false
	}
}

// errNoTerminal refuses a prompt nobody is there to answer.
//
// It names API tokens rather than reporting only that there is no terminal,
// because the caller that hits this is a script, and the reason no flag
// carries a code is that scripts are meant to use a token instead. An error
// that said "no terminal" alone would be read as a bug in tix.
func errNoTerminal(what string) error {
	return core.Precondition(
		"%s needs a terminal to prompt for a code and there is none; "+
			"an unattended caller authenticates with an API token, which is exempt from the second factor: "+
			"see tix token create", what)
}

// ask writes a prompt to standard error and reads one line of answer.
//
// The prompt goes to standard error so that standard output carries only what
// the command was asked for. `tix login -o json` emits a session document
// that a caller parses, and a question mixed into it would break that.
func (g *globals) ask(cmd *cobra.Command, prompt string) (string, error) {
	errw := cmd.ErrOrStderr()
	_, _ = fmt.Fprint(errw, output.NewPainter(g.colorMode(), errw).Muted(prompt))
	line, err := g.stdin(cmd).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", core.Internal("reading the answer").Wrap(err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", core.Invalid("no code was given")
	}
	return line, nil
}

// stdin returns the buffered reader every prompt shares. A fresh reader per
// prompt would read ahead and swallow the answer to the next one.
func (g *globals) stdin(cmd *cobra.Command) *bufio.Reader {
	if g.prompts == nil {
		g.prompts = bufio.NewReader(cmd.InOrStdin())
	}
	return g.prompts
}

// recoveryCodeHeading introduces the codes wherever they are printed rather
// than rendered. It is a constant so that a test can count the times they
// were shown: exactly once is the whole property.
const recoveryCodeHeading = "recovery codes, shown once and never again:"

// showRecoveryCodes writes the codes to standard error.
//
// Not through diag, and not to standard output: these are the only copy that
// will ever exist, so --quiet must not swallow them, while standard output
// belongs to the session document the sign-in is there to produce.
func showRecoveryCodes(cmd *cobra.Command, codes []string) {
	if len(codes) == 0 {
		return
	}
	errw := cmd.ErrOrStderr()
	_, _ = fmt.Fprintln(errw, recoveryCodeHeading)
	for _, code := range codes {
		_, _ = fmt.Fprintln(errw, "  "+code)
	}
}

// showEnrolment draws the authenticator entry on standard error: the
// otpauth:// URI always, and a scannable symbol where the destination can
// carry one.
//
// A symbol needs explicit colours to be legible whatever background the
// terminal has, so whether colour is available is the same question as
// whether a code can be drawn at all. Where it cannot, the URI alone still
// enrols, by paste or by hand.
//
// Standard error rather than standard output, because the URI carries the
// secret: it belongs on the screen of the person enrolling and not in the
// document a caller redirects to a file.
func (g *globals) showEnrolment(cmd *cobra.Command, enrolment *core.TOTPEnrolment) {
	errw := cmd.ErrOrStderr()
	if output.NewPainter(g.colorMode(), errw).Enabled() {
		symbol, err := qr.Encode(enrolment.URI)
		if err == nil {
			_, _ = fmt.Fprint(errw, symbol.Terminal())
		} else {
			g.diag(cmd, "the enrolment will not fit in a scannable code: %v", err)
		}
	}
	_, _ = fmt.Fprintf(errw, "%s\n", enrolment.URI)
	g.diag(cmd, "scan the code above, or add the uri by hand, as %s on %s",
		enrolment.Account, enrolment.Issuer)
}
