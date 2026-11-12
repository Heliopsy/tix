// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"
)

func TestTheReleaseScreenOffersCopyDiagnostics(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/release")
	section := between(t, page, `<section class="relsec reldiag" id="diagnostics"`, "</section>")
	for _, want := range []string{
		`<div class="reldiag-text" id="diagnostics-text" data-diagnostics data-version="`,
		`data-copy-target="diagnostics-text"`, "Copy diagnostics", `data-diagnostics-server`,
		"Newest published: ",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("the diagnostics section lacks %q:\n%s", want, section)
		}
	}
	if pre := between(t, section, `<div class="reldiag-text"`, "</div>"); strings.Contains(pre, "127.0.0.1") || strings.Contains(pre, "csrf") {
		t.Errorf("the server-drawn report names the server or a credential:\n%s", pre)
	}
	if !strings.Contains(page, `<script src="/assets/diagnostics.js" defer></script>`) {
		t.Error("the page does not load the script that adds the browser's facts")
	}
}
