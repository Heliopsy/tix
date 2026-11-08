// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
)

// loopbackEgress is the operator's egress escape hatch opened for loopback
// only, so a test can deliver to or sync from an httptest server.
var loopbackEgress = egress.Policy{Allow: []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}}

// sharedSyncFixture is a sync fixture on a shared host, as tix serve runs it.
func sharedSyncFixture(t *testing.T, opts ...Option) *syncFixture {
	t.Helper()
	base := newSyncFixture(t, "ops")
	f := *base
	f.local = New(base.local.store, append([]Option{WithClock(base.clock)}, opts...)...)
	return &f
}

func TestASharedHostRefusesToStoreAnInternalSyncURL(t *testing.T) {
	for _, target := range []string{
		"http://127.0.0.1:8080", "http://localhost:8080", "http://169.254.169.254/latest",
		"http://[::ffff:10.0.0.1]/", "https://[fd00::1]/",
	} {
		t.Run(target, func(t *testing.T) {
			f := sharedSyncFixture(t)
			_, err := f.local.PutSyncSource(f.ctx, core.SyncSourceInput{System: core.SystemJira, Name: "corp",
				Mapping: hostMapping, Config: map[string]any{
					core.SyncSettingProject: "CORP", core.SyncSettingURL: target}})
			if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "address not allowed") {
				t.Fatalf("PutSyncSource = %v, want the address refused", err)
			}
			if core.FieldOf(err) != core.SyncSettingURL {
				t.Errorf("refusal field = %q, want %q", core.FieldOf(err), core.SyncSettingURL)
			}
		})
	}
}

func TestStoringALoopbackSyncURLNeedsTheEscapeHatchOrATrustedHost(t *testing.T) {
	for name, f := range map[string]*syncFixture{
		"shared host with the escape hatch": sharedSyncFixture(t, WithEgress(loopbackEgress)),
		"shared host allowing private":      sharedSyncFixture(t, WithEgress(egress.Permissive())),
		"trusted host":                      newSyncFixture(t, "ops"),
	} {
		t.Run(name, func(t *testing.T) {
			putJiraCorp(t, f, "http://127.0.0.1:8080")
		})
	}
}

// A url the operator bound through the environment is not stored, so nothing
// refused it when the source was saved; the run is refused at connect time
// and the server behind it never sees a request.
func TestASharedHostRefusesToSyncFromALoopbackServer(t *testing.T) {
	cases := []struct {
		name    string
		opts    func(*syncFixture) []Option
		reached bool
	}{
		{"strict", func(f *syncFixture) []Option {
			return []Option{WithSyncHost(SyncHost{EnvSources: []string{tenantKeyOf(t, f) + "/*"}})}
		}, false},
		{"escape hatch", func(f *syncFixture) []Option {
			return []Option{WithSyncHost(SyncHost{EnvSources: []string{tenantKeyOf(t, f) + "/*"}}),
				WithEgress(loopbackEgress)}
		}, true},
		{"trusted host", func(*syncFixture) []Option {
			return []Option{WithSyncHost(SyncHost{Trusted: true})}
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trap := newTokenTrap(t)
			setOperatorEnv(t, trap.srv.URL)
			base := newSyncFixture(t, "ops")
			f := *base
			f.local = New(base.local.store, append([]Option{WithClock(base.clock)}, tc.opts(base)...)...)
			useImporter(t, nil)
			src := putJiraCorp(t, &f, "")
			_, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: src.ID, DryRun: true})
			if got := trap.received(operatorToken); got != tc.reached {
				t.Fatalf("the loopback source was reached = %v, want %v (run: %v)", got, tc.reached, err)
			}
			if tc.reached {
				return
			}
			if err == nil || !strings.Contains(err.Error(), "address not allowed") {
				t.Fatalf("RunSync = %v, want the address refused", err)
			}
			if strings.Contains(err.Error(), "127.0.0.1") {
				t.Fatalf("the refusal names the address: %v", err)
			}
		})
	}
}

func TestWebhookAndChannelTargetsOnLoopbackNeedTheEscapeHatch(t *testing.T) {
	const target = "http://127.0.0.1:9000/hook"
	for name, tc := range map[string]struct {
		opts []Option
		ok   bool
	}{
		"strict":               {nil, false},
		"egress escape hatch":  {[]Option{WithEgress(loopbackEgress)}, true},
		"egress allow private": {[]Option{WithEgress(egress.Permissive())}, true},
		"webhooks key":         {[]Option{WithPrivateWebhookTargets(true)}, true},
	} {
		t.Run(name, func(t *testing.T) {
			f := newChannelFixture(t, tc.opts...)
			_, werr := f.l.PutWebhook(adminCtx(f.ada), core.WebhookInput{URL: target, Active: true})
			_, cerr := f.l.CreateInboxChannel(f.adaC, core.InboxChannelInput{Name: "phone", URL: target})
			for what, err := range map[string]error{"webhook": werr, "channel": cerr} {
				if tc.ok && err != nil {
					t.Errorf("%s = %v, want it accepted", what, err)
				}
				if !tc.ok && (err == nil || !strings.Contains(err.Error(), "address not allowed")) {
					t.Errorf("%s = %v, want the address refused", what, err)
				}
			}
		})
	}
}
