// SPDX-License-Identifier: AGPL-3.0-or-later

package sso

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// LoopbackService is the part of the service a command-line sign-in calls.
type LoopbackService interface {
	BeginSSOLogin(ctx context.Context, in core.BeginSSOInput) (*core.SSOBegin, error)
	CompleteSSOLogin(ctx context.Context, in core.CompleteSSOInput) (*core.LoginResult, error)
}

// loopbackPage is what the browser shows once it has handed the code over.
const loopbackPage = `<!doctype html><meta charset="utf-8"><title>tix</title>` +
	`<p>%s</p><p>You can close this tab and go back to the terminal.</p>`

// Loopback runs one sign-in through a provider over a loopback redirect.
// open shows the address in a browser; nil, or a failure to open one, leaves
// the printed address for the person to open themselves.
func Loopback(ctx context.Context, svc LoopbackService, provider string, remember bool, errw io.Writer,
	open func(string) error,
) (*core.LoginResult, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, core.Internal("listening on 127.0.0.1 for the sign-in to come back").Wrap(err)
	}
	defer func() { _ = ln.Close() }()
	redirect := fmt.Sprintf("http://%s/callback", ln.Addr().String())
	begin, err := svc.BeginSSOLogin(ctx, core.BeginSSOInput{Provider: provider, RedirectURI: redirect,
		Surface: core.SurfaceCLI, Remember: remember})
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(errw, "open this address to sign in:\n  %s\n", begin.AuthURL)
	if open != nil {
		if err := open(begin.AuthURL); err != nil {
			_, _ = fmt.Fprintln(errw, "no browser could be opened; open the address yourself")
		}
	}

	answers := make(chan core.CompleteSSOInput, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: loopbackHandler(begin, answers)}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	wait, cancel := context.WithTimeout(ctx, auth.SSOFlowTTL)
	defer cancel()
	select {
	case in := <-answers:
		return svc.CompleteSSOLogin(ctx, in)
	case <-wait.Done():
		if errors.Is(wait.Err(), context.DeadlineExceeded) {
			return nil, core.Unauthenticated("the sign-in did not come back within %s; run tix login --sso again",
				auth.SSOFlowTTL)
		}
		return nil, wait.Err()
	}
}

// loopbackHandler takes the provider's redirect. A request whose state is
// not this sign-in's is refused and waited past, so another page on the
// machine cannot end the sign-in with a code of its own.
func loopbackHandler(begin *core.SSOBegin, answers chan<- core.CompleteSSOInput) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/callback" || q.Get("state") != begin.State {
			http.Error(w, "this is not the sign-in tix is waiting for", http.StatusBadRequest)
			return
		}
		in := core.CompleteSSOInput{Flow: begin.Flow, Code: q.Get("code"), State: q.Get("state"),
			Error: strings.TrimSpace(q.Get("error") + " " + q.Get("error_description"))}
		select {
		case answers <- in:
		default:
		}
		msg := "Signed in to tix."
		if in.Error != "" {
			msg = "The identity provider ended the sign-in: " + in.Error
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, loopbackPage, html.EscapeString(msg))
	})
}
