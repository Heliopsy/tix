// SPDX-License-Identifier: AGPL-3.0-or-later

package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// A leaked webhook secret lets anyone forge signed deliveries. json:"-" alone
// does not cover YAML, which is how this leaked once already.
func TestNoSecretLeaksInAnyFormat(t *testing.T) {
	const secret = "SUPER-SECRET-VALUE"

	values := []any{
		core.WebhookEndpoint{ID: "w1", URL: "https://example.com", Secret: secret},
		[]core.WebhookEndpoint{{ID: "w1", URL: "https://example.com", Secret: secret}},
		&core.WebhookEndpoint{ID: "w1", URL: "https://example.com", Secret: secret},
	}

	for _, format := range output.Formats {
		for _, v := range values {
			var buf bytes.Buffer
			if err := output.New(format).Format(&buf, v); err != nil {
				t.Fatalf("%s: %v", format, err)
			}
			if strings.Contains(buf.String(), secret) {
				t.Errorf("%s leaked the webhook secret:\n%s", format, buf.String())
			}
		}
	}
}

// json and yaml must name fields identically, so a script reading one can read
// the other and documentation does not have to describe two shapes.
func TestYAMLAndJSONAgreeOnFieldNames(t *testing.T) {
	task := core.Task{
		ID: "t1", TenantID: "tn1", ProjectID: "p1", Title: "x", Status: "todo",
		CustomFields: map[string]any{"team": "infra"},
	}

	var j, y bytes.Buffer
	if err := output.New(output.FormatJSON).Format(&j, task); err != nil {
		t.Fatalf("json: %v", err)
	}
	if err := output.New(output.FormatYAML).Format(&y, task); err != nil {
		t.Fatalf("yaml: %v", err)
	}

	for _, key := range []string{"tenant_id", "project_id", "created_at", "custom_fields"} {
		if !strings.Contains(j.String(), key) {
			t.Errorf("json output is missing %q:\n%s", key, j.String())
		}
		if !strings.Contains(y.String(), key) {
			t.Errorf("yaml output is missing %q:\n%s", key, y.String())
		}
	}
}
