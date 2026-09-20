package auth

import (
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
)

const testPassword = "correct horse battery staple"

func testHasher(t *testing.T) *Hasher {
	t.Helper()
	return NewHasherWithParams(TestParams())
}

func TestHashVerifyRoundTrip(t *testing.T) {
	h := testHasher(t)
	encoded, err := h.Hash(testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("unexpected encoding prefix: %q", encoded)
	}
	if strings.Contains(encoded, testPassword) {
		t.Fatal("encoded hash contains the plaintext password")
	}
	if err := Verify(encoded, testPassword); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyRejects(t *testing.T) {
	h := testHasher(t)
	good, err := h.Hash(testPassword)
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
			err := Verify(tt.encoded, tt.password)
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
		encoded, err := h.Hash(testPassword)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		if seen[encoded] {
			t.Fatal("the same password produced an identical hash twice")
		}
		seen[encoded] = true
		if err := Verify(encoded, testPassword); err != nil {
			t.Fatalf("verify: %v", err)
		}
	}
}

func TestHashRejectsShortPassword(t *testing.T) {
	h := testHasher(t)
	short := strings.Repeat("x", core.MinPasswordLength-1)
	_, err := h.Hash(short)
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
	strong := DefaultParams()

	encoded, err := NewHasherWithParams(weak).Hash(testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	tests := []struct {
		name string
		want Params
		got  bool
	}{
		{"same parameters", weak, false},
		{"stronger memory", Params{Memory: weak.Memory * 2, Time: weak.Time, Threads: weak.Threads, SaltLength: weak.SaltLength, KeyLength: weak.KeyLength}, true},
		{"stronger time", Params{Memory: weak.Memory, Time: weak.Time + 1, Threads: weak.Threads, SaltLength: weak.SaltLength, KeyLength: weak.KeyLength}, true},
		{"longer key", Params{Memory: weak.Memory, Time: weak.Time, Threads: weak.Threads, SaltLength: weak.SaltLength, KeyLength: weak.KeyLength + 16}, true},
		{"production policy", strong, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsRehash(encoded, tt.want); got != tt.got {
				t.Fatalf("NeedsRehash = %v, want %v", got, tt.got)
			}
		})
	}

	if !NeedsRehash("garbage", strong) {
		t.Fatal("an unparsable hash must be reported as needing a rehash")
	}
}

func TestDefaultParamsAreStrong(t *testing.T) {
	p := DefaultParams()
	if p.Memory != 64*1024 || p.Time != 3 || p.Threads != 4 {
		t.Fatalf("unexpected production parameters: %+v", p)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("default params invalid: %v", err)
	}
	if err := (Params{}).Validate(); err == nil {
		t.Fatal("expected zero parameters to be rejected")
	}
}

func TestHasherUsesDefaultParams(t *testing.T) {
	if got := NewHasher().Params(); got != DefaultParams() {
		t.Fatalf("NewHasher params = %+v", got)
	}
	if _, err := NewHasherWithParams(Params{}).Hash(testPassword); err == nil {
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
