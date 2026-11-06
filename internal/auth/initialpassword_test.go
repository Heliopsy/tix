// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"strings"
	"testing"
)

func TestNewInitialPasswordIsLongReadableAndFresh(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		p, err := NewInitialPassword()
		if err != nil {
			t.Fatalf("NewInitialPassword: %v", err)
		}
		body := strings.ReplaceAll(p, "-", "")
		if len(body) != InitialPasswordLength {
			t.Fatalf("password %q has %d characters, want %d", p, len(body), InitialPasswordLength)
		}
		for _, r := range body {
			if !strings.ContainsRune(initialPasswordAlphabet, r) {
				t.Fatalf("password %q holds %q, outside the readable alphabet", p, r)
			}
		}
		if seen[p] {
			t.Fatalf("password %q was generated twice", p)
		}
		seen[p] = true
	}
	if len(initialPasswordAlphabet) != 26+26+10-len(lookAlikes) {
		t.Errorf("alphabet has %d characters, want every letter and digit less the look-alikes", len(initialPasswordAlphabet))
	}
	for _, r := range "0Oo1lI" {
		if strings.ContainsRune(initialPasswordAlphabet, r) {
			t.Errorf("alphabet holds the ambiguous %q", r)
		}
	}
}
