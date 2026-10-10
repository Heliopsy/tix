// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The field's own help text had promised a generated signing secret "shown
// once, right after you save" while the handler blanked it and no template
// rendered it anywhere. The screen claimed something it did not do.

// webhookSecretBlock returns the one-time secret region of the webhook screen
// and nothing else, so an assertion about how the value is presented reads the
// element that presents it rather than the page it sits on. The page carries
// the word "secret" in its help text and in the form's own label, which is
// exactly how a wider read would pass with the region missing.
func webhookSecretBlock(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<section class="secret"`, "</section>")
}

// saveEndpoint registers an endpoint the way a browser without scripting does.
func saveEndpoint(t *testing.T, b *browser, form url.Values) {
	t.Helper()
	resp := b.post("/admin/webhooks", form)
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
}

// TestAGeneratedWebhookSecretIsShownOnceThenGone is the promise the field-info
// makes, both halves of it.
func TestAGeneratedWebhookSecretIsShownOnceThenGone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	saveEndpoint(t, b, url.Values{
		"url": {"https://hooks.example.test/generated"}, "event_types": {"task.*"}, "active": {"1"}})

	page := b.page("/admin/webhooks")
	secret := webhookSecretBlock(t, page)
	if secret == "" {
		t.Fatalf("a generated signing secret is not shown after saving:\n%s", page)
	}
	value := between(t, secret, `<code id="generated-secret">`, "</code>")
	if value == "" {
		t.Fatalf("the secret region carries no value:\n%s", secret)
	}
	// The same affordances the token screen's secret has, because it is the
	// same partial rather than a second pattern.
	if !strings.Contains(secret, `data-copy-target="generated-secret"`) {
		t.Errorf("the secret has no copy button pointing at it:\n%s", secret)
	}
	if !strings.Contains(secret, `class="copyline"`) {
		t.Errorf("the value is not in a copyline, so nothing wraps it:\n%s", secret)
	}
	if strings.Contains(secret, "flash") {
		t.Errorf("the secret is presented as a transient flash:\n%s", secret)
	}

	// Once. The second read of the same screen must not carry it, and neither
	// must the stored endpoint listing.
	again := b.page("/admin/webhooks")
	if block := webhookSecretBlock(t, again); block != "" {
		t.Errorf("the generated secret is shown again on a later render:\n%s", block)
	}
	if strings.Contains(again, value) {
		t.Errorf("the generated secret value is still on the page after it was shown")
	}
}

// TestASuppliedWebhookSecretIsNotEchoedBack is the other half of the decision:
// tix learned nothing the operator does not already hold, so it shows nothing.
// TestWebhookSecretIsNeverDisplayed covers the stored listing; this covers the
// save that would have put the value in a response.
func TestASuppliedWebhookSecretIsNotEchoedBack(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	saveEndpoint(t, b, url.Values{
		"url": {"https://hooks.example.test/supplied"}, "secret": {"operators-own-value"},
		"event_types": {"task.*"}, "active": {"1"}})

	page := b.page("/admin/webhooks")
	if block := webhookSecretBlock(t, page); block != "" {
		t.Errorf("a secret the operator supplied is presented back to them:\n%s", block)
	}
	if strings.Contains(page, "operators-own-value") {
		t.Errorf("the supplied secret reaches the page")
	}
	if !strings.Contains(page, "hooks.example.test/supplied") {
		t.Errorf("the endpoint was not saved:\n%s", page)
	}
}

// TestTheWebhookSecretHelpTextIsTrueInBothCases reads the field's own help,
// because the defect this change fixes was the help text being a promise
// nothing kept. It reads the field-info beside the secret control, not the
// page: the words appear elsewhere on the screen too.
func TestTheWebhookSecretHelpTextIsTrueInBothCases(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	form := formAt(t, b.page("/admin/webhooks"), "/admin/webhooks")
	_, afterLabel, ok := strings.Cut(form, `<label for="secret">`)
	if !ok {
		t.Fatalf("the register form has no signing secret label:\n%s", form)
	}
	help, _, ok := strings.Cut(afterLabel, "</details>")
	if !ok {
		t.Fatalf("the signing secret label carries no field-info:\n%s", afterLabel)
	}
	for _, want := range []string{"shown once", "Supply your own and nothing is shown"} {
		if !strings.Contains(help, want) {
			t.Errorf("the signing secret help does not say %q:\n%s", want, help)
		}
	}
}
