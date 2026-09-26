// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
	"github.com/spf13/cobra"
)

// shadowed describes one configuration key that a flag on a command also sets.
//
// used reports the value that command would hand to its listener, read off the
// options struct rather than off the configuration it was built from. Reading
// the configuration back would pass while the command threw it away, which is
// exactly how server.listen stayed dead under a green suite.
type shadowed struct {
	key     string
	command string
	flag    string
	used    func(t *testing.T, cfg config.Config) string
}

// shadowedKeys is every key a command flag shadows. A key here must fall back
// to the configured value when its flag is absent.
var shadowedKeys = []shadowed{
	{"server.listen", "serve", "listen", serveWithoutFlags},
	{"ssh.listen", "ssh", "listen", sshField},
	{"ssh.host_key", "ssh", "host-key", sshField},
	{"ssh.allow_public", "ssh", "allow-public", sshField},
	{"ssh.demo", "ssh", "demo", sshField},
	{"ssh.tenant_ttl", "ssh", "tenant-ttl", sshField},
	{"ssh.reap_interval", "ssh", "reap-interval", sshField},
	{"ssh.lease_ttl", "ssh", "lease-ttl", sshField},
	{"ssh.idle_timeout", "ssh", "idle-timeout", sshField},
	{"ssh.keepalive_interval", "ssh", "keepalive-interval", sshField},
	{"ssh.keepalive_max_missed", "ssh", "keepalive-max-missed", sshField},
	{"ssh.max_tenants", "ssh", "max-tenants", sshField},
	{"ssh.max_tasks", "ssh", "max-tasks", sshField},
	{"ssh.rate_per_hour", "ssh", "rate-per-hour", sshField},
	{"ssh.rate_burst", "ssh", "rate-burst", sshField},
	{"ssh.max_sessions_per_key", "ssh", "max-sessions-per-key", sshField},
	{"ssh.max_sessions", "ssh", "max-sessions", sshField},
}

// unshadowedKeys is every remaining key, with the consumer that reads it. A
// key is listed here to record that no flag on its command can win over it
// with a declared default, which is the shape that killed server.listen.
var unshadowedKeys = map[string]string{
	"tenant":                         "connect.Resolve; --tenant only joins the flag layer when non-empty",
	"project":                        "globals.projectFallback; --project only wins when non-empty",
	"current_context":                "config.activeContextName",
	"database.dsn":                   "connect.Resolve; --db only wins when non-empty",
	"database.allow_network_fs":      "connect.dialLocal, ORed with --allow-network-fs",
	"database.connect_timeout":       "connect.openStore",
	"server.url":                     "connect.Resolve; --server only wins when non-empty",
	"server.token":                   "globals.credential, into connect.Overrides.Token",
	"server.trusted_proxies":         "runServe, into server.Options",
	"server.cookie_security":         "runServe, into server.Options",
	"auth.mode":                      "config.Validate only; this build implements one mode",
	"hooks.mode":                     "config.Validate only; this build implements one mode",
	"webhooks.drain_mode":            "connect.drainMode",
	"webhooks.allow_private_targets": "connect.Dial and runServe",
	"discovery.enabled":              "config.discoveryEnabled; --no-discovery is a separate option",
	"discovery.filenames":            "config.discoveryFilenames",
	"retention.audit":                "connect.Dial, via Retention.Policy",
	"retention.events":               "connect.Dial, via Retention.Policy",
	"retention.webhook_deliveries":   "connect.Dial, via Retention.Policy",
	"log.level":                      "globals.logger; the --log-* flags gate on Changed",
	"log.format":                     "globals.logger; the --log-* flags gate on Changed",
	"log.output":                     "globals.logger; the --log-* flags gate on Changed",
	"log.file.max_size_mb":           "globals.logger; the --log-* flags gate on Changed",
	"log.file.max_age":               "globals.logger; the --log-* flags gate on Changed",
	"log.file.max_backups":           "globals.logger; the --log-* flags gate on Changed",
	"log.file.compress":              "globals.logger; the --log-* flags gate on Changed",
	"output.format":                  "globals.formatName; -o only wins when non-empty",
	"output.color":                   "globals.colorMode; --color/--no-color join the flag layer",
	"output.time_format":             "globals.timeStyle",
	"output.timezone":                "globals.timeStyle",
	"tui.keymap":                     "newTUICmd; --keymap only wins when non-empty",
}

// TestEveryConfigKeyIsClassified stops the suite on a new configuration key
// until somebody records which consumer reads it and whether a flag shadows
// it. It is bookkeeping; the behaviour is asserted by the two tests below over
// the same table.
func TestEveryConfigKeyIsClassified(t *testing.T) {
	shadow := map[string]bool{}
	for _, s := range shadowedKeys {
		shadow[s.key] = true
	}
	defined := map[string]bool{}
	for _, k := range config.Keys() {
		defined[k.Path] = true
		_, plain := unshadowedKeys[k.Path]
		switch {
		case shadow[k.Path] && plain:
			t.Errorf("key %q is listed as both shadowed and unshadowed", k.Path)
		case !shadow[k.Path] && !plain:
			t.Errorf("key %q (%s) is in neither table: name the consumer that reads it, "+
				"and add it to shadowedKeys if a flag on its command can win over it", k.Path, k.Env)
		}
	}
	for path := range unshadowedKeys {
		if !defined[path] {
			t.Errorf("unshadowedKeys names %q, which config no longer defines", path)
		}
	}
	for _, s := range shadowedKeys {
		if !defined[s.key] {
			t.Errorf("shadowedKeys names %q, which config no longer defines", s.key)
		}
	}
}

// TestShadowedKeysFallBackToTheFileLayer sets each shadowed key in the lowest
// layer there is, a configuration file, and asserts the value the command
// would use is that one rather than the flag's declared default.
//
// This is the guard server.listen needed: it was set in the file, reported by
// `tix config show --sources`, and then thrown away by the flag.
func TestShadowedKeysFallBackToTheFileLayer(t *testing.T) {
	defaults := config.Defaults()
	for _, s := range shadowedKeys {
		t.Run(s.key, func(t *testing.T) {
			key, ok := config.Lookup(s.key)
			if !ok {
				t.Fatalf("config defines no key %q", s.key)
			}
			assertFlagExists(t, s.command, s.flag)

			want := distinctValue(t, key, &defaults)
			cfg := loadFromFile(t, s.key, want)
			if got := key.Get(&cfg); got != want {
				t.Fatalf("the file layer did not take: %s = %q, want %q", s.key, got, want)
			}
			if got := s.used(t, cfg); got != want {
				t.Errorf("tix %s uses %s = %q with --%s untyped, want the configured %q",
					s.command, s.key, got, s.flag, want)
			}
		})
	}
}

// TestShadowedKeysStillObeyTheirFlag keeps the fallback from turning into the
// opposite defect, a flag that no longer wins.
func TestShadowedKeysStillObeyTheirFlag(t *testing.T) {
	defaults := config.Defaults()
	for _, s := range shadowedKeys {
		t.Run(s.key, func(t *testing.T) {
			key, _ := config.Lookup(s.key)
			typed := distinctValue(t, key, &defaults)
			// The file carries the shipped default, so a probe that ignored
			// the flag would report that default rather than the typed value.
			cfg := loadFromFile(t, s.key, key.Get(&defaults))
			if got := typedFlagValue(t, s, key, typed, cfg); got != typed {
				t.Errorf("tix %s --%s %s used %q, want the flag", s.command, s.flag, typed, got)
			}
		})
	}
}

// typedFlagValue reports the value a command uses once its flag is typed.
func typedFlagValue(t *testing.T, s shadowed, key config.Key, typed string, cfg config.Config) string {
	t.Helper()
	if s.command == "serve" {
		cmd := serveProbe(t)
		o := serveOptions{listen: server.DefaultAddr}
		setFlag(t, cmd, s.flag, typed)
		o.listen = typed
		return o.fromConfig(cmd, cfg.Server).listen
	}
	cmd, o := sshProbe(t)
	setFlag(t, cmd, s.flag, flagLiteral(typed))
	return sshValue(o.fromConfig(cmd, cfg.SSH), key.Path)
}

// serveWithoutFlags reports what tix serve would use with nothing typed. The
// listen field starts on the flag's declared default, which is what a real
// invocation carries into runServe.
func serveWithoutFlags(t *testing.T, cfg config.Config) string {
	t.Helper()
	o := serveOptions{listen: server.DefaultAddr}
	return o.fromConfig(serveProbe(t), cfg.Server).listen
}

// sshField reports what tix ssh would use for one key with nothing typed. The
// key is recovered from the subtest name so one function serves every row.
func sshField(t *testing.T, cfg config.Config) string {
	t.Helper()
	cmd, o := sshProbe(t)
	return sshValue(o.fromConfig(cmd, cfg.SSH), keyUnderTest(t))
}

// keyUnderTest reads the key path off the subtest name, which is the key.
func keyUnderTest(t *testing.T) string {
	t.Helper()
	_, name, ok := strings.Cut(t.Name(), "/")
	if !ok {
		t.Fatalf("test %q is not a per-key subtest", t.Name())
	}
	return name
}

// serveProbe returns the real serve command, whose flag set decides what
// Changed reports.
func serveProbe(t *testing.T) *cobra.Command {
	t.Helper()
	return commandNamed(t, "serve")
}

// sshProbe returns a command carrying the ssh flag set and the options struct
// it writes into. The flags come from registerSSHFlags, the same function the
// real command uses, because a second registration on the real command panics.
func sshProbe(t *testing.T) (*cobra.Command, *sshOptions) {
	t.Helper()
	assertFlagExists(t, "ssh", "listen")
	cmd := &cobra.Command{Use: "ssh"}
	o := &sshOptions{}
	registerSSHFlags(cmd, o)
	return cmd, o
}

// sshValue reads one key's field off the options struct, so the assertion names
// the field the listener is handed rather than the configuration behind it.
func sshValue(o sshOptions, key string) string {
	switch key {
	case "ssh.listen":
		return o.listen
	case "ssh.host_key":
		return o.hostKey
	case "ssh.allow_public":
		return strconv.FormatBool(o.allowPublic)
	case "ssh.demo":
		return strconv.FormatBool(o.demo)
	case "ssh.tenant_ttl":
		return core.Duration(o.tenantTTL).String()
	case "ssh.reap_interval":
		return core.Duration(o.reapInterval).String()
	case "ssh.lease_ttl":
		return core.Duration(o.leaseTTL).String()
	case "ssh.idle_timeout":
		return core.Duration(o.idleTimeout).String()
	case "ssh.keepalive_interval":
		return core.Duration(o.keepaliveInterval).String()
	case "ssh.keepalive_max_missed":
		return strconv.Itoa(o.keepaliveMaxMissed)
	case "ssh.max_tenants":
		return strconv.Itoa(o.maxTenants)
	case "ssh.max_tasks":
		return strconv.Itoa(o.maxTasks)
	case "ssh.rate_per_hour":
		return strconv.Itoa(o.ratePerHour)
	case "ssh.rate_burst":
		return strconv.Itoa(o.rateBurst)
	case "ssh.max_sessions_per_key":
		return strconv.Itoa(o.maxSessionsPerKey)
	case "ssh.max_sessions":
		return strconv.Itoa(o.maxSessions)
	}
	return "no field is mapped for " + key
}

// flagLiteral renders a value in the spelling its pflag accepts: a duration
// key prints days, which time.ParseDuration does not read back.
func flagLiteral(value string) string {
	if d, err := core.ParseDuration(value); err == nil && strings.ContainsAny(value, "smhd") {
		return time.Duration(d).String()
	}
	return value
}

func setFlag(t *testing.T, cmd *cobra.Command, flag, value string) {
	t.Helper()
	if err := cmd.Flags().Set(flag, value); err != nil {
		t.Fatalf("setting --%s to %q: %v", flag, value, err)
	}
}

// distinctValue produces a legal value for a key that differs from the shipped
// default, so a probe returning that default cannot pass by accident.
func distinctValue(t *testing.T, key config.Key, defaults *config.Config) string {
	t.Helper()
	current := key.Get(defaults)
	switch key.Path {
	case "server.listen":
		return "127.0.0.1:18081"
	case "ssh.listen":
		return "127.0.0.1:12222"
	case "ssh.host_key":
		return filepath.Join(t.TempDir(), "host_key")
	}
	switch current {
	case "true":
		return "false"
	case "false":
		return "true"
	}
	if n, err := strconv.Atoi(current); err == nil {
		return strconv.Itoa(n + 7)
	}
	if d, err := core.ParseDuration(current); err == nil {
		return core.Duration(time.Duration(d) + 11*time.Second).String()
	}
	t.Fatalf("no distinct value is known for %s, whose default is %q", key.Path, current)
	return ""
}

// loadFromFile resolves configuration with one key set in a configuration file
// and nothing set in any layer above it.
func loadFromFile(t *testing.T, key, value string) config.Config {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "conf", "tix")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yamlFor(key, value)), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	resolved, err := config.Load(config.Options{
		Dir:         home,
		Home:        home,
		Environ:     []string{"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, "conf")},
		NoDiscovery: true,
	})
	if err != nil {
		t.Fatalf("loading configuration: %v", err)
	}
	if got := resolved.Source(key); got != config.LayerFile {
		t.Fatalf("%s came from the %s layer, want the file layer", key, got)
	}
	return resolved.Config
}

// yamlFor renders one dotted key as the nested document a file carries.
func yamlFor(key, value string) string {
	parts := strings.Split(key, ".")
	out := &strings.Builder{}
	for i, part := range parts[:len(parts)-1] {
		out.WriteString(strings.Repeat("  ", i) + part + ":\n")
	}
	out.WriteString(strings.Repeat("  ", len(parts)-1) + parts[len(parts)-1] + ": " +
		strconv.Quote(value) + "\n")
	return out.String()
}

// commandNamed returns one subcommand off a freshly built root.
func commandNamed(t *testing.T, name string) *cobra.Command {
	t.Helper()
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())
	for _, sub := range root.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	t.Fatalf("the root command has no %s subcommand", name)
	return nil
}

// assertFlagExists keeps the table honest about which flag does the shadowing.
func assertFlagExists(t *testing.T, command, flag string) {
	t.Helper()
	if commandNamed(t, command).Flags().Lookup(flag) == nil {
		t.Fatalf("tix %s has no --%s flag; the shadow table is stale", command, flag)
	}
}

// TestServeBindsTheConfiguredListenAddress is the end-to-end half of the same
// guard: server.listen in a configuration file, no --listen, and the assertion
// is a request answered on that exact address.
//
// Reading the command's own "tix listening on" line would not do, since that
// line prints whatever address the process was handed. The socket is the thing
// a deployment actually gets.
func TestServeBindsTheConfiguredListenAddress(t *testing.T) {
	c := newCLI(t)
	addr := freePort(t)
	writeUserConfig(t, c, "server:\n  listen: "+addr+"\n")

	done := make(chan result, 1)
	go func() { done <- c.run("serve") }()

	if !answers(t, "http://"+addr+"/readyz") {
		// The address the process reports is the diagnostic: a serve that
		// ignored the key says 127.0.0.1:8080 while the key said otherwise.
		reported := stopAndRead(t, done)
		t.Fatalf("nothing answered on the configured %s; serve reported: %s",
			addr, strings.TrimSpace(reported))
	}
	stopServe(t, done)
}

// TestServeBindsTheListenAddressFromTheEnvironment is the layer a container
// actually has. It is a separate test from the file one because the flag
// outranked both, and a deployment reaches the environment, not the file.
func TestServeBindsTheListenAddressFromTheEnvironment(t *testing.T) {
	c := newCLI(t)
	addr := freePort(t)
	done := make(chan result, 1)
	env := append(c.environ(), config.EnvName("server.listen")+"="+addr)
	go func() {
		var out, errb strings.Builder
		code := Run([]string{"serve"}, strings.NewReader(""), &out, &errb, env, c.home)
		done <- result{code: code, out: out.String(), err: errb.String()}
	}()

	if !answers(t, "http://"+addr+"/readyz") {
		reported := stopAndRead(t, done)
		t.Fatalf("nothing answered on the address TIX_SERVER_LISTEN named, %s; serve reported: %s",
			addr, strings.TrimSpace(reported))
	}
	stopServe(t, done)
}

// TestSSHBindsTheConfiguredListenAddress is the same end-to-end assertion for
// the other command that lays configuration under its flags. The per-key table
// above exercises fromConfig; only a listener actually answering on the
// configured address proves runSSH still calls it.
func TestSSHBindsTheConfiguredListenAddress(t *testing.T) {
	c := newCLI(t)
	addr := freePort(t)
	db := filepath.Join(t.TempDir(), "enrolled.db")
	writeUserConfig(t, c, "database:\n  dsn: sqlite://"+db+"\nssh:\n  listen: "+addr+"\n")

	done := make(chan result, 1)
	go func() { done <- c.run("ssh") }()

	if !accepts(t, addr) {
		reported := stopAndRead(t, done)
		t.Fatalf("nothing accepted a connection on the configured %s; ssh reported: %s",
			addr, strings.TrimSpace(reported))
	}
	if got := stopAndRead(t, done); !strings.Contains(got, addr) {
		t.Errorf("ssh reported %q, want the configured %s", strings.TrimSpace(got), addr)
	}
}

// accepts reports whether something accepts a connection on addr before the
// deadline. The assertion is the accepted connection, not a printed line.
func accepts(t *testing.T, addr string) bool {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// TestServerTokenFromConfigurationAuthenticates covers the other half of the
// class, a key nothing read at all. server.token was resolved, reported by
// `tix config show --sources`, and then never presented, so the token a
// context carries authenticated nothing.
func TestServerTokenFromConfigurationAuthenticates(t *testing.T) {
	host := newCLI(t)
	host.mustRun("task", "add", "guarded")
	var created struct {
		Token string `json:"token"`
	}
	out := host.mustRun("token", "create", "guard", "--scope", "task:read", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("token create: %v (%s)", err, out)
	}
	if created.Token == "" {
		t.Fatalf("token create returned no token: %s", out)
	}

	addr := freePort(t)
	done := make(chan result, 1)
	go func() { done <- host.run("serve", "--listen", addr) }()
	waitFor(t, "http://"+addr+"/healthz")

	client := newCLI(t)
	writeUserConfig(t, client, "server:\n  url: http://"+addr+"\n  token: "+created.Token+"\n")
	got := client.run("task", "ls")
	switch {
	case got.code != core.ExitOK:
		t.Errorf("task ls with server.token in the configuration file exited %d: %s",
			got.code, strings.TrimSpace(got.err))
	case !strings.Contains(got.out, "guarded"):
		t.Errorf("task ls listed nothing the server holds: %q", got.out)
	}

	stopServe(t, done)
}

// answers reports whether the url answers 200 before the deadline.
func answers(t *testing.T, url string) bool {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return resp.StatusCode == http.StatusOK
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// stopAndRead interrupts a running command and returns what it printed.
func stopAndRead(t *testing.T, done <-chan result) string {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("signalling: %v", err)
	}
	select {
	case got := <-done:
		return got.out + got.err
	case <-time.After(30 * time.Second):
		return "it did not shut down"
	}
}

// writeUserConfig plants a configuration file in a cli's isolated home.
func writeUserConfig(t *testing.T, c *cli, body string) {
	t.Helper()
	dir := filepath.Join(c.home, "conf", "tix")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("writing the configuration file: %v", err)
	}
}

// stopServe interrupts the running serve command and waits for its exit.
func stopServe(t *testing.T, done <-chan result) {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("signalling serve: %v", err)
	}
	select {
	case got := <-done:
		if got.code != core.ExitOK {
			t.Fatalf("serve exited %d: %s", got.code, got.err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("serve did not shut down")
	}
}
