// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

const flowKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func TestASealedFlowOpensOnlyUnalteredAndInTime(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	flow := SSOFlow{TenantID: "t", ProviderID: "p", State: "s", Nonce: "n", Verifier: "v",
		RedirectURI: "http://127.0.0.1:1/callback", Surface: core.SurfaceCLI, ExpiresAt: now.Add(SSOFlowTTL)}
	sealed, err := SealSSOFlow(flowKey, flow)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "127.0.0.1") {
		t.Error("the sealed flow carries its contents in the clear")
	}
	got, err := OpenSSOFlow(flowKey, sealed, now)
	if err != nil || got != flow {
		t.Fatalf("opened %+v, %v; want %+v", got, err, flow)
	}
	tampered := []byte(sealed)
	tampered[len(tampered)/2] ^= 1
	other := "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXphYmNkZWY="
	for name, try := range map[string]func() error{
		"tampered": func() error { _, err := OpenSSOFlow(flowKey, string(tampered), now); return err },
		"expired":  func() error { _, err := OpenSSOFlow(flowKey, sealed, now.Add(SSOFlowTTL)); return err },
		"wrong key": func() error {
			_, err := OpenSSOFlow(other, sealed, now)
			return err
		},
		"garbage": func() error { _, err := OpenSSOFlow(flowKey, "!!", now); return err },
	} {
		if err := try(); !core.IsKind(err, core.KindUnauthenticated) {
			t.Errorf("%s flow = %v, want unauthenticated", name, err)
		}
	}
	if _, err := SealSSOFlow("", flow); !core.IsKind(err, core.KindPrecondition) {
		t.Errorf("sealing without a key = %v, want a precondition refusal", err)
	}
	a, _ := NewSSOSecret()
	b, _ := NewSSOSecret()
	if len(a) != 43 || a == b {
		t.Errorf("secrets %q and %q, want two distinct 43 character values", a, b)
	}
}
