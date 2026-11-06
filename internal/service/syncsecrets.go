// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// maxSyncMappingBytes bounds a stored mapping document. Real mappings are a
// few kilobytes; this is generous and still keeps a pasted log out of a row.
const maxSyncMappingBytes = 256 << 10

// syncCredentialHintMin is the shortest credential whose last four characters
// are shown. Four characters of a short password give too much of it away.
const syncCredentialHintMin = 12

// syncCredentials is the credential set sealed onto a source.
type syncCredentials struct {
	Token    string `json:"token,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

// checkSyncSourceInput refuses mapping and credential input that cannot be
// stored as given.
func checkSyncSourceInput(in core.SyncSourceInput) error {
	if in.ClearMapping && strings.TrimSpace(in.Mapping) != "" {
		return core.OnField(core.Invalid("send a mapping document or clear_mapping, not both"), "mapping")
	}
	if in.ClearCredentials && in.HasCredentials() {
		return core.OnField(core.Invalid("send credentials or clear_credentials, not both"), "token")
	}
	if in.HasCredentials() && in.System == core.SystemGeneric {
		return core.OnField(core.Invalid("a generic source reads a file on the server and takes no credentials"), "token")
	}
	if len(in.Mapping) > maxSyncMappingBytes {
		return core.OnField(core.Invalid("the mapping document is %d bytes; the limit is %d",
			len(in.Mapping), maxSyncMappingBytes), "mapping")
	}
	if strings.TrimSpace(in.Mapping) != "" {
		if _, err := extsync.ParseMapping([]byte(in.Mapping)); err != nil {
			return core.OnField(err, "mapping")
		}
	}
	return nil
}

// syncSealContext binds a sealed credential set to the one source it belongs
// to, so a ciphertext copied onto another source's row does not open there.
func syncSealContext(tenantID string, src core.SyncSource) string {
	return "sync-source\x00" + tenantID + "\x00" + src.System + "\x00" + src.Name
}

// sealSyncCredentials replaces the source's stored credentials with the ones
// the input carries, sealed under the server's key.
func (l *Local) sealSyncCredentials(tenantID string, src *core.SyncSource, in core.SyncSourceInput) error {
	if l.totpSealer == nil {
		return core.OnField(core.Precondition(
			"this server has no sealing key, so it cannot store credentials; its operator can set auth.totp_key "+
				"(TIX_AUTH_TOTP_KEY), or set %s in the server's environment instead",
			extsync.EnvName(src.Name, extsync.EnvToken)), "token")
	}
	creds := syncCredentials{
		Token:    strings.TrimSpace(in.Token),
		User:     strings.TrimSpace(in.User),
		Password: strings.TrimSpace(in.Password),
	}
	plain, err := json.Marshal(creds)
	if err != nil {
		return core.Internal("encoding sync credentials")
	}
	sealed, nonce, err := l.totpSealer.Seal(syncSealContext(tenantID, *src), string(plain))
	if err != nil {
		return core.Internal("sealing sync credentials")
	}
	src.SealedCredentials, src.CredentialsNonce = sealed, nonce
	src.Credentials = nil
	for _, c := range []struct{ name, value string }{
		{"token", creds.Token}, {"user", creds.User}, {"password", creds.Password},
	} {
		if c.value == "" {
			continue
		}
		hint := core.SyncCredentialHint{Name: c.name}
		if len(c.value) >= syncCredentialHintMin {
			hint.Hint = c.value[len(c.value)-4:]
		}
		src.Credentials = append(src.Credentials, hint)
	}
	return nil
}

// openSyncCredentials returns the credential set stored on a source.
func (l *Local) openSyncCredentials(tenantID string, src core.SyncSource) (syncCredentials, error) {
	var out syncCredentials
	if l.totpSealer == nil {
		return out, core.Invalid("the server's sealing key (auth.totp_key) is not set, so the credentials stored on %q cannot be opened", src.Name)
	}
	plain, err := l.totpSealer.Open(syncSealContext(tenantID, src), src.SealedCredentials, src.CredentialsNonce)
	if err != nil {
		return out, core.Invalid("the credentials stored on %q do not open with the server's sealing key; store them again", src.Name)
	}
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return out, core.Invalid("the credentials stored on %q do not open with the server's sealing key; store them again", src.Name)
	}
	return out, nil
}

// auditableSyncSource is a source as an audit entry records it: which
// credentials are stored, never even their hints.
func auditableSyncSource(src core.SyncSource) core.SyncSource {
	out := src
	out.Credentials = nil
	for _, c := range src.Credentials {
		out.Credentials = append(out.Credentials, core.SyncCredentialHint{Name: c.Name})
	}
	out.SealedCredentials, out.CredentialsNonce, out.Resolved = "", "", nil
	return out
}

// syncNeedsCredentials reports whether a system's adapter talks to a server
// that wants to know who is asking.
func syncNeedsCredentials(system string) bool {
	return system == core.SystemJira || system == core.SystemOpenProject
}

// resolveSync works out, on the server, which mapping, credentials and
// settings a run of src would use, and what it lacks. It reads no credential.
func (l *Local) resolveSync(src core.SyncSource) core.SyncResolution {
	env := func(suffix string) (string, string) {
		name := extsync.EnvName(src.Name, suffix)
		return name, strings.TrimSpace(syncEnv(name))
	}
	var out core.SyncResolution
	setUsage := "tix sync source set " + src.Name

	mappingVar, mappingEnv := env(extsync.EnvMapping)
	switch {
	case strings.TrimSpace(src.Mapping) != "":
		out.Mapping = core.SyncOrigin{From: core.SyncFromStored,
			Detail: "the document stored on the source"}
	case src.MappingPath != "":
		out.Mapping = core.SyncOrigin{From: core.SyncFromPath, Detail: src.MappingPath}
	case mappingEnv != "":
		out.Mapping = core.SyncOrigin{From: core.SyncFromEnv, Detail: mappingVar + "=" + mappingEnv}
	default:
		out.Mapping = core.SyncOrigin{From: core.SyncFromMissing}
		out.Missing = append(out.Missing, "a mapping document (upload one with "+setUsage+
			" --mapping-file FILE or in the source dialog, or set "+mappingVar+" to a file on the server)")
	}
	if path := mappingFileOf(out.Mapping, src.MappingPath, mappingEnv); path != "" {
		if _, err := os.Stat(path); err != nil {
			out.Missing = append(out.Missing, "the mapping file "+path+" (the server cannot read it; "+
				"upload the document itself with "+setUsage+" --mapping-file FILE)")
		}
	}

	out.Credentials = l.resolveSyncCredentials(src, env, setUsage, &out.Missing)

	for _, key := range core.SyncSettingKeys(src.System) {
		suffix, _ := extsync.SettingEnv(key)
		name, value := env(suffix)
		origin := core.SyncSettingOrigin{Key: key, Variable: name, From: core.SyncFromMissing}
		switch {
		case src.Config[key] != "":
			origin.From, origin.Value = core.SyncFromStored, src.Config[key]
		case value != "":
			origin.From, origin.Value = core.SyncFromEnv, value
		}
		out.Settings = append(out.Settings, origin)
	}
	out.Missing = append(out.Missing, missingSyncSettings(src, out.Settings)...)
	return out
}

// mappingFileOf is the file a run would read the mapping from, if any.
func mappingFileOf(origin core.SyncOrigin, stored, env string) string {
	switch origin.From {
	case core.SyncFromPath:
		return stored
	case core.SyncFromEnv:
		return env
	default:
		return ""
	}
}

// resolveSyncCredentials says where a run's credentials come from. A stored
// set wins whole: environment credentials are not mixed into it.
func (l *Local) resolveSyncCredentials(src core.SyncSource, env func(string) (string, string),
	setUsage string, missing *[]string) core.SyncOrigin {
	if src.SealedCredentials != "" {
		names := make([]string, 0, len(src.Credentials))
		for _, c := range src.Credentials {
			names = append(names, c.Name)
		}
		if l.totpSealer == nil {
			*missing = append(*missing, "the server's sealing key (credentials are stored on the source, "+
				"but auth.totp_key is not set, so they cannot be opened)")
		}
		return core.SyncOrigin{From: core.SyncFromStored, Detail: strings.Join(names, ", ")}
	}
	var set []string
	for _, suffix := range []string{extsync.EnvToken, extsync.EnvUser, extsync.EnvPassword} {
		if name, value := env(suffix); value != "" {
			set = append(set, name)
		}
	}
	if len(set) > 0 {
		return core.SyncOrigin{From: core.SyncFromEnv, Detail: strings.Join(set, ", ")}
	}
	if !syncNeedsCredentials(src.System) {
		return core.SyncOrigin{From: core.SyncFromNone}
	}
	tokenVar, _ := env(extsync.EnvToken)
	*missing = append(*missing, "credentials (store them with "+setUsage+
		" --token-stdin or in the source dialog, or set "+tokenVar+" in the server's environment)")
	return core.SyncOrigin{From: core.SyncFromMissing}
}

// missingSyncSettings names the settings a system's adapter refuses to start
// without.
func missingSyncSettings(src core.SyncSource, settings []core.SyncSettingOrigin) []string {
	have := map[string]core.SyncSettingOrigin{}
	for _, s := range settings {
		have[s.Key] = s
	}
	need := func(key, what string) string {
		return what + " (set it on the source, or set " + have[key].Variable + ")"
	}
	unset := func(key string) bool { return have[key].From == core.SyncFromMissing }
	var out []string
	switch src.System {
	case core.SystemGeneric:
		if unset(core.SyncSettingFile) {
			out = append(out, need(core.SyncSettingFile, "the file to read"))
		}
	case core.SystemJira:
		if unset(core.SyncSettingURL) {
			out = append(out, need(core.SyncSettingURL, "the Jira url"))
		}
		if unset(core.SyncSettingProject) && unset(core.SyncSettingQuery) {
			out = append(out, need(core.SyncSettingProject, "a Jira project or query"))
		}
	case core.SystemOpenProject:
		if unset(core.SyncSettingURL) {
			out = append(out, need(core.SyncSettingURL, "the OpenProject url"))
		}
	}
	return out
}

// refuseUnready stops a run whose source lacks something on the server, naming
// all of it at once rather than the first thing an adapter trips over.
func refuseUnready(src core.SyncSource, resolved core.SyncResolution) error {
	if resolved.Ready() {
		return nil
	}
	return core.Invalid("sync source %q cannot run; sync runs on the server, and it is missing on the server: %s",
		src.Name, strings.Join(resolved.Missing, "; "))
}

// loadSyncMapping reads the mapping a run uses: the stored document, else the
// file its resolution names.
func loadSyncMapping(src core.SyncSource, resolved core.SyncResolution) (*extsync.Mapping, error) {
	switch resolved.Mapping.From {
	case core.SyncFromStored:
		m, err := extsync.ParseMapping([]byte(src.Mapping))
		if err != nil {
			return nil, core.Invalid("the mapping stored on %q: %s", src.Name, messageOf(err))
		}
		return m, nil
	case core.SyncFromPath:
		return extsync.LoadMapping(src.MappingPath)
	default:
		return extsync.LoadMapping(syncEnv(extsync.EnvName(src.Name, extsync.EnvMapping)))
	}
}

// syncConfigFor builds the adapter configuration a run uses, with the stored
// credentials in place of the environment's when the source has any.
func (l *Local) syncConfigFor(tenantID string, src core.SyncSource) (extsync.SourceConfig, error) {
	cfg, err := extsync.LoadSourceConfig(src.Name, extsync.StoredFirst(src, syncEnv))
	if err != nil {
		return cfg, err
	}
	if src.SealedCredentials == "" {
		return cfg, nil
	}
	creds, err := l.openSyncCredentials(tenantID, src)
	if err != nil {
		return cfg, err
	}
	return cfg.WithCredentials(creds.Token, creds.User, creds.Password), nil
}

// checkSyncSource validates a source as it would be saved, against the
// project and workflow its mapping names, and describes it. It writes nothing.
func (l *Local) checkSyncSource(ctx context.Context, actor *core.Actor, src core.SyncSource) (*core.SyncSource, error) {
	resolved := l.resolveSync(src)
	if resolved.Mapping.From != core.SyncFromMissing {
		mapping, err := loadSyncMapping(src, resolved)
		if err != nil {
			return nil, core.OnField(err, "mapping")
		}
		if err := l.read(ctx, actor, func(tx store.Tx) error {
			project, err := lookupProject(ctx, tx, mapping.Project)
			if err != nil {
				return core.OnField(core.Invalid("the mapping names project %q, which this tenant does not have", mapping.Project), "mapping")
			}
			wf, err := tx.GetWorkflowByID(ctx, project.WorkflowID)
			if err != nil {
				return err
			}
			return core.OnField(mapping.ValidateAgainst(wf.Definition), "mapping")
		}); err != nil {
			return nil, err
		}
		resolved.Summary = summarizeMapping(mapping)
	}
	out := auditableSyncSource(src)
	out.Credentials = src.Credentials
	out.Resolved = &resolved
	return &out, nil
}

// summarizeMapping lists what a mapping does, for a reader checking it.
func summarizeMapping(m *extsync.Mapping) *core.SyncMappingSummary {
	out := &core.SyncMappingSummary{Project: m.Project, Identity: m.Identity.ID}
	for ext, target := range m.Fields {
		out.Fields = append(out.Fields, ext+" -> "+target)
	}
	for ext, cf := range m.Custom {
		out.Fields = append(out.Fields, ext+" -> custom field "+cf.Key)
	}
	if m.StatusField != "" {
		out.Fields = append(out.Fields, m.StatusField+" -> status")
	}
	for ext, state := range m.Statuses {
		out.Statuses = append(out.Statuses, ext+" -> "+state)
	}
	if m.DefaultStatus != "" {
		out.Statuses = append(out.Statuses, "anything else -> "+m.DefaultStatus)
	}
	sort.Strings(out.Fields)
	sort.Strings(out.Statuses[:len(m.Statuses)])
	return out
}
