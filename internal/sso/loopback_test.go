// SPDX-License-Identifier: AGPL-3.0-or-later

package sso

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestLoopbackRefusesAForeignState(t *testing.T) {
	answers := make(chan core.CompleteSSOInput, 1)
	h := loopbackHandler(&core.SSOBegin{Flow: "f", State: "right"}, answers)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1/callback?code=c&state=wrong", nil))
	if rec.Code != http.StatusBadRequest || len(answers) != 0 {
		t.Errorf("foreign state = %d with %d answers, want 400 and none", rec.Code, len(answers))
	}
}

// fakeLoopback stands in for the service: the address it hands out is the
// loopback callback itself, so opening it is the provider sending the
// browser back.
type fakeLoopback struct {
	query    string
	received core.CompleteSSOInput
}

func (f *fakeLoopback) BeginSSOLogin(_ context.Context, in core.BeginSSOInput) (*core.SSOBegin, error) {
	return &core.SSOBegin{AuthURL: in.RedirectURI + "?" + f.query, Flow: "sealed", State: "right"}, nil
}

func (f *fakeLoopback) CompleteSSOLogin(_ context.Context, in core.CompleteSSOInput) (*core.LoginResult, error) {
	f.received = in
	return &core.LoginResult{Session: &core.Session{Token: "tok"}}, nil
}

func openByGet(address string) error {
	go func() {
		if resp, err := http.Get(address); err == nil {
			_ = resp.Body.Close()
		}
	}()
	return nil
}

func TestLoopbackHandsTheCodeToTheService(t *testing.T) {
	for _, tc := range []struct {
		query, code, errText string
	}{
		{"code=c1&state=right", "c1", ""},
		{"error=access_denied&error_description=no&state=right", "", "access_denied no"},
	} {
		svc := &fakeLoopback{query: tc.query}
		var printed strings.Builder
		got, err := Loopback(context.Background(), svc, "kc", true, &printed, openByGet)
		if err != nil || got.Session == nil {
			t.Fatalf("Loopback = %+v, %v", got, err)
		}
		if svc.received.Code != tc.code || svc.received.Error != tc.errText || svc.received.Flow != "sealed" {
			t.Errorf("completed with %+v", svc.received)
		}
		if !strings.Contains(printed.String(), "http://127.0.0.1:") {
			t.Errorf("printed %q, want the address", printed.String())
		}
	}
}

func TestLoopbackSaysWhenNoBrowserOpensAndStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var printed strings.Builder
	opened := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := Loopback(ctx, &fakeLoopback{query: "state=other"}, "kc", false, &printed,
			func(string) error { close(opened); return errors.New("no browser") })
		done <- err
	}()
	<-opened
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Loopback after cancel = %v, want the context's error", err)
	}
	if !strings.Contains(printed.String(), "no browser could be opened") {
		t.Errorf("printed %q, want it to say no browser opened", printed.String())
	}
}
