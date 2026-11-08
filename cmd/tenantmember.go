// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// membershipRow is one of the caller's tenants as tix tenant ls and tix whoami
// list them. Credential names the context holding a token for the tenant, or
// says the database is opened directly.
type membershipRow struct {
	Key        string    `json:"key" yaml:"key" show:"key"`
	Name       string    `json:"name" yaml:"name"`
	Role       core.Role `json:"role" yaml:"role"`
	Handle     string    `json:"handle" yaml:"handle"`
	Host       string    `json:"host,omitempty" yaml:"host,omitempty"`
	Current    bool      `json:"current" yaml:"current"`
	Credential string    `json:"credential,omitempty" yaml:"credential,omitempty"`
}

// credentialLocal marks a tenant a local database reaches without a token.
const credentialLocal = "local"

// tenantCredentials maps each tenant key to the context holding a token for it
// on the server the connection talks to. The tenant the connection is in maps
// to the context in use. A local database reaches every tenant, so it answers
// nil and callers treat every tenant as reachable.
func (g *globals) tenantCredentials(conn *connect.Conn, current string) map[string]string {
	target := conn.Info.Target
	if target.Mode != connect.ModeRemote {
		return nil
	}
	out := map[string]string{}
	if current != "" {
		out[current] = target.Context
		if out[current] == "" {
			out[current] = "this command's token"
		}
	}
	resolved, err := g.resolve()
	if err != nil {
		return out
	}
	for _, name := range contextNames(resolved.Config.Contexts) {
		c := resolved.Config.Contexts[name]
		key := strings.ToLower(strings.TrimSpace(c.Tenant))
		if key == "" || strings.TrimSpace(c.Token) == "" || !sameServer(c.Server, target.URL) {
			continue
		}
		if _, held := out[key]; !held {
			out[key] = name
		}
	}
	return out
}

// membershipRows renders the caller's memberships, marking the tenant the
// connection is in and the ones a stored credential reaches.
func membershipRows(who *core.Actor, creds map[string]string) []membershipRow {
	rows := make([]membershipRow, 0, len(who.Memberships))
	for _, m := range who.Memberships {
		row := membershipRow{
			Key: m.TenantKey, Name: m.TenantName, Role: m.Role, Handle: m.Handle, Host: m.Host,
			Current: m.TenantID == who.TenantID,
		}
		if creds == nil {
			row.Credential = credentialLocal
		} else {
			row.Credential = creds[m.TenantKey]
		}
		rows = append(rows, row)
	}
	return rows
}

// listMemberships answers tix tenant ls for a caller who is no global admin
// and whose credential lists its memberships. ok is false otherwise, and the
// caller lists tenants as before.
func (g *globals) listMemberships(cmd *cobra.Command, ctx context.Context, conn *connect.Conn) (bool, error) {
	who := memberView(ctx, conn.Service)
	if who == nil || who.GlobalAdmin {
		return false, nil
	}
	return true, g.render(cmd, membershipRows(who, g.tenantCredentials(conn, who.TenantKey)))
}

// memberView is the caller as whoami describes them, or nil when whoami fails
// or lists no memberships, which leaves the caller to the older checks.
func memberView(ctx context.Context, svc core.Service) *core.Actor {
	who, err := svc.WhoAmI(ctx)
	if err != nil || len(who.Memberships) == 0 {
		return nil
	}
	return who
}

// dialForMemberships opens the current target, reporting whether it could:
// a target that cannot be opened leaves tix tenant use to its probe, as before.
func (g *globals) dialForMemberships(cmd *cobra.Command) (*connect.Conn, context.Context, bool) {
	conn, ctx, err := g.dial(cmd)
	return conn, ctx, err == nil
}

// useMembership answers tix tenant use KEY from the caller's memberships. A
// tenant the caller is not a member of is refused. The tenant the connection
// is in, or any tenant of a local database, is written as before; on a server
// another tenant switches to the context holding its token, and with none the
// command names the login that would store one and writes nothing. ok is
// false when the credential lists no memberships, and the caller probes as
// before.
func (g *globals) useMembership(cmd *cobra.Command, key string, dryRun bool,
	write func() error,
) (bool, error) {
	conn, ctx, dialled := g.dialForMemberships(cmd)
	if !dialled {
		return false, nil
	}
	who := memberView(ctx, conn.Service)
	if who == nil {
		return false, nil
	}
	member := false
	for _, m := range who.Memberships {
		member = member || m.TenantKey == key
	}
	if !member {
		if who.GlobalAdmin {
			return false, nil
		}
		return true, core.NotFound("you are not a member of tenant %q; tix tenant ls lists yours", key)
	}
	creds := g.tenantCredentials(conn, who.TenantKey)
	if creds == nil || key == who.TenantKey {
		return true, write()
	}
	name, held := creds[key]
	if !held {
		return true, core.Precondition("no context holds a credential for tenant %q; nothing was written: "+
			"run `tix login --tenant %s` to store one", key, key)
	}
	g.diag(cmd, "context %s holds your credential for tenant %s", name, key)
	return true, g.editConfig(cmd, dryRun, "ctx.use", name, func(cfg *config.Config) error {
		cfg.CurrentContext = name
		return nil
	})
}
