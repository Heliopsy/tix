// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import "testing"

func TestValidateEmail(t *testing.T) {
	for email, ok := range map[string]bool{
		"ada@example.com":    true,
		"  Ada@Example.COM ": true,
		"ada@localhost":      true,
		"":                   false,
		"ada":                false,
		"@example.com":       false,
		"ada@":               false,
		"ada@b@c":            false,
		"a da@example.com":   false,
		"ada@exa\tmple.com":  false,
	} {
		if err := ValidateEmail(email); (err == nil) != ok {
			t.Errorf("ValidateEmail(%q) = %v, want ok=%v", email, err, ok)
		}
	}
}

func TestTenantAdminHandleOrDefault(t *testing.T) {
	for _, tc := range []struct {
		in   TenantAdminInput
		want string
	}{
		{TenantAdminInput{Email: "Villy@Z09.test"}, "villy"},
		{TenantAdminInput{Email: "villy@z09.test", Handle: " VK "}, "vk"},
		{TenantAdminInput{Email: "@z09.test"}, ""},
	} {
		if got := tc.in.HandleOrDefault(); got != tc.want {
			t.Errorf("HandleOrDefault(%+v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCreateTenantInputValidatesItsAdmins(t *testing.T) {
	many := make([]TenantAdminInput, MaxTenantAdmins+1)
	for i := range many {
		many[i] = TenantAdminInput{Email: string(rune('a'+i)) + "@x.test"}
	}
	for _, tc := range []struct {
		name string
		in   CreateTenantInput
		ok   bool
	}{
		{"no admins", CreateTenantInput{Key: "acme", Name: "Acme"}, true},
		{"two admins", CreateTenantInput{Key: "acme", Name: "Acme", Admins: []TenantAdminInput{
			{Email: "a@x.test"}, {Email: "b@x.test", Password: "long-enough-pass"}}}, true},
		{"bad key", CreateTenantInput{Key: "9", Name: "Acme"}, false},
		{"no name", CreateTenantInput{Key: "acme"}, false},
		{"too many", CreateTenantInput{Key: "acme", Name: "Acme", Admins: many}, false},
		{"bad email", CreateTenantInput{Key: "acme", Name: "Acme", Admins: []TenantAdminInput{{Email: "nope"}}}, false},
		{"twice", CreateTenantInput{Key: "acme", Name: "Acme", Admins: []TenantAdminInput{
			{Email: "a@x.test"}, {Email: "A@X.test"}}}, false},
		{"same handle", CreateTenantInput{Key: "acme", Name: "Acme", Admins: []TenantAdminInput{
			{Email: "a@x.test"}, {Email: "a@y.test"}}}, false},
		{"short password", CreateTenantInput{Key: "acme", Name: "Acme", Admins: []TenantAdminInput{
			{Email: "a@x.test", Password: "short"}}}, false},
		{"handle given but blank", CreateTenantInput{Key: "acme", Name: "Acme", Admins: []TenantAdminInput{
			{Email: "a@x.test", Handle: "  "}}}, true},
	} {
		if err := tc.in.Validate(); (err == nil) != tc.ok {
			t.Errorf("%s: Validate = %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}
