// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

const testPassword = "correct horse battery staple"

func testHasher(t *testing.T) *Hasher {
	t.Helper()
	return NewHasherWithParams(TestParams())
}

func TestHashVerifyRoundTrip(t *testing.T) {
	h := testHasher(t)
	encoded, err := h.Hash(t.Context(), testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("unexpected encoding prefix: %q", encoded)
	}
	if strings.Contains(encoded, testPassword) {
		t.Fatal("encoded hash contains the plaintext password")
	}
	if err := Verify(t.Context(), encoded, testPassword); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyRejects(t *testing.T) {
	h := testHasher(t)
	good, err := h.Hash(t.Context(), testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	tests := []struct {
		name     string
		encoded  string
		password string
	}{
		{"wrong password", good, "wrong horse battery staple"},
		{"empty password", good, ""},
		{"empty hash", "", testPassword},
		{"malformed", "not-a-hash", testPassword},
		{"truncated", good[:len(good)/2], testPassword},
		{"missing fields", "$argon2id$v=19$m=8,t=1,p=1$c2FsdA", testPassword},
		{"unknown algorithm", strings.Replace(good, "argon2id", "argon2i", 1), testPassword},
		{"unknown version", strings.Replace(good, "v=19", "v=16", 1), testPassword},
		{"bad params", strings.Replace(good, "m=", "x=", 1), testPassword},
		{"bad salt", "$argon2id$v=19$m=8,t=1,p=1$!!!!$" + strings.Repeat("a", 43), testPassword},
		{"bad key", "$argon2id$v=19$m=8,t=1,p=1$c2FsdHNhbHRzYWx0c2FsdA$!!!!", testPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Verify(t.Context(), tt.encoded, tt.password)
			if err == nil {
				t.Fatal("expected verification to fail")
			}
			if core.KindOf(err) != core.KindUnauthenticated && core.KindOf(err) != core.KindInvalid {
				t.Fatalf("unexpected error kind %q", core.KindOf(err))
			}
			assertNoSecret(t, err.Error(), testPassword)
		})
	}
}

func TestHashIsSalted(t *testing.T) {
	h := testHasher(t)
	seen := make(map[string]bool)
	for range 5 {
		encoded, err := h.Hash(t.Context(), testPassword)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		if seen[encoded] {
			t.Fatal("the same password produced an identical hash twice")
		}
		seen[encoded] = true
		if err := Verify(t.Context(), encoded, testPassword); err != nil {
			t.Fatalf("verify: %v", err)
		}
	}
}

func TestHashRejectsShortPassword(t *testing.T) {
	h := testHasher(t)
	short := strings.Repeat("x", core.MinPasswordLength-1)
	_, err := h.Hash(t.Context(), short)
	if err == nil {
		t.Fatal("expected a short password to be rejected")
	}
	if core.KindOf(err) != core.KindInvalid {
		t.Fatalf("unexpected kind %q", core.KindOf(err))
	}
	assertNoSecret(t, err.Error(), short)

	if err := ValidatePassword(strings.Repeat("x", core.MinPasswordLength)); err != nil {
		t.Fatalf("expected a long enough password to pass: %v", err)
	}
}

func TestNeedsRehash(t *testing.T) {
	weak := TestParams()
	encoded, err := NewHasherWithParams(weak).Hash(t.Context(), testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	with := func(edit func(*Params)) Params { p := weak; edit(&p); return p }

	tests := []struct {
		name string
		want Params
		got  bool
	}{
		{"same parameters", weak, false},
		{"stronger memory", with(func(p *Params) { p.Memory *= 2 }), true},
		{"stronger time", with(func(p *Params) { p.Time++ }), true},
		{"more lanes", with(func(p *Params) { p.Threads++ }), true},
		{"longer key", with(func(p *Params) { p.KeyLength += 16 }), true},
		{"longer salt", with(func(p *Params) { p.SaltLength += 16 }), true},
		{"production policy", DefaultParams(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsRehash(encoded, tt.want); got != tt.got {
				t.Fatalf("NeedsRehash = %v, want %v", got, tt.got)
			}
		})
	}

	if !NeedsRehash("garbage", DefaultParams()) {
		t.Fatal("an unparsable hash must be reported as needing a rehash")
	}
}

// legacyParams are the defaults tix shipped before 46 MiB: hashes stored with
// them are in every existing database.
var legacyParams = Params{Memory: 64 * 1024, Time: 3, Threads: 4, SaltLength: 16, KeyLength: 32}

// TestCostlierHashesMigrateDown pins that a hash stored at a costlier setting
// than the policy still verifies with its own parameters, is reported for a
// rehash, and that the rehash lands on the policy and verifies. The legacy
// shape is scaled down so the test stays cheap; what matters is that it is
// stronger than the policy in every dimension.
func TestCostlierHashesMigrateDown(t *testing.T) {
	policy := TestParams()
	costly := Params{Memory: policy.Memory * 2, Time: policy.Time + 2, Threads: policy.Threads + 3,
		SaltLength: policy.SaltLength, KeyLength: policy.KeyLength}

	stored, err := NewHasherWithParams(costly).Hash(t.Context(), testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := Verify(t.Context(), stored, testPassword); err != nil {
		t.Fatalf("a hash at its own parameters stopped verifying: %v", err)
	}
	if !NeedsRehash(stored, policy) {
		t.Fatal("a costlier hash than the policy was not reported for a rehash")
	}
	rehashed, err := NewHasherWithParams(policy).Hash(t.Context(), testPassword)
	if err != nil {
		t.Fatalf("rehash: %v", err)
	}
	if NeedsRehash(rehashed, policy) {
		t.Fatal("a fresh hash at the policy is reported for another rehash")
	}
	if err := Verify(t.Context(), rehashed, testPassword); err != nil {
		t.Fatalf("the rehashed password does not verify: %v", err)
	}
	if !NeedsRehash(encodeFor(t, legacyParams), DefaultParams()) {
		t.Fatal("a hash at the legacy 64 MiB defaults is not migrated to the new ones")
	}
}

// encodeFor builds an encoded hash at p without deriving, for checks that only
// read its parameters.
func encodeFor(t *testing.T, p Params) string {
	t.Helper()
	encoded, err := NewHasherWithParams(p).Dummy()
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	return encoded
}

func TestDefaultParamsAreStrong(t *testing.T) {
	p := DefaultParams()
	if p.Memory != 46*1024 || p.Time != 2 || p.Threads != 1 {
		t.Fatalf("unexpected production parameters: %+v", p)
	}
	if err := p.CheckPolicy(); err != nil {
		t.Fatalf("default params below the floor: %v", err)
	}
	if err := (Params{}).Validate(); err == nil {
		t.Fatal("expected zero parameters to be rejected")
	}
}

func TestCheckPolicy(t *testing.T) {
	base := DefaultParams()
	with := func(m, tm uint32) Params { p := base; p.Memory, p.Time = m, tm; return p }
	tests := []struct {
		name string
		p    Params
		ok   bool
	}{
		{"default", base, true},
		{"OWASP m=19 MiB t=2", with(19*1024, 2), true},
		{"OWASP m=7 MiB t=5", with(7*1024, 5), true},
		{"legacy 64 MiB t=3", legacyParams, true},
		{"m=46 MiB t=0", with(46*1024, 0), false},
		{"m=19 MiB t=1", with(19*1024, 1), false},
		{"m=6 MiB t=10", with(6*1024, 10), false},
		{"test params", TestParams(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.p.CheckPolicy(); (err == nil) != tt.ok {
				t.Fatalf("CheckPolicy(%+v) = %v", tt.p, err)
			}
		})
	}
}

func TestHasherUsesDefaultParams(t *testing.T) {
	if got := NewHasher().Params(); got != DefaultParams() {
		t.Fatalf("NewHasher params = %+v", got)
	}
	if _, err := NewHasherWithParams(Params{}).Hash(t.Context(), testPassword); err == nil {
		t.Fatal("expected invalid parameters to be rejected")
	}
}

// assertNoSecret fails the test if text leaks the secret value.
func assertNoSecret(t *testing.T, text, secret string) {
	t.Helper()
	if secret == "" {
		return
	}
	if strings.Contains(text, secret) {
		t.Fatalf("secret leaked into %q", text)
	}
}

func TestParamsValidate(t *testing.T) {
	ok := TestParams()
	tests := []struct {
		name string
		p    Params
		bad  bool
	}{
		{"valid", ok, false},
		{"memory too low", Params{Memory: 4, Time: 1, Threads: 1, SaltLength: 16, KeyLength: 32}, true},
		{"no time", Params{Memory: 64, Time: 0, Threads: 1, SaltLength: 16, KeyLength: 32}, true},
		{"no threads", Params{Memory: 64, Time: 1, Threads: 0, SaltLength: 16, KeyLength: 32}, true},
		{"salt too short", Params{Memory: 64, Time: 1, Threads: 1, SaltLength: 4, KeyLength: 32}, true},
		{"key too short", Params{Memory: 64, Time: 1, Threads: 1, SaltLength: 16, KeyLength: 8}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate()
			if tt.bad != (err != nil) {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}
