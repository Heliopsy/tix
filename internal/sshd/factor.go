// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"charm.land/ssh"
	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// maxCodeAttempts is how many answers one session may give before it is
// closed. It is small because an attacker's budget is the enrolment's own
// rate limit, which counts across connections; the only thing a generous
// allowance here buys is a longer hold on a session slot.
const maxCodeAttempts = 3

// maxAnswerLength bounds one typed answer. A TOTP code is six digits and a
// recovery code is a short base32 string, so anything longer is a client
// pasting a file into the prompt.
const maxAnswerLength = 128

// recoveryCodeMarker prefixes each recovery code on screen, so the block can
// be read back as a list rather than as prose.
const recoveryCodeMarker = "  - "

// proveSecondFactor asks this session for its second factor, when one is
// owed, and answers an error when the session must not go on.
//
// It runs between the key being accepted and the interface being built, and
// that placement is the whole feature. A public key is one factor, not a
// second one, so the prompt is not part of an authentication exchange here
// the way it is in a browser: the connection already exists and the terminal
// is already the session's own. A prompt drawn after the board is on screen
// guards nothing, because the tenant's data has reached the terminal before
// the question was asked. Nothing in this file reads a task, a project or
// anything else belonging to the tenant.
func (s *Server) proveSecondFactor(sess ssh.Session, actor *core.Actor) error {
	ctx := core.WithSource(core.WithActor(sess.Context(), actor), core.SourceTUI)

	status, err := s.opts.Service.TOTPStatus(ctx)
	if err != nil {
		if core.IsKind(err, core.KindInvalid) {
			// An actor with no user row behind it holds no credential set a
			// factor could belong to. A demo sandbox's visitor is a key and
			// nothing else, so there is neither a factor to prove nor an
			// enrolment to take it through.
			return nil
		}
		return err
	}
	// The rule belongs to the service and is read from it rather than
	// rebuilt here: a factor is owed when the tenant requires one on this
	// surface, and also when the person holds a confirmed one of their own,
	// because a voluntary enrolment that protected no sign-in would protect
	// nothing at all.
	if !status.RequiredSSH && !status.Enrolled {
		return nil
	}
	if status.VerificationLocked && status.VerificationLockEnd != nil {
		return auth.ErrTOTPLocked(*status.VerificationLockEnd)
	}

	p := &prompter{server: s, sess: sess}
	if status.Enrolled {
		return p.askForCode(ctx)
	}
	return p.askToEnrol(ctx)
}

// prompter owns one session's terminal for the length of the exchange.
type prompter struct {
	server *Server
	sess   ssh.Session

	// pendingLF swallows the line feed of a carriage return pair. A client in
	// raw mode may send either ending, and one answer's trailing byte read as
	// the next answer would submit an empty line nobody typed.
	pendingLF bool
}

// askForCode asks an enrolled person to prove the factor they hold.
func (p *prompter) askForCode(ctx context.Context) error {
	p.write("This tenant asks for a second factor before the board opens.\r\n")
	for attempt := 1; attempt <= maxCodeAttempts; attempt++ {
		code, err := p.ask("Code from your authenticator, or a recovery code: ")
		if err != nil {
			return err
		}
		err = p.server.opts.Service.VerifySecondFactor(ctx, code)
		if err == nil {
			return nil
		}
		if core.IsTOTPLocked(err) {
			// Rate limiting is not a wrong code, and saying so sends
			// somebody looking for a clock problem they do not have.
			return err
		}
		if attempt < maxCodeAttempts {
			p.write(fmt.Sprintf("That code was not accepted; %d more %s.\r\n",
				maxCodeAttempts-attempt, tries(maxCodeAttempts-attempt)))
		}
	}
	return errFactorUnproved()
}

// askToEnrol takes an unenrolled person through enrolment on the terminal.
//
// Refusing them instead would make turning the requirement on lock out every
// user of the tenant, the administrator who turned it on first among them.
// A terminal cannot show a scannable image it can be sure the client renders,
// so it prints both forms an authenticator takes: the URI, which many clients
// make clickable, and the secret, which anybody can type.
func (p *prompter) askToEnrol(ctx context.Context) error {
	enrolment, err := p.server.opts.Service.BeginTOTPEnrolment(ctx)
	if err != nil {
		return err
	}
	p.write("This tenant asks for a second factor and this account has none yet.\r\n" +
		"Add one to an authenticator, by URI or by typing the secret:\r\n\r\n" +
		"  " + enrolment.URI + "\r\n\r\n" +
		"  secret:  " + enrolment.Secret + "\r\n" +
		"  account: " + enrolment.Account + "\r\n" +
		"  issuer:  " + enrolment.Issuer + "\r\n\r\n")

	for attempt := 1; attempt <= maxCodeAttempts; attempt++ {
		code, err := p.ask("Code from your authenticator: ")
		if err != nil {
			return err
		}
		codes, err := p.server.opts.Service.ConfirmTOTPEnrolment(ctx, code)
		if err == nil {
			return p.showRecoveryCodes(codes.Codes)
		}
		if core.IsTOTPLocked(err) {
			return err
		}
		if attempt < maxCodeAttempts {
			p.write(fmt.Sprintf("That code was not accepted; %d more %s.\r\n",
				maxCodeAttempts-attempt, tries(maxCodeAttempts-attempt)))
		}
	}
	return errFactorUnproved()
}

// showRecoveryCodes prints the codes and waits for an acknowledgement.
//
// The wait is not politeness. These are shown exactly once and nothing can
// produce them again, and the next thing this session does is take the
// alternate screen, which paints over everything above it.
func (p *prompter) showRecoveryCodes(codes []string) error {
	var b strings.Builder
	b.WriteString("\r\nEnrolled. These recovery codes are shown once and cannot be shown again.\r\n" +
		"Each works wherever a code is asked for, and each works only once.\r\n\r\n")
	for _, code := range codes {
		b.WriteString(recoveryCodeMarker + code + "\r\n")
	}
	b.WriteString("\r\n")
	p.write(b.String())
	_, err := p.ask("Press enter once you have saved them. ")
	return err
}

// ask writes a prompt and reads one answer, with no echo.
//
// The answer is not echoed because the same prompt takes a recovery code,
// which is a long-lived credential and would otherwise sit in the terminal's
// scrollback for the rest of the day. A TOTP code is on the person's phone,
// so showing it back gains nothing.
//
// The wait is bounded by the listener's idle timeout. The watchdog that ends
// a session nobody is typing at is started along with the interface, and the
// interface does not exist yet, so without this an unanswered prompt would
// hold a session slot and its key's share of the listener for as long as the
// client cared to wait.
func (p *prompter) ask(prompt string) (string, error) {
	p.write(prompt)

	type answer struct {
		text string
		err  error
	}
	done := make(chan answer, 1)
	go func() {
		text, err := p.readLine()
		done <- answer{text: text, err: err}
	}()

	select {
	case a := <-done:
		p.write("\r\n")
		if a.err != nil {
			return "", core.Unauthenticated("the second factor was not proved; this session is closing")
		}
		return a.text, nil
	case <-p.sess.Context().Done():
		return "", core.Unauthenticated("this session ended before the second factor was proved")
	case <-p.server.opts.Clock.After(p.server.opts.IdleTimeout):
		// The reader left behind is unblocked by the close that follows this
		// refusal, which is the only thing that can interrupt a read on a
		// session the client is holding open.
		return "", core.Unauthenticated(
			"the second factor went unanswered for %s; this session is closing",
			short(p.server.opts.IdleTimeout))
	}
}

// readLine reads one answer a byte at a time.
//
// The session's terminal is in raw mode, so there is no line discipline in
// the way: the carriage return, the erase character and the two interrupt
// characters all arrive as bytes and are handled here. Reading unbuffered
// matters, because a buffered reader built per prompt would swallow whatever
// the client had already sent beyond the line ending.
func (p *prompter) readLine() (string, error) {
	var out []byte
	buf := make([]byte, 1)
	for {
		n, err := p.sess.Read(buf)
		if n == 0 {
			if err != nil {
				return "", err
			}
			continue
		}
		switch c := buf[0]; {
		case c == '\n' && p.pendingLF:
			p.pendingLF = false
		case c == '\r':
			p.pendingLF = true
			return string(out), nil
		case c == '\n':
			return string(out), nil
		case c == 0x03 || c == 0x04:
			return "", io.EOF
		case c == 0x7f || c == 0x08:
			p.pendingLF = false
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		case c < 0x20:
			// Every other control byte, an escape sequence's lead among
			// them, is not part of an answer.
			p.pendingLF = false
		default:
			p.pendingLF = false
			if len(out) < maxAnswerLength {
				out = append(out, c)
			}
		}
		if err != nil {
			return string(out), err
		}
	}
}

// write puts text on the session's own terminal.
func (p *prompter) write(text string) {
	_, _ = io.WriteString(p.sess, text)
}

// errFactorUnproved ends a session that ran out of attempts.
func errFactorUnproved() error {
	return core.Unauthenticated("the second factor was not proved; this session is closing")
}

// tries renders the singular of an attempt count.
func tries(n int) string {
	if n == 1 {
		return "try"
	}
	return "tries"
}
