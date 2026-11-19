// SPDX-License-Identifier: AGPL-3.0-or-later

package samlrefresh

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

type tenantKey struct{}

// fakeService holds providers per tenant and records each refresh as the
// service would: the provider is read now.
type fakeService struct {
	mu        sync.Mutex
	clk       *clock.Fake
	providers map[string][]core.SSOProvider
	fail      map[string]error
	updated   map[string]bool
	refreshed []string
	calls     chan string
}

func (f *fakeService) ListSSOProviders(ctx context.Context) ([]core.SSOProvider, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.providers[ctx.Value(tenantKey{}).(string)]), nil
}

func (f *fakeService) RefreshSAMLMetadata(ctx context.Context, ref string) (*core.SSOProvider, error) {
	tenant := ctx.Value(tenantKey{}).(string)
	f.mu.Lock()
	defer func() {
		f.mu.Unlock()
		if f.calls != nil {
			f.calls <- tenant + "/" + ref
		}
	}()
	f.refreshed = append(f.refreshed, tenant+"/"+ref)
	list := f.providers[tenant]
	i := slices.IndexFunc(list, func(p core.SSOProvider) bool { return p.ID == ref })
	result := core.SAMLRefreshUnchanged
	err := f.fail[ref]
	if err != nil {
		result = core.SAMLRefreshFailed
	} else if f.updated[ref] {
		result = core.SAMLRefreshUpdated
	}
	list[i].MetadataRefresh = &core.SAMLMetadataRefresh{At: f.clk.Now(), Result: result}
	if err != nil {
		return nil, err
	}
	p := list[i]
	return &p, nil
}

func (f *fakeService) took() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := slices.Clone(f.refreshed)
	f.refreshed = nil
	return out
}

func saml(id string, edit func(*core.SSOProvider)) core.SSOProvider {
	p := core.SSOProvider{ID: id, Key: id, Kind: core.SSOKindSAML, Enabled: true,
		IdPMetadataURL: "https://idp.example/" + id}
	if edit != nil {
		edit(&p)
	}
	return p
}

func newFixture(interval time.Duration, opts ...Option) (*Refresher, *fakeService, *clock.Fake) {
	clk := clock.NewFakeAt()
	svc := &fakeService{clk: clk, fail: map[string]error{}, providers: map[string][]core.SSOProvider{
		"t1": {
			saml("okta", nil),
			saml("off", func(p *core.SSOProvider) { p.Enabled = false }),
			saml("pasted", func(p *core.SSOProvider) { p.IdPMetadataURL = "" }),
			{ID: "kc", Key: "kc", Kind: core.SSOKindOIDC, Enabled: true},
		},
		"t2": {saml("entra", nil)},
	}}
	r := New(svc, func(context.Context) ([]string, error) { return []string{"t1", "t2"}, nil },
		func(ctx context.Context, tenant string) context.Context {
			return context.WithValue(ctx, tenantKey{}, tenant)
		},
		clk, interval, opts...)
	return r, svc, clk
}

func TestRefresherReadsEnabledSAMLProvidersInEveryTenantWhenDue(t *testing.T) {
	r, svc, clk := newFixture(12 * time.Hour)
	res := r.RunOnce(context.Background())
	if got := svc.took(); !slices.Equal(got, []string{"t1/okta", "t2/entra"}) || res.Refreshed != 2 {
		t.Fatalf("first pass refreshed %v (%+v), want the enabled SAML providers with a metadata URL", got, res)
	}
	clk.Advance(11 * time.Hour)
	if got := svc.took(); r.RunOnce(context.Background()).Refreshed != 0 || len(got) != 0 {
		t.Fatalf("a pass before the interval refreshed %v, want nothing", got)
	}
	clk.Advance(time.Hour)
	r.RunOnce(context.Background())
	if got := svc.took(); len(got) != 2 {
		t.Fatalf("a pass at the interval refreshed %v, want both again", got)
	}
}

func TestRefresherRetriesAFailureSoonerAndKeepsGoing(t *testing.T) {
	var reported []string
	r, svc, clk := newFixture(12*time.Hour, WithErrorHandler(func(tenant, key string, err error) {
		reported = append(reported, tenant+"/"+key+": "+err.Error())
	}))
	svc.fail["okta"] = errors.New("idp down")
	res := r.RunOnce(context.Background())
	if res.Failed != 1 || res.Refreshed != 2 || len(reported) != 1 || reported[0] != "t1/okta: idp down" {
		t.Fatalf("pass = %+v, reported %v; want okta's failure reported and entra still read", res, reported)
	}
	svc.took()
	clk.Advance(core.SAMLRefreshRetry)
	r.RunOnce(context.Background())
	if got := svc.took(); !slices.Equal(got, []string{"t1/okta"}) {
		t.Fatalf("an hour after a failure refreshed %v, want only the failed provider", got)
	}
}

func TestRefresherRunsAtStartThenOnItsTicker(t *testing.T) {
	r, svc, clk := newFixture(12 * time.Hour)
	svc.calls = make(chan string, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	<-svc.calls
	<-svc.calls
	clk.Advance(12 * time.Hour)
	<-svc.calls
	<-svc.calls
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Run = %v, want cancellation", err)
	}
}

func TestRefresherOffDoesNothing(t *testing.T) {
	r, svc, clk := newFixture(0)
	if r.Enabled() {
		t.Fatal("a zero interval reports enabled")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	clk.Advance(48 * time.Hour)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) || len(svc.took()) != 0 || clk.Tickers() != 0 {
		t.Errorf("an off refresher ran %v (%v), want nothing", svc.took(), err)
	}
}

func TestRefresherReportsATenantListingFailure(t *testing.T) {
	var reported []error
	clk := clock.NewFakeAt()
	boom := errors.New("store closed")
	r := New(&fakeService{clk: clk}, func(context.Context) ([]string, error) { return nil, boom },
		func(ctx context.Context, _ string) context.Context { return ctx }, clk, time.Hour,
		WithErrorHandler(func(_, _ string, err error) { reported = append(reported, err) }))
	if res := r.RunOnce(context.Background()); res.Refreshed != 0 || len(reported) != 1 || !errors.Is(reported[0], boom) {
		t.Fatalf("pass = %+v, reported %v; want the listing failure reported", res, reported)
	}
}

func TestRefresherReportsAChange(t *testing.T) {
	var changed []string
	r, svc, _ := newFixture(time.Hour, WithChangeHandler(func(tenant string, p *core.SSOProvider) {
		changed = append(changed, tenant+"/"+p.Key)
	}))
	svc.updated = map[string]bool{"entra": true}
	if res := r.RunOnce(context.Background()); res.Changed != 1 || !slices.Equal(changed, []string{"t2/entra"}) {
		t.Fatalf("pass = %+v, changes %v; want entra's change reported", res, changed)
	}
}

func TestRefresherStopsBetweenTenantsWhenCancelled(t *testing.T) {
	r, svc, _ := newFixture(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if res := r.RunOnce(ctx); res.Refreshed != 0 || len(svc.took()) != 0 {
		t.Fatalf("a cancelled pass refreshed %+v", res)
	}
}
