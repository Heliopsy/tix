package sync

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func fakeEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestEnvNameSlugsTheSourceName(t *testing.T) {
	if got := EnvName("jira prod-1", EnvToken); got != "TIX_SYNC_JIRA_PROD_1_TOKEN" {
		t.Errorf("EnvName() = %q", got)
	}
}

func TestLoadSourceConfigReadsTheEnvironment(t *testing.T) {
	env := fakeEnv(map[string]string{
		"TIX_SYNC_OPS_MAPPING":   "/etc/tix/ops.yaml",
		"TIX_SYNC_OPS_FILE":      "/tmp/ops.csv",
		"TIX_SYNC_OPS_URL":       "https://jira.test/",
		"TIX_SYNC_OPS_TOKEN":     "s3cr3t",
		"TIX_SYNC_OPS_USER":      "svc",
		"TIX_SYNC_OPS_PASSWORD":  "hunter2",
		"TIX_SYNC_OPS_QUERY":     "project = OPS",
		"TIX_SYNC_OPS_PROJECT":   "OPS",
		"TIX_SYNC_OPS_PAGE_SIZE": "25",
	})
	cfg, err := LoadSourceConfig("ops", env)
	if err != nil {
		t.Fatalf("LoadSourceConfig: %v", err)
	}
	if cfg.BaseURL != "https://jira.test" {
		t.Errorf("base url = %q, want the trailing slash trimmed", cfg.BaseURL)
	}
	if cfg.PageSize != 25 || cfg.MappingPath != "/etc/tix/ops.yaml" || cfg.File != "/tmp/ops.csv" {
		t.Errorf("config = %+v", cfg.Redacted())
	}
	if cfg.Token() != "s3cr3t" {
		t.Errorf("token = %q", cfg.Token())
	}
	user, password := cfg.BasicAuth()
	if user != "svc" || password != "hunter2" {
		t.Errorf("basic auth = %q / %q", user, password)
	}
	if !cfg.HasCredential() {
		t.Error("HasCredential() = false with a token configured")
	}
}

func TestLoadSourceConfigRejectsABadPageSize(t *testing.T) {
	for _, raw := range []string{"0", "-1", "many"} {
		_, err := LoadSourceConfig("ops", fakeEnv(map[string]string{"TIX_SYNC_OPS_PAGE_SIZE": raw}))
		if err == nil {
			t.Errorf("LoadSourceConfig accepted page size %q", raw)
		}
	}
	if _, err := LoadSourceConfig("ops", nil); err == nil {
		t.Error("LoadSourceConfig accepted a nil environment")
	}
}

// A credential that reaches a log line, an error, a snapshot or an event is a
// disclosure, so every rendering of the configuration must omit it.
func TestSourceConfigNeverRendersACredential(t *testing.T) {
	cfg, err := LoadSourceConfig("ops", fakeEnv(map[string]string{
		"TIX_SYNC_OPS_URL":      "https://jira.test",
		"TIX_SYNC_OPS_TOKEN":    "s3cr3t",
		"TIX_SYNC_OPS_PASSWORD": "hunter2",
		"TIX_SYNC_OPS_USER":     "svc",
	}))
	if err != nil {
		t.Fatalf("LoadSourceConfig: %v", err)
	}

	asJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	asYAML, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshalling yaml: %v", err)
	}
	renderings := []string{cfg.String(), string(asJSON), string(asYAML)}
	for _, rendering := range renderings {
		for _, secret := range []string{"s3cr3t", "hunter2", "svc"} {
			if strings.Contains(rendering, secret) {
				t.Errorf("rendering %q disclosed %q", rendering, secret)
			}
		}
		if !strings.Contains(rendering, "redacted") {
			t.Errorf("rendering %q does not mark the credential as redacted", rendering)
		}
	}

	empty, err := LoadSourceConfig("ops", fakeEnv(nil))
	if err != nil {
		t.Fatalf("LoadSourceConfig: %v", err)
	}
	if empty.Redacted()["credential"] != "unset" {
		t.Errorf("Redacted() = %v, want an unset credential", empty.Redacted())
	}
}

func TestWithCredentialsReplacesThemOutOfBand(t *testing.T) {
	cfg := SourceConfig{BaseURL: "https://x.test"}.WithCredentials("t", "u", "p")
	if cfg.Token() != "t" {
		t.Errorf("token = %q", cfg.Token())
	}
	if strings.Contains(cfg.String(), "t") && strings.Contains(cfg.String(), "token=") {
		t.Errorf("String() disclosed the token: %s", cfg.String())
	}
}
