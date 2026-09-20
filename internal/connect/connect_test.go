package connect

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/auth"
	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/config"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
	"github.com/thereisnotime/tix/internal/store"
	"github.com/thereisnotime/tix/internal/store/sqlite"
)

// load resolves a configuration rooted at a throwaway home directory.
func load(t *testing.T, home string, environ []string, opts ...func(*config.Options)) *config.Resolved {
	t.Helper()
	o := config.Options{
		Dir:     home,
		Home:    home,
		Environ: append([]string{"HOME=" + home}, environ...),
	}
	for _, fn := range opts {
		fn(&o)
	}
	resolved, err := config.Load(o)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return resolved
}

func TestResolveDefaultsToLocalDatabase(t *testing.T) {
	home := t.TempDir()
	target, err := Resolve(load(t, home, nil), Overrides{Home: home})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if target.Mode != ModeLocal {
		t.Fatalf("mode = %q, want local", target.Mode)
	}
	want := filepath.Join(home, ".local", "share", "tix", "tix.db")
	if target.Path != want {
		t.Fatalf("path = %q, want %q", target.Path, want)
	}
	if target.Origin != OriginDefault {
		t.Fatalf("origin = %q, want %q", target.Origin, OriginDefault)
	}
}

func TestResolveHonoursXDGDataHome(t *testing.T) {
	home := t.TempDir()
	data := filepath.Join(home, "xdg")
	target, err := Resolve(load(t, home, []string{"XDG_DATA_HOME=" + data}),
		Overrides{Home: home, Environ: []string{"XDG_DATA_HOME=" + data}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if want := filepath.Join(data, "tix", "tix.db"); target.Path != want {
		t.Fatalf("path = %q, want %q", target.Path, want)
	}
}

func TestResolveOverrideOrder(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, `
current_context: work
contexts:
  work:
    server: https://work.example
  local:
    database: sqlite://`+filepath.Join(home, "local.db")+`
`)
	cases := []struct {
		name    string
		ctxName string
		ov      Overrides
		mode    Mode
		want    string
		origin  Origin
	}{
		{"db flag wins", "", Overrides{DB: filepath.Join(home, "flag.db")}, ModeLocal, filepath.Join(home, "flag.db"), OriginFlag},
		{"server flag wins", "", Overrides{Server: "https://flag.example"}, ModeRemote, "https://flag.example", OriginFlag},
		{"explicit context", "local", Overrides{}, ModeLocal, filepath.Join(home, "local.db"), OriginContext},
		{"current context", "", Overrides{}, ModeRemote, "https://work.example", OriginContext},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved := load(t, home, nil, func(o *config.Options) {
				o.Context = tc.ctxName
				o.Flags = flagsFor(tc.ov)
			})
			tc.ov.Home = home
			target, err := Resolve(resolved, tc.ov)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if target.Mode != tc.mode {
				t.Fatalf("mode = %q, want %q", target.Mode, tc.mode)
			}
			got := target.Path
			if target.Mode == ModeRemote {
				got = target.URL
			}
			if got != tc.want {
				t.Fatalf("target = %q, want %q", got, tc.want)
			}
			if target.Origin != tc.origin {
				t.Fatalf("origin = %q, want %q", target.Origin, tc.origin)
			}
		})
	}
}

func TestResolveRejectsBothOverrides(t *testing.T) {
	home := t.TempDir()
	_, err := Resolve(load(t, home, nil), Overrides{DB: "a.db", Server: "https://x", Home: home})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("err = %v, want invalid", err)
	}
}

func TestResolveRejectsUnsupportedScheme(t *testing.T) {
	home := t.TempDir()
	_, err := Resolve(load(t, home, nil), Overrides{DB: "mysql://host/db", Home: home})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("err = %v, want invalid", err)
	}
}

func TestOpenCreatesAndMigratesOnFirstUse(t *testing.T) {
	home := t.TempDir()
	ctx := context.Background()
	conn, err := Dial(ctx, load(t, home, nil), Overrides{Home: home})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	path := filepath.Join(home, ".local", "share", "tix", "tix.db")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database not created: %v", err)
	}
	if conn.Info.SchemaVersion <= 0 {
		t.Fatalf("schema version = %d, want positive", conn.Info.SchemaVersion)
	}
	if conn.Actor == nil || conn.Actor.ID == "" {
		t.Fatal("no synthesized actor")
	}
	if !conn.Actor.HasScope(core.ScopeAll) {
		t.Fatal("synthesized actor lacks scopes")
	}

	task, err := conn.Service.CreateTask(conn.Context(ctx), core.CreateTaskInput{Title: "buy milk"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if !strings.HasPrefix(task.Ref, "default-") {
		t.Fatalf("ref = %q, want default-N", task.Ref)
	}
}

func TestOpenReusesTheSameActorAcrossInvocations(t *testing.T) {
	home := t.TempDir()
	ctx := context.Background()
	first, err := Dial(ctx, load(t, home, nil), Overrides{Home: home})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	id := first.Actor.ID
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := Dial(ctx, load(t, home, nil), Overrides{Home: home})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = second.Close() }()
	if second.Actor.ID != id {
		t.Fatalf("actor id = %q, want %q", second.Actor.ID, id)
	}
}

func TestOpenRejectsAnInvalidToken(t *testing.T) {
	home := t.TempDir()
	_, err := Dial(context.Background(), load(t, home, nil), Overrides{Home: home, Token: "tix_pat_nonsense"})
	if !core.IsKind(err, core.KindUnauthenticated) && !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("err = %v, want unauthenticated or invalid", err)
	}
}

func TestOpenAcceptsAMintedToken(t *testing.T) {
	home := t.TempDir()
	ctx := context.Background()
	conn, err := Dial(ctx, load(t, home, nil), Overrides{Home: home})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	actorID, tenantID := conn.Actor.ID, conn.Info.TenantID
	path := conn.Info.Target.Path
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	value := mintToken(t, path, tenantID, actorID)
	second, err := Dial(ctx, load(t, home, nil), Overrides{Home: home, Token: value})
	if err != nil {
		t.Fatalf("Dial with token: %v", err)
	}
	defer func() { _ = second.Close() }()
	if second.Actor.ID != actorID {
		t.Fatalf("actor = %q, want %q", second.Actor.ID, actorID)
	}
	if second.Actor.TokenID == "" {
		t.Fatal("actor carries no token id")
	}
}

// mintToken writes an API token straight into the store and returns its value.
func mintToken(t *testing.T, path, tenantID, actorID string) string {
	t.Helper()
	clk := clock.New()
	st, err := sqlite.Open(path, clk)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = st.Close() }()

	minted, err := auth.MintAPIToken(clk, tenantID, core.CreateTokenInput{
		Name:    "agent",
		ActorID: actorID,
		Scopes:  []core.Scope{core.ScopeAll},
	})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	minted.Issued.ActorID = actorID
	if err := st.Update(context.Background(), core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		return tx.CreateToken(context.Background(), &minted.Issued.APIToken, minted.Hash)
	}); err != nil {
		t.Fatalf("store token: %v", err)
	}
	return minted.Issued.Token
}

func TestOpenRemoteUsesTheHTTPClient(t *testing.T) {
	home := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != httpapi.RouteWhoAmI {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"actor-1","tenant_id":"tenant-1","kind":"agent","handle":"remote"}`)
	}))
	defer srv.Close()

	conn, err := Dial(context.Background(), load(t, home, nil), Overrides{Home: home, Server: srv.URL})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if conn.Info.Target.Mode != ModeRemote {
		t.Fatalf("mode = %q, want remote", conn.Info.Target.Mode)
	}
	if conn.Actor == nil || conn.Actor.Handle != "remote" {
		t.Fatalf("actor = %+v", conn.Actor)
	}
}

func TestOpenRemoteReportsAnUnreachableServer(t *testing.T) {
	home := t.TempDir()
	_, err := Open(context.Background(), load(t, home, nil),
		Overrides{Home: home, Server: "http://127.0.0.1:1"})
	if err == nil {
		t.Fatal("dial to a closed port succeeded")
	}
}

func TestConfigWritePathPrefersXDG(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "conf")
	got := ConfigWritePath([]string{"XDG_CONFIG_HOME=" + xdg}, home)
	if want := filepath.Join(xdg, config.RelativeConfigPath); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	// An explicitly empty environment, not nil: nil means "read the real one",
	// and a runner with XDG_CONFIG_HOME set would otherwise decide this test.
	if got := ConfigWritePath([]string{}, home); got != filepath.Join(home, ".config", config.RelativeConfigPath) {
		t.Fatalf("fallback path = %q", got)
	}
}

func writeConfig(t *testing.T, home, body string) {
	t.Helper()
	path := filepath.Join(home, ".config", config.RelativeConfigPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func flagsFor(ov Overrides) map[string]string {
	out := map[string]string{}
	if ov.DB != "" {
		out["database.dsn"] = ov.DB
	}
	if ov.Server != "" {
		out["server.url"] = ov.Server
	}
	return out
}

func TestTargetDescribe(t *testing.T) {
	local := Target{Mode: ModeLocal, Path: "/tmp/tix.db", Origin: OriginDefault}
	if got := local.Describe(); got != "local /tmp/tix.db (from default)" {
		t.Fatalf("describe = %q", got)
	}
	remote := Target{Mode: ModeRemote, URL: "https://tix.example", Origin: OriginFlag}
	if got := remote.Describe(); got != "remote https://tix.example (from flag)" {
		t.Fatalf("describe = %q", got)
	}
}
