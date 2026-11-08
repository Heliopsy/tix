// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/store"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// SyncHost is what the operator lets external sync reach on the host that
// runs it: files, and the TIX_SYNC_<SOURCE>_* environment.
//
// A service is shared unless Trusted is set: it answers people other than the
// operator, as tix serve does, so a path or a source name a tenant sends is
// never enough to read a server file or the server's environment.
type SyncHost struct {
	// Trusted marks a service run by the operator on their own behalf, a
	// command against a database directly, where every path and variable is
	// already theirs.
	Trusted bool
	// FileRoot holds one directory per tenant key that a shared service lets
	// that tenant's sources name files in. Empty, no tenant names a file.
	FileRoot string
	// EnvSources lists, as TENANT/SOURCE or TENANT/*, the sources a shared
	// service lets read the TIX_SYNC_<SOURCE>_* environment.
	EnvSources []string
}

// WithSyncHost sets what external sync may reach on this host. Without it a
// service is shared with no file root and no environment binding. An
// unparseable binding binds nothing; config validation refuses one at start.
func WithSyncHost(h SyncHost) Option {
	return func(l *Local) {
		l.syncHost = h
		l.syncEnvBindings, _ = extsync.ParseEnvBindings(h.EnvSources)
	}
}

// syncScope is what one tenant's sources may reach on this host.
type syncScope struct {
	tenantKey string
	trusted   bool
	egress    egress.Policy
	fileDir   string
	bindings  extsync.EnvBindings
}

// syncScopeFor works out what the actor's tenant may reach.
func (l *Local) syncScopeFor(ctx context.Context, actor *core.Actor) (syncScope, error) {
	out := syncScope{trusted: l.syncHost.Trusted, bindings: l.syncEnvBindings, egress: l.egress}
	if out.trusted {
		out.egress = egress.Permissive()
	}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		t, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		out.tenantKey = t.Key
		return nil
	}); err != nil {
		return out, err
	}
	if root := strings.TrimSpace(l.syncHost.FileRoot); root != "" && filepath.IsLocal(out.tenantKey) {
		out.fileDir = filepath.Join(root, out.tenantKey)
	}
	return out, nil
}

// envBound reports whether a source reads the server's environment.
func (s syncScope) envBound(src core.SyncSource) bool {
	return s.trusted || s.bindings.Allows(s.tenantKey, src.Name)
}

// getenv is the environment as a source sees it: the server's when the
// operator bound it, and nothing otherwise, so an unbound source can neither
// use a variable nor learn whether one is set.
func (s syncScope) getenv(src core.SyncSource) func(string) string {
	if !s.envBound(src) {
		return func(string) string { return "" }
	}
	return syncEnv
}

// checkStoredURL refuses a base url the egress policy already refuses by its
// text alone, before it is stored. Names are not resolved here: the dialer
// decides those on every run.
func (s syncScope) checkStoredURL(system, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || s.egress.CheckURL(raw) == nil {
		return nil
	}
	return core.OnField(core.Invalid("%s url: address not allowed; this server does not reach loopback, "+
		"link-local or private addresses for a tenant unless its operator allows them in server.egress_allow",
		system), core.SyncSettingURL)
}

// confinesPaths reports whether paths stored on a source are held to the
// tenant's directory.
func (s syncScope) confinesPaths() bool { return !s.trusted }

// noFileRoot is the refusal for a stored server path on a shared host with no
// directory for the tenant.
func noFileRoot(what string) error {
	return core.Invalid("this server does not read %s named by a tenant; upload the document instead, "+
		"or ask its operator to set sync.file_root and put the file in this tenant's directory", what)
}

// checkStoredPath refuses a server path a tenant may not name, before it is
// stored. It looks at the path only, so the answer says nothing about what
// exists on the server.
func (s syncScope) checkStoredPath(field, what, path string) error {
	if path == "" || !s.confinesPaths() {
		return nil
	}
	if s.fileDir == "" {
		return core.OnField(noFileRoot(what), field)
	}
	if _, err := extsync.ConfinedName(s.fileDir, path); err != nil {
		return core.OnField(err, field)
	}
	return nil
}

// readStored reads a path stored on a source: anywhere for a trusted host, and
// only inside the tenant's directory otherwise.
func (s syncScope) readStored(what, path string) ([]byte, error) {
	if !s.confinesPaths() {
		// #nosec G304 -- a trusted host is the operator's own process.
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, core.Invalid("reading %s %q: %v", what, path, err)
		}
		return raw, nil
	}
	if s.fileDir == "" {
		return nil, noFileRoot(what)
	}
	return extsync.ReadConfined(s.fileDir, path)
}

// storedPathUnreadable says why a stored path cannot be read, or "" when it
// can. A shared host answers the same for a missing file, one outside the
// tenant's directory and one behind a link out of it.
func (s syncScope) storedPathUnreadable(what, path string) string {
	if strings.Contains(path, "..") {
		return what + " " + path + " (a path with \"..\" is not read)"
	}
	if !s.confinesPaths() {
		if _, err := os.Stat(path); err != nil {
			return what + " " + path + " (the server cannot read it)"
		}
		return ""
	}
	if s.fileDir == "" {
		return what + " " + path + " (this server reads no file a tenant names; upload it, or ask the operator to set sync.file_root)"
	}
	if err := extsync.CheckConfined(s.fileDir, path); err != nil {
		return what + " " + path + " (not readable in this tenant's sync directory)"
	}
	return ""
}
