// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "testing"

// An open redirect turns a login link into a phishing vector: the victim signs
// in legitimately and is then bounced to an attacker's page.
func TestSafeNextRefusesOffSiteDestinations(t *testing.T) {
	for _, hostile := range []string{
		"https://evil.example.com",
		"//evil.example.com",
		"http://evil.example.com/path",
		`/\evil.example.com`,
		`/\/evil.example.com`,
		"javascript:alert(1)",
		"/\tevil",
		"/\r\nLocation: https://evil.example.com",
		"",
		"projects",
	} {
		if got := safeNext(hostile); got != RouteProjects {
			t.Errorf("safeNext(%q) = %q, want the safe default", hostile, got)
		}
	}
}

func TestSafeNextKeepsRelativePaths(t *testing.T) {
	for _, ok := range []string{"/", "/projects", "/admin/users", "/tasks?status=todo"} {
		if got := safeNext(ok); got != ok {
			t.Errorf("safeNext(%q) = %q, want it preserved", ok, got)
		}
	}
}
