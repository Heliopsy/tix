// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/wire"
)

// On a shared host the tenant of a request is the tenant of its credential.
// Under /dav/ that credential is the Basic password and nothing else; off it,
// a Basic password is never read.
func TestCredentialHashReadsABasicPasswordUnderDAVOnly(t *testing.T) {
	const password, cookie, bearer = "tix_pat_app", "session-value", "tix_pat_bearer"
	for _, tc := range []struct {
		name, path string
		setup      func(r *http.Request)
		want       string
	}{
		{"basic under dav", "/dav/acme/calendars/x/", func(r *http.Request) { r.SetBasicAuth("ana", password) },
			auth.HashToken(password)},
		{"basic at the dav root", "/dav/", func(r *http.Request) { r.SetBasicAuth("ana", password) },
			auth.HashToken(password)},
		{"a cookie under dav", "/dav/", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: wire.SessionCookieName, Value: cookie})
		}, ""},
		{"a bearer under dav", "/dav/", func(r *http.Request) { r.Header.Set(wire.HeaderAuth, "Bearer "+bearer) }, ""},
		{"basic on the api", wire.RouteTasks, func(r *http.Request) { r.SetBasicAuth("ana", password) }, ""},
		{"a bearer on the api", wire.RouteTasks, func(r *http.Request) { r.Header.Set(wire.HeaderAuth, "Bearer "+bearer) },
			auth.HashToken(bearer)},
		{"a path that only starts like dav", "/davx/", func(r *http.Request) { r.SetBasicAuth("ana", password) }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := http.NewRequest("PROPFIND", tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			tc.setup(r)
			if got := credentialHash(r); got != tc.want {
				t.Errorf("credentialHash = %q, want %q", got, tc.want)
			}
		})
	}
}
