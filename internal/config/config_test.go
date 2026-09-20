package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
)

func environ(pairs map[string]string) []string {
	out := make([]string, 0, len(pairs))
	for name, value := range pairs {
		out = append(out, name+"="+value)
	}
	return out
}

func write(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
	return path
}

func mustLoad(t *testing.T, opts Options) *Resolved {
	t.Helper()
	got, err := Load(opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return got
}

func TestEnvNameIsDerivedFromKeyPath(t *testing.T) {
	cases := []struct{ key, want string }{
		{"database.dsn", "TIX_DATABASE_DSN"},
		{"server.url", "TIX_SERVER_URL"},
		{"hooks.mode", "TIX_HOOKS_MODE"},
		{"discovery.enabled", "TIX_DISCOVERY_ENABLED"},
		{"current_context", "TIX_CURRENT_CONTEXT"},
		{"output.format", "TIX_OUTPUT_FORMAT"},
		{"retention.audit", "TIX_RETENTION_AUDIT"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			if got := EnvName(tc.key); got != tc.want {
				t.Fatalf("EnvName(%q) = %q, want %q", tc.key, got, tc.want)
			}
			key, ok := Lookup(tc.key)
			if !ok {
				t.Fatalf("key %q is not registered", tc.key)
			}
			if key.Env != tc.want {
				t.Fatalf("registered env = %q, want %q", key.Env, tc.want)
			}
		})
	}
}

func TestEveryKeyHasOneUniqueEnvVar(t *testing.T) {
	keys := Keys()
	if len(keys) == 0 {
		t.Fatal("no configuration keys registered")
	}
	seen := make(map[string]string, len(keys))
	for _, k := range keys {
		if k.Env != EnvName(k.Path) {
			t.Fatalf("key %q env %q is not mechanically derived", k.Path, k.Env)
		}
		if !strings.HasPrefix(k.Env, EnvPrefix) {
			t.Fatalf("key %q env %q lacks the %s prefix", k.Path, k.Env, EnvPrefix)
		}
		if prev, dup := seen[k.Env]; dup {
			t.Fatalf("keys %q and %q both generate %s", prev, k.Path, k.Env)
		}
		seen[k.Env] = k.Path
	}
	for _, want := range []string{"database.dsn", "server.url", "server.token", "tenant", "project",
		"auth.mode", "hooks.mode", "discovery.enabled", "discovery.filenames",
		"retention.audit", "retention.events", "retention.webhook_deliveries",
		"log.level", "server.listen"} {
		if _, ok := Lookup(want); !ok {
			t.Fatalf("required key %q is missing from the registry", want)
		}
	}
}

func TestPrecedenceChain(t *testing.T) {
	cases := []struct {
		name       string
		flag       bool
		env        bool
		dotenv     bool
		file       bool
		want       string
		wantSource Layer
	}{
		{"flag beats all", true, true, true, true, "t-flag", LayerFlag},
		{"env beats dotenv and file", false, true, true, true, "t-env", LayerEnv},
		{"dotenv beats file", false, false, true, true, "t-dotenv", LayerDotenv},
		{"file beats default", false, false, false, true, "t-file", LayerFile},
		{"default when nothing set", false, false, false, false, DefaultTenant, LayerDefault},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			home := t.TempDir()
			env := map[string]string{"HOME": home}

			if tc.dotenv {
				write(t, filepath.Join(dir, ".env"), "TIX_TENANT=t-dotenv\n")
			}
			if tc.file {
				cfg := write(t, filepath.Join(home, ".config", "tix", "config.yaml"), "tenant: t-file\n")
				env[EnvConfigFile] = cfg
			}
			if tc.env {
				env["TIX_TENANT"] = "t-env"
			}
			opts := Options{Dir: dir, Home: home, Environ: environ(env)}
			if tc.flag {
				opts.Flags = map[string]string{"tenant": "t-flag"}
			}

			got := mustLoad(t, opts)
			if got.Config.Tenant != tc.want {
				t.Fatalf("tenant = %q, want %q", got.Config.Tenant, tc.want)
			}
			if got.Source("tenant") != tc.wantSource {
				t.Fatalf("source = %q, want %q", got.Source("tenant"), tc.wantSource)
			}
		})
	}
}

func TestPartialOverrideLeavesSiblingsIntact(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	cfg := write(t, filepath.Join(home, "config.yaml"), "tenant: from-file\nlog:\n  level: warn\n")
	env := map[string]string{"HOME": home, EnvConfigFile: cfg}

	got := mustLoad(t, Options{
		Dir: dir, Home: home, Environ: environ(env),
		Flags: map[string]string{"output.format": "json"},
	})
	if got.Config.Output.Format != "json" {
		t.Fatalf("output.format = %q, want json", got.Config.Output.Format)
	}
	if got.Config.Tenant != "from-file" {
		t.Fatalf("tenant = %q, want from-file", got.Config.Tenant)
	}
	if got.Config.Log.Level != "warn" {
		t.Fatalf("log.level = %q, want warn", got.Config.Log.Level)
	}
	if got.Config.Server.Listen != DefaultListen {
		t.Fatalf("server.listen = %q, want the default", got.Config.Server.Listen)
	}
}

func TestDotenvDoesNotOverrideRealEnvironment(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	write(t, filepath.Join(dir, ".env"), "TIX_DATABASE_DSN=sqlite://dotenv.db\nTIX_LOG_LEVEL=debug\n")

	env := map[string]string{"HOME": home, "TIX_DATABASE_DSN": "sqlite://exported.db"}
	got := mustLoad(t, Options{Dir: dir, Home: home, Environ: environ(env)})

	if got.Config.Database.DSN != "sqlite://exported.db" {
		t.Fatalf("dsn = %q, want the exported value", got.Config.Database.DSN)
	}
	if got.Source("database.dsn") != LayerEnv {
		t.Fatalf("dsn source = %q, want environment", got.Source("database.dsn"))
	}
	if got.Config.Log.Level != "debug" {
		t.Fatalf("log.level = %q, want debug from dotenv", got.Config.Log.Level)
	}
	if got.Source("log.level") != LayerDotenv {
		t.Fatalf("log.level source = %q, want dotenv", got.Source("log.level"))
	}
}

func TestDotenvDiscovery(t *testing.T) {
	t.Run("parent directory", func(t *testing.T) {
		root := t.TempDir()
		child := filepath.Join(root, "a", "b")
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, ".env"), "TIX_TENANT=parent\n")
		got := mustLoad(t, Options{Dir: child, Home: t.TempDir(), Environ: environ(map[string]string{})})
		if got.Config.Tenant != "parent" {
			t.Fatalf("tenant = %q, want parent", got.Config.Tenant)
		}
		if got.DotenvFile != filepath.Join(root, ".env") {
			t.Fatalf("dotenv file = %q", got.DotenvFile)
		}
	})

	t.Run("nearest wins", func(t *testing.T) {
		root := t.TempDir()
		child := filepath.Join(root, "a")
		write(t, filepath.Join(root, ".env"), "TIX_TENANT=parent\n")
		write(t, filepath.Join(child, ".env"), "TIX_TENANT=child\n")
		got := mustLoad(t, Options{Dir: child, Home: t.TempDir(), Environ: environ(map[string]string{})})
		if got.Config.Tenant != "child" {
			t.Fatalf("tenant = %q, want child", got.Config.Tenant)
		}
	})

	t.Run("none present", func(t *testing.T) {
		dir := t.TempDir()
		got := mustLoad(t, Options{Dir: dir, Home: t.TempDir(), Environ: environ(map[string]string{})})
		if got.DotenvFile != "" {
			t.Fatalf("dotenv file = %q, want empty", got.DotenvFile)
		}
		if got.Config.Tenant != DefaultTenant {
			t.Fatalf("tenant = %q, want the default", got.Config.Tenant)
		}
	})

	t.Run("walk stops at git root", func(t *testing.T) {
		outer := t.TempDir()
		repo := filepath.Join(outer, "repo")
		child := filepath.Join(repo, "pkg")
		if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(outer, ".env"), "TIX_TENANT=above-root\n")
		got := mustLoad(t, Options{Dir: child, Home: t.TempDir(), Environ: environ(map[string]string{})})
		if got.DotenvFile != "" {
			t.Fatalf("dotenv file = %q, want none above the git root", got.DotenvFile)
		}
	})
}

func TestParseDotenv(t *testing.T) {
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, ".env"), strings.Join([]string{
		"# comment",
		"",
		"export TIX_TENANT=exported",
		`TIX_LOG_LEVEL="debug"`,
		"TIX_PROJECT='quoted'",
		"TIX_SERVER_URL=https://example.test # trailing",
		"bare-line-without-equals",
		"=novalue",
	}, "\n"))

	got, err := ParseDotenv(path)
	if err != nil {
		t.Fatalf("ParseDotenv: %v", err)
	}
	want := map[string]string{
		"TIX_TENANT":     "exported",
		"TIX_LOG_LEVEL":  "debug",
		"TIX_PROJECT":    "quoted",
		"TIX_SERVER_URL": "https://example.test",
	}
	for name, value := range want {
		if got[name] != value {
			t.Fatalf("%s = %q, want %q", name, got[name], value)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d entries, want %d: %v", len(got), len(want), got)
	}
	if _, err := ParseDotenv(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected an error for a missing dotenv")
	}
}

func TestConfigFileLocation(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	explicit := write(t, filepath.Join(t.TempDir(), "explicit.yaml"), "tenant: explicit\n")
	xdgFile := write(t, filepath.Join(xdg, RelativeConfigPath), "tenant: xdg\n")
	homeFile := write(t, filepath.Join(home, ".config", RelativeConfigPath), "tenant: home\n")

	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"explicit wins", map[string]string{EnvConfigFile: explicit, "XDG_CONFIG_HOME": xdg}, explicit},
		{"xdg when set", map[string]string{"XDG_CONFIG_HOME": xdg}, xdgFile},
		{"home fallback", map[string]string{}, homeFile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FilePath(tc.env, home)
			if err != nil {
				t.Fatalf("FilePath: %v", err)
			}
			if got != tc.want {
				t.Fatalf("path = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("tilde is expanded", func(t *testing.T) {
		got, err := FilePath(map[string]string{EnvConfigFile: "~/.config/" + RelativeConfigPath}, home)
		if err != nil {
			t.Fatalf("FilePath: %v", err)
		}
		if got != homeFile {
			t.Fatalf("path = %q, want %q", got, homeFile)
		}
	})

	t.Run("missing file is not an error", func(t *testing.T) {
		got, err := FilePath(map[string]string{}, t.TempDir())
		if err != nil {
			t.Fatalf("FilePath: %v", err)
		}
		if got != "" {
			t.Fatalf("path = %q, want empty", got)
		}
	})

	t.Run("explicit path that does not exist fails", func(t *testing.T) {
		_, err := FilePath(map[string]string{EnvConfigFile: filepath.Join(home, "nope.yaml")}, home)
		if !core.IsKind(err, core.KindInvalid) {
			t.Fatalf("err = %v, want an invalid-kind error", err)
		}
		if !strings.Contains(err.Error(), "nope.yaml") {
			t.Fatalf("err = %v, want it to name the missing path", err)
		}
	})
}

func TestNoConfigFileAnywhereStillResolves(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	env := map[string]string{
		"HOME":             home,
		"TIX_DATABASE_DSN": "sqlite://env.db",
		"TIX_TENANT":       "acme",
		"TIX_LOG_LEVEL":    "debug",
	}
	got := mustLoad(t, Options{Dir: dir, Home: home, Environ: environ(env)})
	if got.ConfigFile != "" {
		t.Fatalf("config file = %q, want empty", got.ConfigFile)
	}
	if got.Config.Database.DSN != "sqlite://env.db" || got.Config.Tenant != "acme" || got.Config.Log.Level != "debug" {
		t.Fatalf("config = %+v, want the environment values", got.Config)
	}
}

func TestContextResolution(t *testing.T) {
	const body = `
current_context: work
contexts:
  work:
    server: https://work.example
    token: work-token
    tenant: work-tenant
    project: work-project
  local:
    database: sqlite://local.db
`
	newOpts := func(t *testing.T) Options {
		t.Helper()
		home := t.TempDir()
		cfg := write(t, filepath.Join(home, "config.yaml"), body)
		return Options{
			Dir: t.TempDir(), Home: home,
			Environ: environ(map[string]string{"HOME": home, EnvConfigFile: cfg}),
		}
	}

	t.Run("current context supplies settings", func(t *testing.T) {
		got := mustLoad(t, newOpts(t))
		if got.ContextName != "work" {
			t.Fatalf("context = %q, want work", got.ContextName)
		}
		if got.Config.Server.URL != "https://work.example" || got.Config.Server.Token != "work-token" {
			t.Fatalf("server = %+v", got.Config.Server)
		}
		if got.Config.Project != "work-project" || got.Config.Tenant != "work-tenant" {
			t.Fatalf("identity = %q/%q", got.Config.Tenant, got.Config.Project)
		}
	})

	t.Run("per-invocation override", func(t *testing.T) {
		opts := newOpts(t)
		opts.Context = "local"
		got := mustLoad(t, opts)
		if got.ContextName != "local" {
			t.Fatalf("context = %q, want local", got.ContextName)
		}
		if got.Config.Database.DSN != "sqlite://local.db" {
			t.Fatalf("dsn = %q", got.Config.Database.DSN)
		}
		if got.Config.Server.URL != "" {
			t.Fatalf("server.url = %q, want empty", got.Config.Server.URL)
		}
	})

	t.Run("unknown override is a usage error", func(t *testing.T) {
		opts := newOpts(t)
		opts.Context = "nope"
		_, err := Load(opts)
		if !core.IsKind(err, core.KindInvalid) {
			t.Fatalf("err = %v, want an invalid-kind error", err)
		}
	})

	t.Run("unknown persisted context is not found", func(t *testing.T) {
		home := t.TempDir()
		cfg := write(t, filepath.Join(home, "config.yaml"), "current_context: ghost\n")
		_, err := Load(Options{
			Dir: t.TempDir(), Home: home,
			Environ: environ(map[string]string{"HOME": home, EnvConfigFile: cfg}),
		})
		if !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("err = %v, want a not-found error", err)
		}
	})

	t.Run("flag beats the context", func(t *testing.T) {
		opts := newOpts(t)
		opts.Flags = map[string]string{"tenant": "flag-tenant"}
		got := mustLoad(t, opts)
		if got.Config.Tenant != "flag-tenant" {
			t.Fatalf("tenant = %q, want flag-tenant", got.Config.Tenant)
		}
	})
}

func TestContextRejectsBothEndpoints(t *testing.T) {
	home := t.TempDir()
	cfg := write(t, filepath.Join(home, "config.yaml"), `
current_context: mixed
contexts:
  mixed:
    database: sqlite://x.db
    server: https://x.example
`)
	_, err := Load(Options{
		Dir: t.TempDir(), Home: home,
		Environ: environ(map[string]string{"HOME": home, EnvConfigFile: cfg}),
	})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("err = %v, want an invalid-kind error", err)
	}
	if !strings.Contains(err.Error(), "mixed") {
		t.Fatalf("err = %v, want it to name the offending context", err)
	}
}

func TestContextValidate(t *testing.T) {
	cases := []struct {
		name string
		ctx  Context
		ok   bool
	}{
		{"database only", Context{Database: "sqlite://a.db"}, true},
		{"server only", Context{Server: "https://a.example"}, true},
		{"neither", Context{Tenant: "t"}, true},
		{"both", Context{Database: "sqlite://a.db", Server: "https://a.example"}, false},
		{"bad auth mode", Context{Server: "https://a", AuthMode: "magic"}, false},
		{"bad hook mode", Context{Server: "https://a", HookMode: "always"}, false},
		{"good modes", Context{Server: "https://a", AuthMode: "token", HookMode: "off"}, true},
		{"unimplemented auth mode", Context{Server: "https://a", AuthMode: "oidc"}, false},
		{"unimplemented hook mode", Context{Server: "https://a", HookMode: "warn"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ctx.validate(tc.name)
			if tc.ok && err != nil {
				t.Fatalf("validate: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
	if !(Context{Server: "https://a"}).Remote() {
		t.Fatal("Remote should be true for a server context")
	}
	if (Context{Database: "sqlite://a.db"}).Remote() {
		t.Fatal("Remote should be false for a database context")
	}
}

func TestPerDirectoryDiscovery(t *testing.T) {
	setup := func(t *testing.T) (string, string, map[string]string) {
		t.Helper()
		outer := t.TempDir()
		repo := filepath.Join(outer, "repo")
		child := filepath.Join(repo, "pkg")
		if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		return outer, child, map[string]string{"HOME": t.TempDir()}
	}

	t.Run("local file in the working directory", func(t *testing.T) {
		_, child, env := setup(t)
		write(t, filepath.Join(child, ".tix.yaml"), "tenant: local\n")
		got := mustLoad(t, Options{Dir: child, Home: env["HOME"], Environ: environ(env)})
		if got.Config.Tenant != "local" {
			t.Fatalf("tenant = %q, want local", got.Config.Tenant)
		}
		if got.LocalFile == "" {
			t.Fatal("expected a discovered local file")
		}
	})

	t.Run("dot directory form", func(t *testing.T) {
		_, child, env := setup(t)
		write(t, filepath.Join(child, ".tix", "config.yaml"), "tenant: dotdir\n")
		got := mustLoad(t, Options{Dir: child, Home: env["HOME"], Environ: environ(env)})
		if got.Config.Tenant != "dotdir" {
			t.Fatalf("tenant = %q, want dotdir", got.Config.Tenant)
		}
	})

	t.Run("nearest file wins", func(t *testing.T) {
		_, child, env := setup(t)
		write(t, filepath.Join(filepath.Dir(child), ".tix.yaml"), "tenant: repo\n")
		write(t, filepath.Join(child, ".tix.yaml"), "tenant: pkg\n")
		got := mustLoad(t, Options{Dir: child, Home: env["HOME"], Environ: environ(env)})
		if got.Config.Tenant != "pkg" {
			t.Fatalf("tenant = %q, want pkg", got.Config.Tenant)
		}
	})

	t.Run("walk stops at the git root", func(t *testing.T) {
		outer, child, env := setup(t)
		write(t, filepath.Join(outer, ".tix.yaml"), "tenant: above\n")
		got := mustLoad(t, Options{Dir: child, Home: env["HOME"], Environ: environ(env)})
		if got.LocalFile != "" {
			t.Fatalf("local file = %q, want none above the git root", got.LocalFile)
		}
		if got.Config.Tenant != DefaultTenant {
			t.Fatalf("tenant = %q, want the default", got.Config.Tenant)
		}
	})

	t.Run("custom filenames", func(t *testing.T) {
		_, child, env := setup(t)
		write(t, filepath.Join(child, ".tix.yaml"), "tenant: standard\n")
		write(t, filepath.Join(child, "team.yaml"), "tenant: custom\n")
		env["TIX_DISCOVERY_FILENAMES"] = "team.yaml"
		got := mustLoad(t, Options{Dir: child, Home: env["HOME"], Environ: environ(env)})
		if got.Config.Tenant != "custom" {
			t.Fatalf("tenant = %q, want custom", got.Config.Tenant)
		}
	})

	t.Run("flag disables discovery", func(t *testing.T) {
		_, child, env := setup(t)
		write(t, filepath.Join(child, ".tix.yaml"), "tenant: local\n")
		got := mustLoad(t, Options{Dir: child, Home: env["HOME"], Environ: environ(env), NoDiscovery: true})
		if got.LocalFile != "" {
			t.Fatalf("local file = %q, want none", got.LocalFile)
		}
		if got.Config.Tenant != DefaultTenant {
			t.Fatalf("tenant = %q, want the default", got.Config.Tenant)
		}
	})

	t.Run("setting disables discovery", func(t *testing.T) {
		_, child, env := setup(t)
		write(t, filepath.Join(child, ".tix.yaml"), "tenant: local\n")
		env["TIX_DISCOVERY_ENABLED"] = "false"
		got := mustLoad(t, Options{Dir: child, Home: env["HOME"], Environ: environ(env)})
		if got.LocalFile != "" {
			t.Fatalf("local file = %q, want none", got.LocalFile)
		}
	})

	t.Run("local file names a context", func(t *testing.T) {
		_, child, env := setup(t)
		write(t, filepath.Join(child, ".tix.yaml"), "current_context: team\ncontexts:\n  team:\n    server: https://team.example\n")
		got := mustLoad(t, Options{Dir: child, Home: env["HOME"], Environ: environ(env)})
		if got.ContextName != "team" || got.Config.Server.URL != "https://team.example" {
			t.Fatalf("context = %q server = %q", got.ContextName, got.Config.Server.URL)
		}
	})
}

func TestSourceAttributionAndRedaction(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	cfg := write(t, filepath.Join(home, "config.yaml"), "log:\n  level: warn\n")
	env := map[string]string{
		"HOME":             home,
		EnvConfigFile:      cfg,
		"TIX_SERVER_TOKEN": "super-secret-token",
		"TIX_DATABASE_DSN": "postgres://user:hunter2@db.example:5432/tix",
		"TIX_LOG_LEVEL":    "error",
	}
	got := mustLoad(t, Options{Dir: dir, Home: home, Environ: environ(env)})

	if got.Source("log.level") != LayerEnv {
		t.Fatalf("log.level source = %q, want environment", got.Source("log.level"))
	}

	entries := got.Sources()
	if len(entries) != len(Keys()) {
		t.Fatalf("got %d entries, want %d", len(entries), len(Keys()))
	}
	byKey := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		if entry.Source == "" {
			t.Fatalf("key %q has no source layer", entry.Key)
		}
		byKey[entry.Key] = entry
	}

	token := byKey["server.token"]
	if token.Value != Redacted {
		t.Fatalf("token value = %q, want %q", token.Value, Redacted)
	}
	if token.Source != LayerEnv || !token.Secret {
		t.Fatalf("token entry = %+v", token)
	}

	dsn := byKey["database.dsn"]
	if strings.Contains(dsn.Value, "hunter2") {
		t.Fatalf("dsn value %q leaks the password", dsn.Value)
	}
	if !strings.Contains(dsn.Value, "db.example") {
		t.Fatalf("dsn value %q lost its host", dsn.Value)
	}

	raw := got.SourcesRaw()
	found := false
	for _, entry := range raw {
		if entry.Key == "server.token" {
			found = entry.Value == "super-secret-token"
		}
	}
	if !found {
		t.Fatal("SourcesRaw should expose the real token")
	}
}

func TestRedactDSN(t *testing.T) {
	cases := []struct {
		name, in string
		leaked   string
	}{
		{"url with password", "postgres://user:hunter2@host/db", "hunter2"},
		{"key value", "host=db user=tix password=hunter2 sslmode=disable", "hunter2"},
		{"token key value", "host=db token=hunter2", "hunter2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactDSN(tc.in)
			if strings.Contains(got, tc.leaked) {
				t.Fatalf("RedactDSN(%q) = %q, still leaks", tc.in, got)
			}
			if !strings.Contains(got, Redacted) {
				t.Fatalf("RedactDSN(%q) = %q, want a redaction marker", tc.in, got)
			}
		})
	}
	if got := RedactDSN("sqlite:///tmp/tix.db"); got != "sqlite:///tmp/tix.db" {
		t.Fatalf("RedactDSN left a harmless dsn as %q", got)
	}
	if redact(secretNone, "") != "" || redact(secretOpaque, "") != "" {
		t.Fatal("empty values must stay empty")
	}
	if redact(secretNone, "plain") != "plain" {
		t.Fatal("non-secret values must pass through")
	}
}

func TestValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		env  map[string]string
		want string
	}{
		{"unknown key in file", "nope: 1\n", nil, "nope"},
		{"unknown nested key", "database:\n  bogus: 1\n", nil, "database.bogus"},
		{"wrong duration type", "retention:\n  audit: not-a-duration\n", nil, "retention.audit"},
		{"bad bool", "discovery:\n  enabled: maybe\n", nil, "discovery.enabled"},
		{"invalid env value", "", map[string]string{"TIX_HOOKS_MODE": "always"}, "hooks.mode"},
		{"negative retention", "", map[string]string{"TIX_RETENTION_EVENTS": "-1h"}, "retention.events"},
		{"negative delivery retention", "", map[string]string{"TIX_RETENTION_WEBHOOK_DELIVERIES": "-1h"}, "retention.webhook_deliveries"},
		{"invalid log level", "", map[string]string{"TIX_LOG_LEVEL": "loud"}, "log.level"},
		{"invalid output format", "", map[string]string{"TIX_OUTPUT_FORMAT": "xml"}, "output.format"},
		{"empty tenant", "", map[string]string{"TIX_TENANT": " "}, "tenant"},
		{"malformed yaml", "tenant: [unclosed\n", nil, "parsing config"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			env := map[string]string{"HOME": home}
			for name, value := range tc.env {
				env[name] = value
			}
			if tc.body != "" {
				env[EnvConfigFile] = write(t, filepath.Join(home, "config.yaml"), tc.body)
			}
			_, err := Load(Options{Dir: t.TempDir(), Home: home, Environ: environ(env)})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("err = %v, want an invalid-kind error", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestUnknownFlagKeyIsRejected(t *testing.T) {
	_, err := Load(Options{
		Dir: t.TempDir(), Home: t.TempDir(), Environ: environ(map[string]string{}),
		Flags: map[string]string{"not.a.key": "x"},
	})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("err = %v, want an invalid-kind error", err)
	}
}

func TestErrorNamesTheLayer(t *testing.T) {
	home := t.TempDir()
	_, err := Load(Options{
		Dir: t.TempDir(), Home: home,
		Environ: environ(map[string]string{"HOME": home, "TIX_RETENTION_AUDIT": "soon"}),
	})
	var domain *core.Error
	if err == nil || !errorsAs(err, &domain) {
		t.Fatalf("err = %v, want a domain error", err)
	}
	if domain.Details["layer"] != string(LayerEnv) {
		t.Fatalf("details = %v, want the environment layer", domain.Details)
	}
}

func TestKeyRoundTrip(t *testing.T) {
	cfg := Defaults()
	cases := []struct{ key, raw, want string }{
		{"tenant", "acme", "acme"},
		{"discovery.enabled", "false", "false"},
		{"discovery.filenames", "a.yaml, b.yaml", "a.yaml,b.yaml"},
		{"retention.audit", "48h", "48h0m0s"},
		{"server.listen", ":9000", ":9000"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			key, ok := Lookup(tc.key)
			if !ok {
				t.Fatalf("key %q not registered", tc.key)
			}
			if err := key.Set(&cfg, tc.raw); err != nil {
				t.Fatalf("Set: %v", err)
			}
			if got := key.Get(&cfg); got != tc.want {
				t.Fatalf("Get = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDefaultsAreValid(t *testing.T) {
	defaults := Defaults()
	if err := Validate(&defaults, map[string]Layer{}); err != nil {
		t.Fatalf("defaults do not validate: %v", err)
	}
	if !defaults.Discovery.Enabled {
		t.Fatal("discovery should default to enabled")
	}
	if !slices.Equal(defaults.Discovery.Filenames, DefaultDiscoveryFilenames) {
		t.Fatalf("filenames = %v", defaults.Discovery.Filenames)
	}
}

func TestHomeIsExpandedInDSN(t *testing.T) {
	home := t.TempDir()
	got := mustLoad(t, Options{Dir: t.TempDir(), Home: home, Environ: environ(map[string]string{"HOME": home})})
	if strings.HasPrefix(got.Config.Database.DSN, "~") {
		t.Fatalf("dsn = %q, want the tilde expanded", got.Config.Database.DSN)
	}
	if !strings.Contains(got.Config.Database.DSN, home) {
		t.Fatalf("dsn = %q, want it under %q", got.Config.Database.DSN, home)
	}
	if expandHome("~", home) != home {
		t.Fatal("a bare tilde should expand to home")
	}
	if expandHome("~user/x", home) != "~user/x" {
		t.Fatal("other users' tildes must be left alone")
	}
	if expandHome("/abs", "") != "/abs" {
		t.Fatal("absolute paths must be left alone")
	}
	if got := expandDSNHome("~/db.sqlite", home); got != filepath.Join(home, "db.sqlite") {
		t.Fatalf("expandDSNHome = %q", got)
	}
	if got := expandDSNHome("postgres://host/db", home); got != "postgres://host/db" {
		t.Fatalf("expandDSNHome = %q", got)
	}
}

func TestSaveIsAtomicAndPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "tix")
	path := filepath.Join(dir, "config.yaml")
	cfg := Defaults()
	cfg.Contexts = map[string]Context{"work": {Server: "https://work.example", Token: "t"}}
	cfg.CurrentContext = "work"

	if err := Save(path, &cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode = %v, want 0700", dirInfo.Mode().Perm())
	}

	cfg.Tenant = "second"
	if err := Save(path, &cfg); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want only the config", len(entries))
	}

	reloaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if reloaded.CurrentContext != "work" {
		t.Fatalf("current_context = %q", reloaded.CurrentContext)
	}
	if reloaded.Contexts["work"].Server != "https://work.example" {
		t.Fatalf("contexts = %v", reloaded.Contexts)
	}
	if reloaded.Values["tenant"] != "second" {
		t.Fatalf("tenant = %q, want second", reloaded.Values["tenant"])
	}
}

func TestSaveRejectsUnwritableDirectory(t *testing.T) {
	base := t.TempDir()
	blocker := write(t, filepath.Join(base, "blocker"), "not a directory\n")
	cfg := Defaults()
	if err := Save(filepath.Join(blocker, "config.yaml"), &cfg); err == nil {
		t.Fatal("expected an error saving under a file")
	}
}

func TestLoadFileRejectsUnknownContextField(t *testing.T) {
	path := write(t, filepath.Join(t.TempDir(), "config.yaml"), "contexts:\n  a:\n    nope: 1\n")
	if _, err := LoadFile(path); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("err = %v, want an invalid-kind error", err)
	}
	if _, err := LoadFile(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestLoadUsesProcessEnvironmentByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("TIX_TENANT", "from-process")
	t.Chdir(t.TempDir())

	got := mustLoad(t, Options{})
	if got.Config.Tenant != "from-process" {
		t.Fatalf("tenant = %q, want from-process", got.Config.Tenant)
	}
}

func errorsAs(err error, target **core.Error) bool {
	return errors.As(err, target)
}

func TestProjectPrecedenceChain(t *testing.T) {
	cases := []struct {
		name       string
		flag       bool
		env        bool
		dotenv     bool
		file       bool
		want       string
		wantSource Layer
	}{
		{"flag beats all", true, true, true, true, "p-flag", LayerFlag},
		{"env beats dotenv and file", false, true, true, true, "p-env", LayerEnv},
		{"dotenv beats file", false, false, true, true, "p-dotenv", LayerDotenv},
		{"file beats default", false, false, false, true, "p-file", LayerFile},
		{"default when nothing set", false, false, false, false, "", LayerDefault},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			home := t.TempDir()
			env := map[string]string{"HOME": home}

			if tc.dotenv {
				write(t, filepath.Join(dir, ".env"), "TIX_PROJECT=p-dotenv\n")
			}
			if tc.file {
				cfg := write(t, filepath.Join(home, ".config", "tix", "config.yaml"), "project: p-file\n")
				env[EnvConfigFile] = cfg
			}
			if tc.env {
				env["TIX_PROJECT"] = "p-env"
			}
			opts := Options{Dir: dir, Home: home, Environ: environ(env)}
			if tc.flag {
				opts.Flags = map[string]string{"project": "p-flag"}
			}

			got := mustLoad(t, opts)
			if got.Config.Project != tc.want {
				t.Fatalf("project = %q, want %q", got.Config.Project, tc.want)
			}
			if got.Source("project") != tc.wantSource {
				t.Fatalf("source = %q, want %q", got.Source("project"), tc.wantSource)
			}
		})
	}
}

func TestProjectFromAContextIsResolved(t *testing.T) {
	const body = `
current_context: work
contexts:
  work:
    database: sqlite://work.db
    project: ctx-project
`
	home := t.TempDir()
	env := map[string]string{"HOME": home}
	env[EnvConfigFile] = write(t, filepath.Join(home, "config.yaml"), body)

	got := mustLoad(t, Options{Dir: t.TempDir(), Home: home, Environ: environ(env)})
	if got.Config.Project != "ctx-project" {
		t.Fatalf("project = %q, want the context value", got.Config.Project)
	}
	if got.Source("project") != LayerFile {
		t.Fatalf("source = %q, want %q", got.Source("project"), LayerFile)
	}
}

func TestUnimplementedEnumValuesAreRefused(t *testing.T) {
	cases := []struct {
		name      string
		env       map[string]string
		key       string
		supported []string
	}{
		{"auth mode oidc", map[string]string{"TIX_AUTH_MODE": "oidc"}, "auth.mode", AuthModes},
		{"auth mode none", map[string]string{"TIX_AUTH_MODE": "none"}, "auth.mode", AuthModes},
		{"hook mode warn", map[string]string{"TIX_HOOKS_MODE": "warn"}, "hooks.mode", HookModes},
		{"hook mode enforce", map[string]string{"TIX_HOOKS_MODE": "enforce"}, "hooks.mode", HookModes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			env := map[string]string{"HOME": home}
			for name, value := range tc.env {
				env[name] = value
			}
			_, err := Load(Options{Dir: t.TempDir(), Home: home, Environ: environ(env)})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("err = %v, want an invalid-kind error", err)
			}
			if !strings.Contains(err.Error(), "not implemented") {
				t.Fatalf("err = %v, want it to say the value is not implemented", err)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("err = %v, want it to name %q", err, tc.key)
			}
			for _, supported := range tc.supported {
				if !strings.Contains(err.Error(), supported) {
					t.Fatalf("err = %v, want it to name the supported value %q", err, supported)
				}
			}
		})
	}
}

func TestUnimplementedContextModesAreRefused(t *testing.T) {
	const body = `
current_context: work
contexts:
  work:
    database: sqlite://work.db
    auth_mode: oidc
`
	home := t.TempDir()
	env := map[string]string{"HOME": home, EnvConfigFile: write(t, filepath.Join(home, "config.yaml"), body)}
	_, err := Load(Options{Dir: t.TempDir(), Home: home, Environ: environ(env)})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("err = %v, want an invalid-kind error", err)
	}
	if !strings.Contains(err.Error(), "not implemented") || !strings.Contains(err.Error(), "token") {
		t.Fatalf("err = %v, want it to refuse the mode and name what is supported", err)
	}
}

func TestRetentionKeysResolveIndependently(t *testing.T) {
	home := t.TempDir()
	cfg := write(t, filepath.Join(home, "config.yaml"), "retention:\n  audit: 100h\n")
	env := map[string]string{
		"HOME":                             home,
		EnvConfigFile:                      cfg,
		"TIX_RETENTION_WEBHOOK_DELIVERIES": "48h",
	}
	got := mustLoad(t, Options{Dir: t.TempDir(), Home: home, Environ: environ(env)})

	cases := []struct {
		key        string
		value      string
		wantSource Layer
	}{
		{"retention.audit", "100h0m0s", LayerFile},
		{"retention.events", mustDuration(DefaultRetentionEvent).String(), LayerDefault},
		{"retention.webhook_deliveries", "48h0m0s", LayerEnv},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			key, ok := Lookup(tc.key)
			if !ok {
				t.Fatalf("key %q is not registered", tc.key)
			}
			if value := key.Get(&got.Config); value != tc.value {
				t.Fatalf("%s = %q, want %q", tc.key, value, tc.value)
			}
			if src := got.Source(tc.key); src != tc.wantSource {
				t.Fatalf("%s source = %q, want %q", tc.key, src, tc.wantSource)
			}
		})
	}
}

func TestRetentionPolicyMirrorsTheConfiguredWindows(t *testing.T) {
	cfg := Defaults()
	policy := cfg.Retention.Policy("acme")
	if policy.TenantID != "acme" {
		t.Fatalf("tenant = %q, want acme", policy.TenantID)
	}
	if policy.AuditEntries != cfg.Retention.Audit ||
		policy.Events != cfg.Retention.Events ||
		policy.WebhookDeliveries != cfg.Retention.WebhookDeliveries {
		t.Fatalf("policy = %+v, want the configured windows %+v", policy, cfg.Retention)
	}
}
