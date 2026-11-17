// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/sso"
	"github.com/heliopsy/tix/internal/sso/ssotest"
)

// runStdinEnv runs tix with extra environment and standard input.
func runStdinEnv(c *cli, stdin string, extra []string, args ...string) result {
	c.t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &errb, append(c.environ(), extra...), c.home)
	return result{code: code, out: out.String(), err: errb.String()}
}

// addCLIProvider adds provider kc against idp through tix tenant sso add.
func addCLIProvider(t *testing.T, c *cli, idp *ssotest.OIDC) core.SSOProvider {
	t.Helper()
	got := runStdinEnv(c, idp.ClientSecret+"\n", []string{totpKey}, "tenant", "sso", "add", "kc", "--kind", "oidc",
		"--name", "Keycloak", "--issuer", idp.Issuer(), "--client-id", idp.ClientID, "--client-secret", "-",
		"--role-map", "tix-admins=admin", "--allowed-domain", "example.com", "-o", "json")
	if got.code != core.ExitOK {
		t.Fatalf("sso add exited %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
	}
	if strings.Contains(got.out+got.err, idp.ClientSecret) {
		t.Fatalf("sso add printed the client secret:\n%s", got.out)
	}
	var p core.SSOProvider
	if err := json.Unmarshal([]byte(got.out), &p); err != nil {
		t.Fatalf("sso add printed no provider: %v\n%s", err, got.out)
	}
	return p
}

func TestTenantSSOCommands(t *testing.T) {
	c := newCLI(t)
	idp := ssotest.NewOIDC(t, time.Now)
	p := addCLIProvider(t, c, idp)
	if !p.SecretSet || p.RoleMapping["tix-admins"] != core.RoleAdmin || len(p.AllowedDomains) != 1 {
		t.Errorf("added provider = %+v", p)
	}
	listed := c.mustRunEnv([]string{totpKey}, "tenant", "sso", "ls", "-o", "json")
	if !strings.Contains(listed.out, `"key": "kc"`) && !strings.Contains(listed.out, `"key":"kc"`) {
		t.Errorf("sso ls does not list kc:\n%s", listed.out)
	}
	if strings.Contains(listed.out, idp.ClientSecret) {
		t.Error("sso ls printed the client secret")
	}
	edited := c.mustRunEnv([]string{totpKey}, "tenant", "sso", "edit", "kc", "--enabled=false", "--role-map", "", "-o", "json")
	var after core.SSOProvider
	if err := json.Unmarshal([]byte(edited.out), &after); err != nil || after.Enabled || after.RoleMapping != nil ||
		after.Issuer != idp.Issuer() {
		t.Errorf("edited provider = %+v, %v; want it off with no mapping and its issuer kept", after, err)
	}
	c.mustRunEnv([]string{totpKey}, "tenant", "sso", "rm", "kc")
	if got := runEnv(c, []string{totpKey}, "tenant", "sso", "show", "kc"); got.code != core.ExitNotFound {
		t.Errorf("show after rm exited %d, want not found", got.code)
	}
}

// TestSSOLoopbackSignsIn drives the loopback redirect the way a browser would:
// the opener follows the provider's redirect back to the listener.
func TestSSOLoopbackSignsIn(t *testing.T) {
	c := newCLI(t)
	idp := ssotest.NewOIDC(t, time.Now)
	addCLIProvider(t, c, idp)
	c.mustRun("user", "create", "ada@example.com", "--password", "correct horse battery", "--handle", "ada")
	url := c.loginServerWith(service.WithTOTPKey(strings.TrimPrefix(totpKey, "TIX_AUTH_TOTP_KEY=")),
		service.WithEgress(egress.Permissive()))
	remote, err := client.New(url, "")
	if err != nil {
		t.Fatal(err)
	}
	idp.SetClaims(map[string]any{"sub": "ada-1", "email": "ada@example.com", "email_verified": true})

	var printed bytes.Buffer
	opened := ""
	browser := func(address string) error {
		opened = address
		go func() {
			resp, err := http.Get(address)
			if err == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
		}()
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := sso.Loopback(ctx, remote, "kc", false, &printed, browser)
	if err != nil {
		t.Fatalf("ssoLoopback: %v\n%s", err, printed.String())
	}
	if result.Session == nil || result.Session.Token == "" {
		t.Fatalf("result = %+v, want a session", result)
	}
	if opened == "" || !strings.Contains(printed.String(), opened) {
		t.Errorf("the address was not printed beside opening it:\n%s", printed.String())
	}
	if !strings.Contains(opened, "redirect_uri=http%3A%2F%2F127.0.0.1%3A") {
		t.Errorf("the sign-in did not ask for a loopback redirect: %s", opened)
	}
}

func TestLoginSSOPicksTheOnlyProviderOrSaysWhichToName(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", "correct horse battery", "--handle", "ada")
	key := strings.TrimPrefix(totpKey, "TIX_AUTH_TOTP_KEY=")
	url := c.loginServerWith(service.WithTOTPKey(key), service.WithEgress(egress.Permissive()))
	if got := c.run("--server", url, "login", "--sso"); got.code == core.ExitOK || !strings.Contains(got.err, "no single sign-on provider") {
		t.Errorf("--sso with no provider exited %d: %s", got.code, got.err)
	}
	if got := c.run("--server", url, "login", "--sso", "--email", "a@example.com"); got.code != core.ExitUsage {
		t.Errorf("--sso beside --email exited %d, want usage", got.code)
	}
	for _, k := range []string{"one", "two"} {
		idp := ssotest.NewOIDC(t, time.Now)
		got := runStdinEnv(c, "", []string{totpKey}, "tenant", "sso", "add", k, "--issuer", idp.Issuer(), "--client-id", "x")
		if got.code != core.ExitOK {
			t.Fatalf("adding %s: %s", k, got.err)
		}
	}
	got := c.run("--server", url, "login", "--sso")
	if got.code != core.ExitUsage || !strings.Contains(got.err, "one, two") {
		t.Errorf("--sso with two providers exited %d: %s", got.code, got.err)
	}
}
