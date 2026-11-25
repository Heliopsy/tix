// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestServerPublicURLIsRegisteredWithItsVariable(t *testing.T) {
	key, ok := Lookup("server.public_url")
	if !ok {
		t.Fatal("server.public_url is not registered")
	}
	if key.Env != "TIX_SERVER_PUBLIC_URL" {
		t.Fatalf("server.public_url generates %q", key.Env)
	}
	if key.Secret() {
		t.Fatal("a public address is not a secret")
	}
}

// TestServerPublicURLIsAnAddressWithABasePath refuses what would end up
// inside every link people paste, and accepts the shapes an operator writes.
func TestServerPublicURLIsAnAddressWithABasePath(t *testing.T) {
	cases := map[string]bool{
		"":                                  true,
		"https://tix.example.com":           true,
		"https://tix.example.com/":          true,
		"http://10.0.0.5:8080":              true,
		"https://example.com/tix":           true,
		"https://example.com/tix/":          true,
		"https://[2001:db8::1]:8443":        true,
		"tix.example.com":                   false,
		"ftp://tix.example.com":             false,
		"https://":                          false,
		"https:tix.example.com":             false,
		"https://tix.example.com/?a=b":      false,
		"https://tix.example.com/?":         false,
		"https://tix.example.com/#top":      false,
		"https://root@tix.example.com":      false,
		"https://tix.example.com:0":         false,
		"https://tix.example.com:99999":     false,
		"https://tix example.com":           false,
		"https://tix.example.com/tasks?x#y": false,
	}
	for raw, ok := range cases {
		cfg := Defaults()
		cfg.Server.PublicURL = raw
		err := Validate(&cfg, map[string]Layer{"server.public_url": LayerEnv})
		if (err == nil) != ok {
			t.Errorf("server.public_url %q: Validate = %v, want accepted %v", raw, err, ok)
			continue
		}
		if err != nil && (!core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "server.public_url")) {
			t.Errorf("server.public_url %q: the refusal %v does not name the key", raw, err)
		}
	}
}

func TestPublicBaseDropsTheTrailingSlash(t *testing.T) {
	for raw, want := range map[string]string{
		"":                          "",
		"https://tix.example.com/":  "https://tix.example.com",
		"https://example.com/tix//": "https://example.com/tix",
		"http://h:8080":             "http://h:8080",
	} {
		if got := (Server{PublicURL: raw}).PublicBase(); got != want {
			t.Errorf("PublicBase(%q) = %q, want %q", raw, got, want)
		}
	}
}
