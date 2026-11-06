// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"strings"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sshd"
	"github.com/heliopsy/tix/internal/web"
	"github.com/heliopsy/tix/internal/wire"
)

// latestVia returns the channel of the newest audit entry for an action.
func (h *matrixHarness) latestVia(t *testing.T, action string) core.Via {
	t.Helper()
	entries, _, err := h.local.ListAudit(h.adminCtx, core.AuditFilter{
		Actions: []string{action},
		Page:    core.Page{Sort: "seq", Direction: core.Descending, Limit: 1},
	})
	if err != nil {
		t.Fatalf("listing audit: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("no %s entry was recorded", action)
	}
	return entries[0].Via
}

// latestEventVia returns the channel of the newest event of a type.
func (h *matrixHarness) latestEventVia(t *testing.T, typ core.EventType) core.Via {
	t.Helper()
	db, err := sql.Open("sqlite", h.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var kind string
	var detail sql.NullString
	if err := db.QueryRow(`SELECT via_kind, via_detail FROM events WHERE type = ? ORDER BY seq DESC LIMIT 1`,
		string(typ)).Scan(&kind, &detail); err != nil {
		t.Fatalf("reading the latest %s event: %v", typ, err)
	}
	v := core.Via{}
	if detail.Valid {
		if err := json.Unmarshal([]byte(detail.String), &v); err != nil {
			t.Fatal(err)
		}
	}
	v.Kind = core.ViaKind(kind)
	return v
}

// storedText is every audit and event row as the database holds it, for
// asserting a secret never reached either table.
func (h *matrixHarness) storedText(t *testing.T) string {
	t.Helper()
	db, err := sql.Open("sqlite", h.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var out strings.Builder
	for _, table := range []string{"audit_entries", "events"} {
		rows, err := db.Query("SELECT * FROM " + table) // #nosec G202 -- a fixed table name
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]sql.NullString, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			for _, v := range vals {
				out.WriteString(v.String)
				out.WriteByte('\n')
			}
		}
		_ = rows.Close()
	}
	return out.String()
}

// token mints a named token for the admin.
func (h *matrixHarness) token(t *testing.T, name string) (id, secret string) {
	t.Helper()
	issued, err := h.local.CreateToken(h.adminCtx, core.CreateTokenInput{
		Name: name, ActorID: h.adminActor.ID, Scopes: []core.Scope{core.ScopeAll},
	})
	if err != nil {
		t.Fatalf("minting %s: %v", name, err)
	}
	return issued.ID, issued.Token
}

// postProject creates a project over the API with the headers given.
func (h *matrixHarness) postProject(t *testing.T, key, bearer string, header http.Header) {
	t.Helper()
	body, _ := json.Marshal(core.CreateProjectInput{Key: key, Name: key})
	req, err := http.NewRequest(http.MethodPost, h.baseURL+wire.RouteProjects, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header = header.Clone()
	if req.Header == nil {
		req.Header = http.Header{}
	}
	req.Header.Set(wire.HeaderContentType, wire.ContentJSON)
	req.Header.Set(wire.HeaderAuth, "Bearer "+bearer)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
		t.Fatalf("creating %s = %d: %s", key, res.StatusCode, readAll(t, res))
	}
	_ = res.Body.Close()
}

// A bearer token is the API. The client hint narrows that to the CLI or MCP
// and to nothing else: a header claiming the browser, SSH or the system is
// ignored, and the token is recorded by name and ID, never by its secret.
func TestATokenRequestRecordsItsChannelAndCannotClaimAnother(t *testing.T) {
	h := newMatrixHarness(t)
	id, secret := h.token(t, "ci-agent")
	for i, tc := range []struct {
		hint string
		want core.ViaKind
	}{
		{"", core.ViaAPI},
		{"cli", core.ViaCLIRemote},
		{"mcp", core.ViaMCP},
		{"ssh", core.ViaAPI},
		{"web", core.ViaAPI},
		{"system", core.ViaAPI},
		{"cli-local", core.ViaAPI},
	} {
		t.Run("hint "+tc.hint, func(t *testing.T) {
			header := http.Header{"User-Agent": {"curl/8.0 " + strings.Repeat("x", 300)}}
			if tc.hint != "" {
				header.Set(wire.HeaderClient, tc.hint)
			}
			h.postProject(t, "tok"+string(rune('a'+i)), secret, header)
			got := h.latestVia(t, "project.create")
			if got.Kind != tc.want {
				t.Errorf("recorded %q, want %q", got.Kind, tc.want)
			}
			if got.TokenID != id || got.TokenName != "ci-agent" {
				t.Errorf("recorded token %q/%q, want %q/ci-agent", got.TokenID, got.TokenName, id)
			}
			if got.ClientAddr != "127.0.0.1" || !strings.HasPrefix(got.UserAgent, "curl/8.0") ||
				len(got.UserAgent) > core.MaxViaUserAgent {
				t.Errorf("recorded address %q and agent of %d bytes", got.ClientAddr, len(got.UserAgent))
			}
			if ev := h.latestEventVia(t, core.EventProjectCreated); ev.Kind != tc.want ||
				ev.TokenName != "ci-agent" || ev.ClientAddr != "" || ev.UserAgent != "" {
				t.Errorf("the event recorded %+v, want the kind and token and no network detail", ev)
			}
		})
	}
	if strings.Contains(h.storedText(t), secret) {
		t.Error("the token secret reached the audit or event tables")
	}
}

// A browser session is the web, whatever header it sends, and it is recorded
// by an opaque reference rather than the cookie.
func TestAWebFormRecordsTheSessionNotTheCookie(t *testing.T) {
	h := newMatrixHarness(t)
	const password = "correct-horse-battery-staple"
	if _, err := h.local.CreateUser(h.adminCtx, core.CreateUserInput{
		Email: "browser@example.test", Handle: "browser", Password: password, Role: core.RoleAdmin,
	}); err != nil {
		t.Fatal(err)
	}
	browser := signIn(t, h.baseURL, "browser@example.test", password)
	if _, err := h.local.CreateProject(h.adminCtx, core.CreateProjectInput{Key: "webp", Name: "webp"}); err != nil {
		t.Fatal(err)
	}

	page, err := browser.Get(h.baseURL + web.RouteTasks)
	if err != nil {
		t.Fatal(err)
	}
	csrf := betweenQuotes(readAll(t, page), `name="`+web.CSRFFieldName+`" value="`)
	form := url.Values{web.CSRFFieldName: {csrf}, "quick": {"1"}, "title": {"filed from the browser"}, "project_ref": {"webp"}}
	req, err := http.NewRequest(http.MethodPost, h.baseURL+web.RouteTasks, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(wire.HeaderContentType, "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (test)")
	req.Header.Set(wire.HeaderClient, "cli")
	res, err := browser.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("posting the task = %d: %s", res.StatusCode, readAll(t, res))
	}
	_ = res.Body.Close()

	got := h.latestVia(t, "task.create")
	if got.Kind != core.ViaWeb {
		t.Fatalf("a browser form recorded %q, want web", got.Kind)
	}
	if !strings.HasPrefix(got.Session, "s-") || len(got.Session) != 18 {
		t.Errorf("session reference = %q, want s- and 16 hex characters", got.Session)
	}
	if got.ClientAddr != "127.0.0.1" || got.UserAgent != "Mozilla/5.0 (test)" || got.TokenID != "" {
		t.Errorf("recorded %+v", got)
	}

	base, _ := url.Parse(h.baseURL)
	stored := h.storedText(t)
	for _, c := range browser.Jar.Cookies(base) {
		if c.Name == wire.SessionCookieName && strings.Contains(stored, c.Value) {
			t.Error("the session cookie reached the audit or event tables")
		}
	}
	signedIn := h.latestVia(t, "user.login")
	if signedIn.Kind != core.ViaWeb || signedIn.Session != "" {
		t.Errorf("the sign-in itself recorded %+v, want web before any session existed", signedIn)
	}
}

// The command line through a server is cli-remote, because connect sends the
// hint; the same token without it would read as the API.
func TestTheRemoteCommandLineRecordsItsChannel(t *testing.T) {
	h := newMatrixHarness(t)
	_, secret := h.token(t, "laptop")
	conn, err := connect.Dial(context.Background(), &config.Resolved{},
		connect.Overrides{Server: h.baseURL, Token: secret})
	if err != nil {
		t.Fatalf("dialling the server: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := conn.Service.CreateProject(conn.Context(context.Background()),
		core.CreateProjectInput{Key: "remote", Name: "remote"}); err != nil {
		t.Fatal(err)
	}
	got := h.latestVia(t, "project.create")
	if got.Kind != core.ViaCLIRemote || got.TokenName != "laptop" || got.UserAgent != "tix-client" {
		t.Errorf("the remote CLI recorded %+v, want cli-remote on the laptop token", got)
	}
}

// The command line against the database is cli-local, with the host and the
// operating-system user only when the configuration asks for them.
func TestTheLocalCommandLineRecordsItsChannel(t *testing.T) {
	h := newMatrixHarness(t)
	for _, record := range []bool{false, true} {
		cfg := &config.Resolved{}
		cfg.Config.CLI.RecordHost = record
		conn, err := connect.Dial(context.Background(), cfg, connect.Overrides{DB: h.dbPath})
		if err != nil {
			t.Fatalf("dialling the database: %v", err)
		}
		key := "local" + map[bool]string{false: "a", true: "b"}[record]
		if _, err := conn.Service.CreateProject(conn.Context(context.Background()),
			core.CreateProjectInput{Key: key, Name: key}); err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
		got := h.latestVia(t, "project.create")
		if got.Kind != core.ViaCLILocal {
			t.Fatalf("the local CLI recorded %q, want cli-local", got.Kind)
		}
		host, _ := os.Hostname()
		me, _ := user.Current()
		switch {
		case !record && (got.Host != "" || got.OSUser != ""):
			t.Errorf("host details were recorded with cli.record_host off: %+v", got)
		case record && (got.Host != host || got.OSUser != me.Username):
			t.Errorf("recorded %q/%q with cli.record_host on, want %q/%q", got.Host, got.OSUser, host, me.Username)
		}
	}
}

// An import's rows are the import's, started from whichever channel ran it.
func TestAnImportRecordsTheImportChannel(t *testing.T) {
	h := newMatrixHarness(t)
	conn, err := connect.Dial(context.Background(), &config.Resolved{}, connect.Overrides{DB: h.dbPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx := conn.Context(context.Background())
	if _, err := conn.Service.CreateProject(ctx, core.CreateProjectInput{Key: "imp", Name: "imp"}); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Service.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "imp", Title: "carried"}); err != nil {
		t.Fatal(err)
	}
	var snapshot bytes.Buffer
	if err := conn.Service.ExportTo(ctx, core.ExportInput{}, &snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Service.ImportFrom(ctx, &snapshot, core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatal(err)
	}
	got := h.latestVia(t, "import")
	if got.Kind != core.ViaImport || got.Origin != core.ViaCLILocal {
		t.Errorf("the import recorded %+v, want import started from cli-local", got)
	}
}

// A terminal session over SSH records the key it signed in with and the
// address it came from, driven through a real listener and a real keystroke.
func TestAnSSHSessionRecordsItsKeyAndAddress(t *testing.T) {
	h := newSSHHarness(t, false)
	owner := &core.Actor{
		ID: h.actorID, TenantID: h.tenantID, Kind: core.ActorUser,
		Handle: h.actorHandle, Role: core.RoleAdmin, Scopes: []core.Scope{core.ScopeAll},
	}
	ctx := core.WithVia(core.WithActor(context.Background(), owner), core.Via{Kind: core.ViaCLILocal})
	if _, err := h.svc.CreateProject(ctx, core.CreateProjectInput{Key: "sshp", Name: "sshp"}); err != nil {
		t.Fatal(err)
	}
	task, err := h.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "sshp", Title: "claim me over ssh"})
	if err != nil {
		t.Fatal(err)
	}
	signer := newSigner(t)
	h.enrol(signer.PublicKey())
	client := dialSSH(t, h.addr, signer, sshd.NeutralUser)
	term := openTerminal(t, client, "sshp")
	term.waitFor("claim me over ssh")
	if _, err := term.stdin.Write([]byte("c")); err != nil {
		t.Fatal(err)
	}

	var got core.Via
	waitUntil(t, func() bool {
		entries, _, err := h.svc.ListAudit(ctx, core.AuditFilter{SubjectID: task.ID, Actions: []string{"task.claim"}})
		if err != nil || len(entries) == 0 {
			return false
		}
		got = entries[0].Via
		return true
	})
	term.quit()
	if got.Kind != core.ViaSSH {
		t.Fatalf("the claim recorded %q, want ssh", got.Kind)
	}
	if want := gossh.FingerprintSHA256(signer.PublicKey()); got.KeyFingerprint != want {
		t.Errorf("recorded key %q, want %q", got.KeyFingerprint, want)
	}
	if got.ClientAddr != "127.0.0.1" {
		t.Errorf("recorded address %q", got.ClientAddr)
	}
}

// waitUntil polls until done reports true, failing after the SSH wait.
func waitUntil(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(sshWait)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the condition did not hold within %s", sshWait)
}
