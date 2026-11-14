// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strings"
	"testing"
)

func TestMonogramTakesTheFirstLetterOfTheFirstTwoWords(t *testing.T) {
	cases := []struct{ name, want string }{
		{"Acme", "A"},
		{"acme staging", "AS"},
		{"Acme Corp Europe", "AC"},
		{"  north-west  ops", "NW"},
		{"élan vital", "ÉV"},
		{"42 labs", "4L"},
		{"", ""},
		{"---", ""},
	}
	for _, c := range cases {
		if got := Monogram(c.name); got != c.want {
			t.Errorf("Monogram(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTenantIconTextFallsBackToTheMonogram(t *testing.T) {
	cases := []struct {
		name   string
		tenant Tenant
		want   string
	}{
		{"text icon", Tenant{Name: "Acme", Icon: "🚀"}, "🚀"},
		{"built-in icon", Tenant{Name: "Acme", IconName: "rocket"}, "\U0001F680"},
		{"image", Tenant{Name: "Acme", IconImage: "abc"}, ProjectImageGlyph},
		{"no icon", Tenant{Name: "Acme staging"}, "AS"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.tenant.IconText(); got != c.want {
				t.Errorf("IconText() = %q, want %q", got, c.want)
			}
			m := AccountMembership{TenantName: c.tenant.Name, Icon: c.tenant.Icon,
				IconName: c.tenant.IconName, IconImage: c.tenant.IconImage}
			if got := m.IconText(); got != c.want {
				t.Errorf("membership IconText() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTenantIconChoiceNamesTheTenant(t *testing.T) {
	text, name := "A", "rocket"
	cases := []struct {
		name string
		in   UpdateTenantInput
		want string
	}{
		{"two kinds", UpdateTenantInput{Icon: &text, IconName: &name}, "a tenant icon is one of"},
		{"unknown built-in", UpdateTenantInput{IconName: new("nope")}, "tenant icon name \"nope\""},
		{"too long", UpdateTenantInput{Icon: new("abc")}, "tenant icon \"abc\" is longer"},
		{"not an image", UpdateTenantInput{IconFile: &ProjectIconFile{Data: []byte("hello")}}, "tenant icon image is not"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.in.Validate()
			if !IsKind(err, KindInvalid) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Validate() = %v, want invalid naming %q", err, c.want)
			}
			if field := FieldOf(err); field != "icon" {
				t.Errorf("refusal is on field %q, want icon", field)
			}
		})
	}
	if err := (UpdateTenantInput{IconName: &name}).Validate(); err != nil {
		t.Errorf("one built-in icon refused: %v", err)
	}
}
