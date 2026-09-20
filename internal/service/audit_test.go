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
