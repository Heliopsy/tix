package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactedJSONRemovesSecrets(t *testing.T) {
	const leak = "hunter2-SHOULD-NOT-APPEAR"

	in := map[string]any{
		"id":            "u1",
		"email":         "a@example.com",
		"password":      leak,
		"password_hash": leak,
		"token_hash":    leak,
		"api_secret":    leak,
		"credentials":   leak,
		"dsn":           leak,
		"nested": map[string]any{
			"webhook_secret": leak,
			"url":            "https://example.com",
		},
		"list": []any{
			map[string]any{"private_key": leak, "name": "k1"},
		},
	}

	got, err := redactedJSON(in)
	if err != nil {
		t.Fatalf("redactedJSON: %v", err)
	}
	if strings.Contains(string(got), leak) {
		t.Fatalf("secret survived redaction:\n%s", got)
	}
	for _, keep := range []string{"a@example.com", "https://example.com", "k1", "u1"} {
		if !strings.Contains(string(got), keep) {
			t.Errorf("redaction removed non-secret %q:\n%s", keep, got)
		}
	}
}

func TestRedactedJSONNilIsNil(t *testing.T) {
	got, err := redactedJSON(nil)
	if err != nil {
		t.Fatalf("redactedJSON(nil): %v", err)
	}
	if got != nil {
		t.Errorf("redactedJSON(nil) = %s, want nil so a create has no before state", got)
	}
}

func TestRedactedJSONPreservesShape(t *testing.T) {
	type task struct {
		ID    string   `json:"id"`
		Title string   `json:"title"`
		Tags  []string `json:"tags"`
	}
	got, err := redactedJSON(task{ID: "t1", Title: "x", Tags: []string{"a", "b"}})
	if err != nil {
		t.Fatalf("redactedJSON: %v", err)
	}

	var back map[string]any
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatalf("result is not valid json: %v", err)
	}
	if back["id"] != "t1" || back["title"] != "x" {
		t.Errorf("shape changed: %s", got)
	}
	if tags, ok := back["tags"].([]any); !ok || len(tags) != 2 {
		t.Errorf("array not preserved: %s", got)
	}
}

func TestIsSecretKey(t *testing.T) {
	for _, k := range []string{
		"password", "Password", "password_hash", "token", "TokenHash",
		"secret", "webhook_secret", "credential", "private_key", "passphrase", "dsn",
	} {
		if !isSecretKey(k) {
			t.Errorf("isSecretKey(%q) = false, want true", k)
		}
	}
	for _, k := range []string{"id", "title", "email", "url", "status", "tenant_id"} {
		if isSecretKey(k) {
			t.Errorf("isSecretKey(%q) = true, want false", k)
		}
	}
}

// Key names alone are not enough: a credential embedded in a url reaches the
// audit log under a key nobody would call secret.
func TestRedactedJSONRemovesCredentialsFromValues(t *testing.T) {
	const leak = "hunter2-SHOULD-NOT-APPEAR"

	in := map[string]any{
		"url":           "https://admin:" + leak + "@hooks.example.com/notify",
		"callback":      "https://hooks.example.com/notify?access_token=" + leak + "&project=infra",
		"header":        "Bearer " + leak + "AAAA",
		"note":          "-----BEGIN RSA PRIVATE KEY-----\n" + leak + "\n-----END RSA PRIVATE KEY-----",
		"authorization": leak,
		"api_key":       leak,
		"cookie":        leak,
		"nested": map[string]any{
			"endpoint": "postgres://tix:" + leak + "@db.internal:5432/tix",
		},
		"list": []any{"amqp://guest:" + leak + "@broker.internal/"},
	}

	got, err := redactedJSON(in)
	if err != nil {
		t.Fatalf("redactedJSON: %v", err)
	}
	if strings.Contains(string(got), leak) {
		t.Fatalf("a credential survived redaction:\n%s", got)
	}
	for _, keep := range []string{"hooks.example.com", "db.internal", "project=infra"} {
		if !strings.Contains(string(got), keep) {
			t.Errorf("redaction removed the non-secret %q:\n%s", keep, got)
		}
	}
}

func TestIsSecretKeyCoversTheWiderSet(t *testing.T) {
	for _, k := range []string{
		"authorization", "Authorization", "cookie", "Set-Cookie", "api_key", "apiKey",
		"access_key", "signing_key", "signature", "salt", "passwd", "bearer_token",
	} {
		if !isSecretKey(k) {
			t.Errorf("isSecretKey(%q) = false, want true", k)
		}
	}
	for _, k := range []string{
		"key", "keys", "new_key", "project_key", "workflow_key", "tenant_key",
		"field_key", "key_path", "id", "title", "url", "status",
	} {
		if isSecretKey(k) {
			t.Errorf("isSecretKey(%q) = true, want false; it names an identifier", k)
		}
	}
}
