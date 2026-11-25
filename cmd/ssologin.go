// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sso"
)

// ssoAuto is what --sso holds when given without a provider key.
const ssoAuto = "auto"

// loginWithSSO signs in through one of the tenant's providers with a loopback
// redirect: it listens on 127.0.0.1, opens the provider in the browser, and
// completes the sign-in with the code the browser brings back.
func (g *globals) loginWithSSO(cmd *cobra.Command, provider, contextName string, remember, noBrowser bool) error {
	conn, ctx, err := g.dialAnonymous(cmd)
	if err != nil {
		return err
	}
	svc, tl, err := g.loginService(conn, contextName)
	if err != nil {
		return err
	}
	if provider == ssoAuto {
		if provider, err = pickProvider(ctx, svc); err != nil {
			return err
		}
	}
	open := openInBrowser
	if noBrowser {
		open = nil
	}
	result, err := sso.Loopback(ctx, svc, provider, remember, cmd.ErrOrStderr(), open)
	if err != nil {
		return err
	}
	return g.finishLogin(cmd, svc, ctx, result, tl, remember)
}

// pickProvider is the tenant's one enabled provider, or a refusal naming them
// all when there is a choice to make.
func pickProvider(ctx context.Context, svc core.Service) (string, error) {
	options, err := svc.SignInOptions(ctx)
	if err != nil {
		return "", err
	}
	var keys []string
	saml := 0
	for _, p := range options.Providers {
		if p.Kind == core.SSOKindSAML {
			saml++
			continue
		}
		keys = append(keys, p.Key)
	}
	switch {
	case len(keys) == 0 && saml > 0:
		return "", core.Precondition("this tenant signs in through SAML, which works in the browser only; sign in " +
			"there, then create a token on the Connect screen for this machine")
	case len(keys) == 0:
		return "", core.Precondition("this tenant has no single sign-on provider; sign in with --email and --password")
	case len(keys) == 1:
		return keys[0], nil
	}
	return "", core.Invalid("this tenant has several providers; name one with --sso KEY: %s", strings.Join(keys, ", "))
}

// openInBrowser opens an address in the desktop's browser.
func openInBrowser(address string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", address) // #nosec G204 -- the provider's authorization address, not shell text
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", address) // #nosec G204 -- as above
	default:
		c = exec.Command("xdg-open", address) // #nosec G204 -- as above
	}
	return c.Start()
}
