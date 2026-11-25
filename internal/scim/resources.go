// SPDX-License-Identifier: AGPL-3.0-or-later

package scim

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// Meta is a resource's metadata.
type Meta struct {
	ResourceType string `json:"resourceType"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Location     string `json:"location,omitempty"`
}

// Name is a user's name.
type Name struct {
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

// MultiValue is one entry of a multi-valued attribute such as emails.
type MultiValue struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

// User is the SCIM User resource.
type User struct {
	Schemas     []string     `json:"schemas"`
	ID          string       `json:"id"`
	ExternalID  string       `json:"externalId,omitempty"`
	UserName    string       `json:"userName"`
	Name        *Name        `json:"name,omitempty"`
	DisplayName string       `json:"displayName,omitempty"`
	Emails      []MultiValue `json:"emails,omitempty"`
	Active      bool         `json:"active"`
	Groups      []MultiValue `json:"groups,omitempty"`
	Meta        Meta         `json:"meta"`
}

// Group is the SCIM Group resource.
type Group struct {
	Schemas     []string     `json:"schemas"`
	ID          string       `json:"id"`
	ExternalID  string       `json:"externalId,omitempty"`
	DisplayName string       `json:"displayName"`
	Members     []MultiValue `json:"members,omitempty"`
	Meta        Meta         `json:"meta"`
}

// ListResponse is a page of resources.
type ListResponse[T any] struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    []T      `json:"Resources"`
}

// stamp renders an instant the way meta does, or nothing for zero.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// userOut renders a provisioned member under base, the endpoint's address.
func userOut(u core.SCIMUser, base string) User {
	out := User{
		Schemas: []string{SchemaUser}, ID: u.ID, ExternalID: u.ExternalID, UserName: u.UserName,
		DisplayName: u.DisplayName, Active: u.Active,
		Meta: Meta{ResourceType: "User", Created: stamp(u.Created), LastModified: stamp(u.Modified),
			Location: base + "/Users/" + u.ID},
	}
	if u.GivenName != "" || u.FamilyName != "" {
		out.Name = &Name{GivenName: u.GivenName, FamilyName: u.FamilyName,
			Formatted: strings.TrimSpace(u.GivenName + " " + u.FamilyName)}
	}
	if u.Email != "" {
		out.Emails = []MultiValue{{Value: u.Email, Type: "work", Primary: true}}
	}
	for _, g := range u.Groups {
		out.Groups = append(out.Groups, MultiValue{Value: g.ID, Display: g.Display, Ref: base + "/Groups/" + g.ID})
	}
	return out
}

// groupOut renders a group under base.
func groupOut(g core.SCIMGroup, base string) Group {
	out := Group{
		Schemas: []string{SchemaGroup}, ID: g.ID, ExternalID: g.ExternalID, DisplayName: g.DisplayName,
		Meta: Meta{ResourceType: "Group", Created: stamp(g.Created), LastModified: stamp(g.Modified),
			Location: base + "/Groups/" + g.ID},
	}
	for _, m := range g.Members {
		out.Members = append(out.Members, MultiValue{Value: m.ID, Display: m.Display, Ref: base + "/Users/" + m.ID})
	}
	return out
}

// userIn is a User as a provider sends it. Active is raw because providers
// send it as a boolean or as the string "True".
type userIn struct {
	ExternalID  string          `json:"externalId"`
	UserName    string          `json:"userName"`
	Name        *Name           `json:"name"`
	DisplayName string          `json:"displayName"`
	Emails      []emailIn       `json:"emails"`
	Active      json.RawMessage `json:"active"`
}

// emailIn is one email a provider sends; primary may be a string too.
type emailIn struct {
	Value   string          `json:"value"`
	Type    string          `json:"type"`
	Primary json.RawMessage `json:"primary"`
}

// pickEmail chooses the address a user is known by: the primary one, else
// the work one, else the first.
func pickEmail(emails []emailIn) string {
	for _, e := range emails {
		if on, err := parseBool(e.Primary, false); err == nil && on {
			return strings.TrimSpace(e.Value)
		}
	}
	for _, e := range emails {
		if strings.EqualFold(e.Type, "work") {
			return strings.TrimSpace(e.Value)
		}
	}
	if len(emails) > 0 {
		return strings.TrimSpace(emails[0].Value)
	}
	return ""
}

// parseBool reads a boolean sent as JSON true or false, or as a string
// spelling one in any case. Absent reads as def.
func parseBool(raw json.RawMessage, def bool) (bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return def, nil
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return false, badRequest(TypeInvalidValue, "active must be true or false")
}

// toUser reads a provider's User as the whole of a member's SCIM record.
func (in userIn) toUser() (core.SCIMUser, error) {
	active, err := parseBool(in.Active, true)
	if err != nil {
		return core.SCIMUser{}, err
	}
	out := core.SCIMUser{
		ExternalID: strings.TrimSpace(in.ExternalID), UserName: strings.TrimSpace(in.UserName),
		DisplayName: strings.TrimSpace(in.DisplayName), Email: pickEmail(in.Emails), Active: active,
	}
	if in.Name != nil {
		out.GivenName, out.FamilyName = strings.TrimSpace(in.Name.GivenName), strings.TrimSpace(in.Name.FamilyName)
	}
	return out, nil
}

// groupIn is a Group as a provider sends it.
type groupIn struct {
	ExternalID  string       `json:"externalId"`
	DisplayName string       `json:"displayName"`
	Members     []MultiValue `json:"members"`
}

// toGroup reads a provider's Group.
func (in groupIn) toGroup() core.SCIMGroup {
	out := core.SCIMGroup{ExternalID: strings.TrimSpace(in.ExternalID), DisplayName: strings.TrimSpace(in.DisplayName)}
	for _, m := range in.Members {
		out.Members = append(out.Members, core.SCIMRef{ID: strings.TrimSpace(m.Value)})
	}
	return out
}
