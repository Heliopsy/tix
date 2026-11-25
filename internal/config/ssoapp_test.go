// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSSOAppRedirectsResolve(t *testing.T) {
	if got := Defaults().Auth.SSOAppRedirects; !slices.Equal(got, []string{"red.tix.mobile:/sso"}) {
		t.Fatalf("default sso_app_redirects = %v, want the mobile app's", got)
	}
	home, dir := t.TempDir(), t.TempDir()
	off := mustLoad(t, Options{Dir: dir, Home: home,
		Environ: environ(map[string]string{"HOME": home, "TIX_AUTH_SSO_APP_REDIRECTS": ""})})
	if len(off.Config.Auth.SSOAppRedirects) != 0 {
		t.Errorf("an empty variable left %v, want app sign-in off", off.Config.Auth.SSOAppRedirects)
	}
	write(t, filepath.Join(dir, ".tix.yaml"), "auth:\n  sso_app_redirects: [red.tix.mobile:/sso, com.example.fork:/cb]\n")
	two := mustLoad(t, Options{Dir: dir, Home: home, Environ: environ(map[string]string{"HOME": home})})
	if !slices.Equal(two.Config.Auth.SSOAppRedirects, []string{"red.tix.mobile:/sso", "com.example.fork:/cb"}) {
		t.Errorf("sso_app_redirects from a file = %v", two.Config.Auth.SSOAppRedirects)
	}
	write(t, filepath.Join(dir, ".tix.yaml"), "auth:\n  sso_app_redirects: []\n")
	empty := mustLoad(t, Options{Dir: dir, Home: home, Environ: environ(map[string]string{"HOME": home})})
	if len(empty.Config.Auth.SSOAppRedirects) != 0 {
		t.Errorf("an empty list in a file left %v, want app sign-in off", empty.Config.Auth.SSOAppRedirects)
	}
}

func TestSSOAppRedirectsValidation(t *testing.T) {
	for _, tc := range []struct {
		value string
		ok    bool
	}{
		{"red.tix.mobile:/sso", true},
		{"https://app.example/sso", true},
		{"/sso", false},
		{"red.tix.mobile:/sso#frag", false},
		{"red.tix.mobile:", false},
		{"javascript:alert(1)//", false},
		{"JavaScript:alert(1)", false},
		{"data:text/html,<script>alert(1)</script>", false},
		{"DATA:text/html;base64,PHNjcmlwdD4=", false},
		{"vbscript:msgbox(1)", false},
		{"VBScript:msgbox(1)", false},
		{"file:///etc/passwd", false},
		{"FILE:/etc/passwd", false},
		{"http://app.example/sso", true},
	} {
		cfg := Defaults()
		cfg.Auth.SSOAppRedirects = []string{tc.value}
		err := Validate(&cfg, map[string]Layer{})
		if tc.ok != (err == nil) || (err != nil && !strings.Contains(err.Error(), "auth.sso_app_redirects")) {
			t.Errorf("Validate(%q) = %v, want ok %v", tc.value, err, tc.ok)
		}
	}
}
