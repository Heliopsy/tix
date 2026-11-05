// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func toolFor(t *testing.T, id string) Tool {
	t.Helper()
	for _, tool := range shippedPlan(t, Options{AllowDestructive: true}) {
		if tool.Op.ID == id {
			return tool
		}
	}
	t.Fatalf("no tool for %s", id)
	return Tool{}
}

func TestArgumentsBecomeOneRequest(t *testing.T) {
	for _, tc := range []struct {
		name, op, args    string
		path, query, body string
	}{
		{"path escaped", "task.show", `{"ref":"a b/c"}`, "/api/v1/tasks/a%20b%2Fc", "", ""},
		{"list repeats the key", "task.list", `{"status":["todo","doing"],"limit":5,"root_only":true}`,
			"/api/v1/tasks", "limit=5&root_only=true&status=todo&status=doing", ""},
		{"body at the top level", "task.create", `{"title":"t","priority":2}`, "/api/v1/tasks", "", `{"priority":2,"title":"t"}`},
		{"body nested on a collision", "workflow.put_by_key", `{"key":"k","body":{"key":"k","name":"n"}}`,
			"/api/v1/workflows/k", "", `{"key":"k","name":"n"}`},
		{"null reaches the body as a clear", "task.update", `{"ref":"ops-1","due_at":null}`,
			"/api/v1/tasks/ops-1", "", `{"due_at":null}`},
		{"optional body left out", "claim.next", `{}`, "/api/v1/claims/next", "", ""},
		{"no arguments at all", "tag.list", ``, "/api/v1/tags", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, q, body, err := toolFor(t, tc.op).request(json.RawMessage(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			if path != tc.path || q.Encode() != tc.query || string(body) != tc.body {
				t.Errorf("request = %q ? %q body %q, want %q ? %q body %q", path, q.Encode(), body, tc.path, tc.query, tc.body)
			}
		})
	}
}

func TestArgumentsTheOperationCannotTakeAreRefused(t *testing.T) {
	for _, tc := range []struct{ op, args, want string }{
		{"task.show", `{"ref":"x","title":"y"}`, `no argument "title"`},
		{"task.show", `{}`, "needs every path argument"},
		{"task.list", `{"format":"csv"}`, `no argument "format"`},
		{"task.show", `[1]`, "not a JSON object"},
	} {
		_, _, _, err := toolFor(t, tc.op).request(json.RawMessage(tc.args))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s %s: err = %v, want one naming %s", tc.op, tc.args, err, tc.want)
		}
	}
}

func TestResultsCarryTheResponseAndTheErrorEnvelope(t *testing.T) {
	ok := success([]byte(`{"ref":"infra-1"}`))
	if ok.IsError || ok.StructuredContent.(map[string]any)["ref"] != "infra-1" {
		t.Errorf("success = %+v", ok)
	}
	if empty := success(nil); empty.IsError || empty.StructuredContent != nil {
		t.Errorf("an empty response = %+v", empty)
	}

	fail := failure(&core.Error{Kind: core.KindNotFound, Message: "task infra-9", Details: map[string]any{"ref": "infra-9"}})
	env := fail.StructuredContent.(map[string]any)["error"].(map[string]any)
	if !fail.IsError || env["code"] != core.KindNotFound || env["message"] != "task infra-9" || env["details"] == nil {
		t.Errorf("failure = %+v", env)
	}
	plain := failure(errors.New("dial tcp: refused")).StructuredContent.(map[string]any)["error"].(map[string]any)
	if plain["code"] != core.KindInternal || plain["details"] != nil {
		t.Errorf("a transport failure = %+v", plain)
	}
}

func TestAnnotationsFollowTheMethod(t *testing.T) {
	for _, tc := range []struct {
		op                      string
		read, idempotent, destr bool
	}{
		{"task.list", true, true, false},
		{"task.create", false, false, false},
		{"workflow.put", false, true, false},
		{"task.delete", false, true, true},
		{"task.bulk", false, false, true},
	} {
		ann := toolFor(t, tc.op).descriptor().Annotations
		destr := ann.DestructiveHint != nil && *ann.DestructiveHint
		if ann.ReadOnlyHint != tc.read || ann.IdempotentHint != tc.idempotent || destr != tc.destr {
			t.Errorf("%s annotations = %+v (destructive %v)", tc.op, ann, destr)
		}
	}
}
