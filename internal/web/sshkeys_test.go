// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// The keys the enrolment screen is exercised with, and the fingerprints an
// ssh client prints for them.
const (
	webKeyAlice         = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKpaQG1fAyPyzoozlbuepYMqx02VTQ9mqBAspmTw/ejG alice@laptop"
	webKeyBob           = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIO7iBRzdBv6R5Wr8RU7z/FEdyBU4QE1TCop8p3ktimOD bob@laptop"
	webFingerprintAlice = "SHA256:NPySjtCQLeCy1E3TC3R8JVnFKaX49G0XduQuNXOluTU"
	webFingerprintBob   = "SHA256:UMq6cSS4w3VPAh2oabxXTOPPFQL6Ok541rvAsUzwBVk"
)

// revokeIDs returns the key identifiers the screen's revoke forms carry.
var revokeIDs = regexp.MustCompile(`name="id" value="([^"]+)"`)

// enrolKey submits the enrolment form the way a browser without JavaScript
// does, and returns the screen that follows.
func (b *browser) enrolKey(key, label string) *http.Response {
	b.t.Helper()
	return b.post("/admin/ssh-keys", url.Values{"public_key": {key}, "label": {label}})
}

func TestSSHKeyScreenEnrolsAndListsWithoutJavaScript(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	empty := b.page("/admin/ssh-keys")
	for _, want := range []string{"SSH keys", "Enrol a key", "No keys.",
		`name="public_key"`, `method="post"`} {
		if !strings.Contains(empty, want) {
			t.Errorf("the empty screen does not contain %q", want)
		}
	}

	resp := b.enrolKey(webKeyAlice, "laptop")
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if location := resp.Header.Get("Location"); !strings.HasPrefix(location, "/admin/ssh-keys") {
		t.Fatalf("location = %q, want the key screen", location)
	}

	page := b.page("/admin/ssh-keys")
	for _, want := range []string{webFingerprintAlice, "laptop", "Active", "Revoke"} {
		if !strings.Contains(page, want) {
			t.Errorf("the screen does not show %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "alice@laptop") {
		t.Error("the screen shows the comment from the submission, which is not stored")
	}
	if strings.Contains(page, "No keys.") {
		t.Error("the screen still reports no keys after one was enrolled")
	}
}

func TestSSHKeyScreenShowsARevokedKeyAsRevoked(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, k := range []struct{ key, label string }{
		{webKeyAlice, "kept"}, {webKeyBob, "gone"},
	} {
		resp := b.enrolKey(k.key, k.label)
		wantStatus(t, resp, http.StatusSeeOther)
		_ = resp.Body.Close()
	}

	page := b.page("/admin/ssh-keys")
	if n := len(revokeIDs.FindAllStringSubmatch(page, -1)); n != 2 {
		t.Fatalf("the screen offers %d revoke forms, want 2:\n%s", n, page)
	}
	for _, want := range []string{webFingerprintAlice, webFingerprintBob} {
		if !strings.Contains(page, want) {
			t.Fatalf("the screen does not show %q:\n%s", want, page)
		}
	}
	goneID := idForLabel(t, page, "gone")
	resp := b.post("/admin/ssh-keys/revoke", url.Values{"id": {goneID}})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()

	after := b.page("/admin/ssh-keys")
	if !strings.Contains(after, "gone") {
		t.Fatalf("the revoked key was dropped from the listing:\n%s", after)
	}
	if !strings.Contains(after, "Revoked") {
		t.Fatalf("the revoked key is not reported as revoked:\n%s", after)
	}
	if !strings.Contains(after, "Active") {
		t.Fatalf("the key that was not revoked is no longer active:\n%s", after)
	}
	if n := len(revokeIDs.FindAllStringSubmatch(after, -1)); n != 1 {
		t.Errorf("the screen offers %d revoke forms, want only the live key's", n)
	}
}

// idForLabel returns the identifier of the row carrying a label.
func idForLabel(t *testing.T, page, label string) string {
	t.Helper()
	rows := strings.Split(page, "<tr>")
	for _, row := range rows {
		if !strings.Contains(row, ">"+label+"<") {
			continue
		}
		if m := revokeIDs.FindStringSubmatch(row); m != nil {
			return m[1]
		}
	}
	t.Fatalf("no revocable row carries the label %q:\n%s", label, page)
	return ""
}

func TestSSHKeyScreenRefusesTextThatIsNotAPublicKey(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.enrolKey("-----BEGIN OPENSSH PRIVATE KEY-----", "oops")
	page := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(page, "that is a private key") {
		t.Errorf("the refusal does not name the mistake:\n%s", page)
	}
	if strings.Contains(page, "BEGIN OPENSSH PRIVATE KEY") {
		t.Error("the refusal echoed the submitted private key back to the page")
	}
	if after := b.page("/admin/ssh-keys"); !strings.Contains(after, "No keys.") {
		t.Errorf("a refused enrolment recorded a key:\n%s", after)
	}
}

// An enrolled key belongs to one tenant: no screen of another tenant may show
// it, and no form of another tenant may reach it.
func TestSSHKeyScreenDoesNotLeakAcrossTenants(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	alice, bob := f.as("alice"), f.as("bob")

	resp := alice.enrolKey(webKeyAlice, "tenant a laptop")
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()

	mine := idForLabel(t, alice.page("/admin/ssh-keys"), "tenant a laptop")

	theirs := bob.page("/admin/ssh-keys")
	for _, forbidden := range []string{mine, "tenant a laptop", webFingerprintAlice,
		f.actorA.ID, f.tenantA.ID} {
		if strings.Contains(theirs, forbidden) {
			t.Errorf("the other tenant's screen discloses %q:\n%s", forbidden, theirs)
		}
	}
	if !strings.Contains(theirs, "No keys.") {
		t.Errorf("the other tenant's screen is not empty:\n%s", theirs)
	}

	refused := bob.post("/admin/ssh-keys/revoke", url.Values{"id": {mine}})
	refusedPage := body(t, refused)
	if refused.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-tenant revocation = %d, want 404", refused.StatusCode)
	}
	if strings.Contains(refusedPage, "tenant a laptop") {
		t.Error("the refusal disclosed the other tenant's key")
	}

	still := alice.page("/admin/ssh-keys")
	if !strings.Contains(still, "Active") || strings.Contains(still, "Revoked") {
		t.Fatalf("a cross-tenant revocation reached the key:\n%s", still)
	}
}
