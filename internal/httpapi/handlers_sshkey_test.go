// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// The keys the enrolment routes are exercised with, and the fingerprint
// ssh-keygen -lf prints for the first of them.
const (
	keyAlice          = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINbLmcjzUpQm1iTZZ1ZJV70AoGpmRi8DhMiRdISAjRIP alice@laptop"
	keyBob            = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINfnqsPmPLwnm0g6tJVVk0G0ub/l2ng5plLlpxuXnkEn bob@laptop"
	fingerprintAlice  = "SHA256:MBkOVJQdzV+obyHKOxaP12lcJVbycDH6lMIuagMZGQ8"
	canonicalKeyAlice = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINbLmcjzUpQm1iTZZ1ZJV70AoGpmRi8DhMiRdISAjRIP"
	directiveKeyAlice = `command="/bin/sh",no-pty ` + keyAlice
	unparseableKey    = "ssh-ed25519 this-is-not-base64-key-material"
)

// enrol posts one key and returns the record the route answered with.
func (f *apiFixture) enrol(in core.EnrolSSHKeyInput) core.SSHKey {
	f.t.Helper()
	resp := f.call(http.MethodPost, wire.RouteSSHKeys, in)
	mustStatus(f.t, resp, http.StatusCreated)
	var key core.SSHKey
	decodeBody(f.t, resp, &key)
	return key
}

func TestEnrolSSHKeyRouteAnswersTheEnrolledRecord(t *testing.T) {
	f := newFixture(t)
	key := f.enrol(core.EnrolSSHKeyInput{PublicKey: keyAlice, Label: "laptop"})

	if key.ID == "" {
		t.Error("the enrolled key carries no identifier")
	}
	if key.Fingerprint != fingerprintAlice {
		t.Errorf("fingerprint = %q, want the one ssh-keygen prints, %q", key.Fingerprint, fingerprintAlice)
	}
	if key.PublicKey != canonicalKeyAlice {
		t.Errorf("public key = %q, want the canonical form %q", key.PublicKey, canonicalKeyAlice)
	}
	if key.Label != "laptop" {
		t.Errorf("label = %q, want %q", key.Label, "laptop")
	}
	if key.ActorID != f.actorA.ID {
		t.Errorf("actor = %q, want the calling actor %q", key.ActorID, f.actorA.ID)
	}
	if key.TenantID != f.tenantA.ID {
		t.Errorf("tenant = %q, want the calling tenant %q", key.TenantID, f.tenantA.ID)
	}
	if !key.Active() {
		t.Error("a freshly enrolled key is not active")
	}
}

// The route hands the submission to the service and returns what was stored.
// It must not parse, canonicalize or re-derive anything of its own, so the
// comment the submission carried is absent from the response.
func TestEnrolSSHKeyRouteReturnsWhatWasStoredRatherThanWhatWasSubmitted(t *testing.T) {
	f := newFixture(t)
	key := f.enrol(core.EnrolSSHKeyInput{PublicKey: "  " + keyAlice + "\n"})

	if key.PublicKey != canonicalKeyAlice {
		t.Fatalf("public key = %q, want %q", key.PublicKey, canonicalKeyAlice)
	}
	for _, forbidden := range []string{"alice@laptop", "command=", "no-pty"} {
		if strings.Contains(key.PublicKey, forbidden) {
			t.Errorf("the response carries %q from the submission", forbidden)
		}
	}
}

// A line carrying authorized_keys options is refused by the frozen contract
// before the service can parse it, so the transport never sees a directive.
// The spec's "Options are not stored" scenario expects such a line to enrol
// successfully; that disagreement lives in core.EnrolSSHKeyInput.Validate.
func TestEnrolSSHKeyRouteRefusesAnOptionsBearingLine(t *testing.T) {
	f := newFixture(t)
	resp := f.call(http.MethodPost, wire.RouteSSHKeys,
		core.EnrolSSHKeyInput{PublicKey: directiveKeyAlice})
	mustStatus(t, resp, http.StatusBadRequest)
	var env wire.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Code != core.KindInvalid {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindInvalid)
	}
	if strings.Contains(env.Error.Message, "command=") {
		t.Errorf("the refusal echoes the submitted directive: %q", env.Error.Message)
	}
}

func TestListSSHKeysRouteReportsARevokedKeyRatherThanOmittingIt(t *testing.T) {
	f := newFixture(t)
	kept := f.enrol(core.EnrolSSHKeyInput{PublicKey: keyAlice, Label: "kept"})
	gone := f.enrol(core.EnrolSSHKeyInput{PublicKey: keyBob, Label: "gone"})

	revoked := f.call(http.MethodDelete, "/api/v1/ssh-keys/"+gone.ID, nil)
	mustStatus(t, revoked, http.StatusNoContent)
	_ = revoked.Body.Close()

	listed := f.call(http.MethodGet, wire.RouteSSHKeys+"?actor_id="+f.actorA.ID, nil)
	mustStatus(t, listed, http.StatusOK)
	var page wire.Page[core.SSHKey]
	decodeBody(t, listed, &page)

	if len(page.Items) != 2 {
		t.Fatalf("listed %d keys, want both the live one and the revoked one", len(page.Items))
	}
	byID := map[string]core.SSHKey{}
	for _, k := range page.Items {
		byID[k.ID] = k
	}
	if !byID[kept.ID].Active() {
		t.Error("the key that was not revoked is reported as revoked")
	}
	revokedKey, ok := byID[gone.ID]
	if !ok {
		t.Fatal("the revoked key was omitted from the listing")
	}
	if revokedKey.Active() || revokedKey.RevokedAt == nil {
		t.Errorf("the revoked key is not reported as revoked: %+v", revokedKey)
	}
	if revokedKey.Label != "gone" {
		t.Errorf("label = %q, want %q", revokedKey.Label, "gone")
	}
}

func TestSSHKeyRoutesRefuseBadInputWithTheStandardEnvelope(t *testing.T) {
	f := newFixture(t)

	cases := []struct {
		name   string
		body   any
		status int
		want   string
	}{
		{"no key", core.EnrolSSHKeyInput{}, http.StatusBadRequest, "a public key is required"},
		{"a private key", core.EnrolSSHKeyInput{
			PublicKey: "-----BEGIN OPENSSH PRIVATE KEY-----"}, http.StatusBadRequest,
			"that is a private key"},
		{"not a key at all", core.EnrolSSHKeyInput{PublicKey: "hello"}, http.StatusBadRequest,
			"authorized_keys form"},
		{"unparseable", core.EnrolSSHKeyInput{PublicKey: unparseableKey},
			http.StatusBadRequest, "not an ssh public key"},
		{"two keys at once", core.EnrolSSHKeyInput{PublicKey: keyAlice + "\n" + keyBob},
			http.StatusBadRequest, "one key at a time"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.call(http.MethodPost, wire.RouteSSHKeys, tc.body)
			mustStatus(t, resp, tc.status)
			var env wire.ErrorEnvelope
			decodeBody(t, resp, &env)
			if env.Error.Code != core.KindInvalid {
				t.Errorf("code = %q, want %q", env.Error.Code, core.KindInvalid)
			}
			if !strings.Contains(env.Error.Message, tc.want) {
				t.Errorf("message = %q, want it to mention %q", env.Error.Message, tc.want)
			}
		})
	}

	nothing := f.call(http.MethodGet, wire.RouteSSHKeys, nil)
	mustStatus(t, nothing, http.StatusOK)
	var page wire.Page[core.SSHKey]
	decodeBody(t, nothing, &page)
	if len(page.Items) != 0 {
		t.Fatalf("a refused enrolment recorded %d keys", len(page.Items))
	}
}

func TestEnrollingTheSameKeyTwiceInOneTenantConflicts(t *testing.T) {
	f := newFixture(t)
	f.enrol(core.EnrolSSHKeyInput{PublicKey: keyAlice})

	again := f.call(http.MethodPost, wire.RouteSSHKeys,
		core.EnrolSSHKeyInput{PublicKey: canonicalKeyAlice + " a different comment"})
	mustStatus(t, again, http.StatusConflict)
	var env wire.ErrorEnvelope
	decodeBody(t, again, &env)
	if env.Error.Code != core.KindConflict {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindConflict)
	}
}

func TestRevokeSSHKeyRouteReportsAKeyItCannotFind(t *testing.T) {
	f := newFixture(t)
	resp := f.call(http.MethodDelete, "/api/v1/ssh-keys/no-such-key", nil)
	mustStatus(t, resp, http.StatusNotFound)
	var env wire.ErrorEnvelope
	decodeBody(t, resp, &env)
	if env.Error.Code != core.KindNotFound {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindNotFound)
	}
}

// An enrolled key is tenant-scoped data: no route may list it, name it or
// revoke it from another tenant.
func TestSSHKeyRoutesDoNotLeakAcrossTenants(t *testing.T) {
	f := newFixture(t)
	mine := f.enrol(core.EnrolSSHKeyInput{PublicKey: keyAlice, Label: "tenant a laptop"})

	// The same key enrolled in the second tenant is a second, separate row.
	theirs := f.do(http.MethodPost, wire.RouteSSHKeys, f.hostB, f.tokenB,
		core.EnrolSSHKeyInput{PublicKey: keyAlice, Label: "tenant b laptop"})
	mustStatus(t, theirs, http.StatusCreated)
	var other core.SSHKey
	decodeBody(t, theirs, &other)
	if other.ID == mine.ID || other.TenantID != f.tenantB.ID {
		t.Fatalf("the second tenant's enrolment is not its own row: %+v", other)
	}

	listed := f.do(http.MethodGet, wire.RouteSSHKeys, f.hostB, f.tokenB, nil)
	mustStatus(t, listed, http.StatusOK)
	body := readBody(t, listed)
	for _, forbidden := range []string{mine.ID, "tenant a laptop", f.actorA.ID, f.tenantA.ID} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the other tenant's listing discloses %q:\n%s", forbidden, body)
		}
	}

	named := f.do(http.MethodGet, wire.RouteSSHKeys+"?actor_id="+f.actorA.ID,
		f.hostB, f.tokenB, nil)
	mustStatus(t, named, http.StatusNotFound)
	if named := readBody(t, named); strings.Contains(named, "tenant a laptop") {
		t.Errorf("naming another tenant's actor disclosed its keys:\n%s", named)
	}

	revoke := f.do(http.MethodDelete, "/api/v1/ssh-keys/"+mine.ID, f.hostB, f.tokenB, nil)
	mustStatus(t, revoke, http.StatusNotFound)
	_ = revoke.Body.Close()

	still := f.call(http.MethodGet, wire.RouteSSHKeys, nil)
	mustStatus(t, still, http.StatusOK)
	var page wire.Page[core.SSHKey]
	decodeBody(t, still, &page)
	if len(page.Items) != 1 || !page.Items[0].Active() {
		t.Fatalf("a cross-tenant revocation reached the key: %+v", page.Items)
	}
}
