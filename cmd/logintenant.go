// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// tenantLogin is a sign-in with --tenant against a server, which ends in a
// context holding a token for that tenant rather than a printed session.
type tenantLogin struct {
	server  string
	tenant  string
	context string
}

// loginContext is what tix login --tenant reports: the context it wrote. The
// token itself stays in the configuration file.
type loginContext struct {
	Context   string `json:"context" yaml:"context"`
	Server    string `json:"server" yaml:"server"`
	Tenant    string `json:"tenant" yaml:"tenant"`
	User      string `json:"user" yaml:"user"`
	TokenName string `json:"token_name" yaml:"token_name"`
	Current   bool   `json:"current" yaml:"current"`
}

// loginService is the service a sign-in talks to. With --tenant against a
// server it names the tenant in the X-Tix-Tenant header, which a host serving
// several tenants reads and a host bound to another tenant refuses, and it
// returns where the resulting token is to be stored. A local database takes
// the tenant from the configuration as before.
func (g *globals) loginService(conn *connect.Conn, contextName string) (core.Service, *tenantLogin, error) {
	remote, ok := conn.Service.(*client.Client)
	if strings.TrimSpace(g.tenant) == "" || !ok {
		if contextName != "" {
			return nil, nil, core.Invalid("--context stores a token for a tenant on a server; pass --tenant KEY with a server")
		}
		return conn.Service, nil, nil
	}
	key := strings.ToLower(strings.TrimSpace(g.tenant))
	name := contextName
	if name == "" {
		if resolved, err := g.resolve(); err == nil {
			name = resolved.ContextName
		}
	}
	if name == "" {
		name = key
	}
	return remote.WithTenantHeader(key), &tenantLogin{
		server: conn.Info.Target.URL, tenant: key, context: name,
	}, nil
}

// storeTenantLogin exchanges a fresh session for an API token of the same
// actor, ends the session, and writes the token to the context. The session
// is a browser credential the API does not accept as a bearer token, so the
// command line keeps a token instead. The token carries account:read, so
// tix tenant ls and the terminal interface can list the person's tenants.
func (g *globals) storeTenantLogin(cmd *cobra.Command, ctx context.Context, tl *tenantLogin,
	session *core.Session,
) error {
	sess, err := client.New(tl.server, "", client.WithSessionCookie(session.Token),
		client.WithClientHint(core.ClientHintCLI))
	if err != nil {
		return err
	}
	defer func() { _ = sess.Close() }()
	sess = sess.WithTenantHeader(tl.tenant)
	name := "tix login " + tl.context + " " + time.Now().UTC().Format("20060102T150405Z")
	issued, err := sess.CreateToken(ctx, core.CreateTokenInput{
		Name: name, Scopes: []core.Scope{core.ScopeAll, core.ScopeAccountRead},
	})
	if lerr := sess.Logout(ctx); lerr != nil {
		g.diag(cmd, "the sign-in session could not be ended: %v", lerr)
	}
	if core.IsKind(err, core.KindForbidden) {
		return core.Forbidden("signed in to tenant %q, but your role there may not mint an API token; nothing "+
			"was written: ask an admin of %s for a token and add it with `tix ctx add %s --server %s "+
			"--tenant %s --token TOKEN`", tl.tenant, tl.tenant, tl.context, tl.server, tl.tenant)
	}
	if err != nil {
		return err
	}
	actor, err := g.confirmTenantToken(ctx, tl, issued.Token)
	if err != nil {
		return err
	}
	current, err := g.writeLoginContext(tl, issued.Token)
	if err != nil {
		return err
	}
	handle := actor.Handle
	for _, m := range actor.Memberships {
		if m.TenantKey == tl.tenant {
			handle = m.Handle
		}
	}
	return g.render(cmd, loginContext{
		Context: tl.context, Server: tl.server, Tenant: tl.tenant, User: handle,
		TokenName: name, Current: current,
	})
}

// confirmTenantToken asks the server, with the new token and no header, who it
// acts as, and refuses a token that lands in another tenant than the one
// signed in to, before anything is written.
func (g *globals) confirmTenantToken(ctx context.Context, tl *tenantLogin, token string) (*core.Actor, error) {
	c, err := client.New(tl.server, token, client.WithClientHint(core.ClientHintCLI))
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()
	actor, err := c.WhoAmI(ctx)
	if err != nil {
		return nil, err
	}
	if actor.TenantKey != "" && actor.TenantKey != tl.tenant {
		return nil, core.Internal("the server answered for tenant %q, not %q; nothing was written",
			actor.TenantKey, tl.tenant)
	}
	return actor, nil
}

// writeLoginContext stores the token on the context, keeping its project when
// the context already pointed at the same server, and makes it current when no
// context is. It reports whether the context is the current one.
func (g *globals) writeLoginContext(tl *tenantLogin, token string) (bool, error) {
	path := g.configFilePath()
	existing, cfg, err := readConfigFile(path)
	if err != nil {
		return false, err
	}
	before := *cfg
	entry := config.Context{Server: tl.server, Tenant: tl.tenant, Token: token}
	if old, ok := cfg.Contexts[tl.context]; ok && sameServer(old.Server, tl.server) {
		entry.Project = old.Project
	}
	if cfg.Contexts == nil {
		cfg.Contexts = map[string]config.Context{}
	} else {
		contexts := make(map[string]config.Context, len(cfg.Contexts)+1)
		for k, v := range cfg.Contexts {
			contexts[k] = v
		}
		cfg.Contexts = contexts
	}
	cfg.Contexts[tl.context] = entry
	if cfg.CurrentContext == "" {
		cfg.CurrentContext = tl.context
	}
	if err := config.SaveChanges(path, existing, &before, cfg); err != nil {
		return false, core.Internal("saving configuration").Wrap(err)
	}
	return cfg.CurrentContext == tl.context, nil
}

// sameServer compares two server addresses as a person typed them.
func sameServer(a, b string) bool {
	norm := func(s string) string { return strings.ToLower(strings.TrimRight(strings.TrimSpace(s), "/")) }
	return a != "" && norm(a) == norm(b)
}
