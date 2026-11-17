// SPDX-License-Identifier: AGPL-3.0-or-later

package scim

// MaxResults is the most resources one list answers, and the default count.
const MaxResults = 200

// DefaultCount is the page size when a provider names none.
const DefaultCount = 100

// supported is a feature flag of the service provider configuration.
type supported struct {
	Supported bool `json:"supported"`
}

// ServiceProviderConfig describes what tix supports, RFC 7643 section 5.
type ServiceProviderConfig struct {
	Schemas               []string     `json:"schemas"`
	DocumentationURI      string       `json:"documentationUri"`
	Patch                 supported    `json:"patch"`
	Bulk                  bulkConfig   `json:"bulk"`
	Filter                filterConfig `json:"filter"`
	ChangePassword        supported    `json:"changePassword"`
	Sort                  supported    `json:"sort"`
	ETag                  supported    `json:"etag"`
	AuthenticationSchemes []authScheme `json:"authenticationSchemes"`
	Meta                  Meta         `json:"meta"`
}

type bulkConfig struct {
	Supported      bool `json:"supported"`
	MaxOperations  int  `json:"maxOperations"`
	MaxPayloadSize int  `json:"maxPayloadSize"`
}

type filterConfig struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

type authScheme struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Primary     bool   `json:"primary"`
}

// spConfig is the configuration tix answers with.
func spConfig(base string) ServiceProviderConfig {
	return ServiceProviderConfig{
		Schemas:          []string{SchemaSPConfig},
		DocumentationURI: "https://tix.red/docs/scim",
		Patch:            supported{Supported: true},
		Bulk:             bulkConfig{},
		Filter:           filterConfig{Supported: true, MaxResults: MaxResults},
		AuthenticationSchemes: []authScheme{{
			Type: "oauthbearertoken", Name: "Bearer token",
			Description: "A tix API token carrying scim:provision, minted by tix tenant scim enable.",
			Primary:     true,
		}},
		Meta: Meta{ResourceType: "ServiceProviderConfig", Location: base + "/ServiceProviderConfig"},
	}
}

// ResourceType describes one resource type, RFC 7643 section 6.
type ResourceType struct {
	Schemas     []string `json:"schemas"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Endpoint    string   `json:"endpoint"`
	Description string   `json:"description"`
	Schema      string   `json:"schema"`
	Meta        Meta     `json:"meta"`
}

// resourceTypes are the User and Group types.
func resourceTypes(base string) []ResourceType {
	return []ResourceType{
		{Schemas: []string{SchemaResourceType}, ID: "User", Name: "User", Endpoint: "/Users",
			Description: "A member of the tenant", Schema: SchemaUser,
			Meta: Meta{ResourceType: "ResourceType", Location: base + "/ResourceTypes/User"}},
		{Schemas: []string{SchemaResourceType}, ID: "Group", Name: "Group", Endpoint: "/Groups",
			Description: "A group of the tenant's members, which can be mapped to a role", Schema: SchemaGroup,
			Meta: Meta{ResourceType: "ResourceType", Location: base + "/ResourceTypes/Group"}},
	}
}

// Attribute describes one attribute of a schema.
type Attribute struct {
	Name          string      `json:"name"`
	Type          string      `json:"type"`
	MultiValued   bool        `json:"multiValued"`
	Required      bool        `json:"required"`
	CaseExact     bool        `json:"caseExact"`
	Mutability    string      `json:"mutability"`
	Returned      string      `json:"returned"`
	Uniqueness    string      `json:"uniqueness"`
	SubAttributes []Attribute `json:"subAttributes,omitempty"`
}

// Schema describes a resource's attributes, RFC 7643 section 7.
type Schema struct {
	Schemas     []string    `json:"schemas"`
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Attributes  []Attribute `json:"attributes"`
	Meta        Meta        `json:"meta"`
}

// attr builds a single-valued, read-write, case-insensitive string attribute.
func attr(name string) Attribute {
	return Attribute{Name: name, Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}
}

// schemas are the User and Group schemas, as far as tix keeps them.
func schemas(base string) []Schema {
	userName := attr("userName")
	userName.Required, userName.Uniqueness = true, "server"
	active := attr("active")
	active.Type = "boolean"
	email := attr("emails")
	email.Type, email.MultiValued = "complex", true
	email.SubAttributes = []Attribute{attr("value"), attr("type"), {Name: "primary", Type: "boolean",
		Mutability: "readWrite", Returned: "default", Uniqueness: "none"}}
	name := attr("name")
	name.Type = "complex"
	name.SubAttributes = []Attribute{attr("formatted"), attr("givenName"), attr("familyName")}
	groups := attr("groups")
	groups.Type, groups.MultiValued, groups.Mutability = "complex", true, "readOnly"
	groups.SubAttributes = []Attribute{attr("value"), attr("display"), attr("$ref")}
	members := attr("members")
	members.Type, members.MultiValued = "complex", true
	members.SubAttributes = []Attribute{attr("value"), attr("display"), attr("$ref")}
	displayName := attr("displayName")
	groupName := attr("displayName")
	groupName.Required = true
	externalID := attr("externalId")
	externalID.CaseExact = true
	return []Schema{
		{Schemas: []string{SchemaSchema}, ID: SchemaUser, Name: "User", Description: "User account",
			Attributes: []Attribute{userName, externalID, name, displayName, email, active, groups},
			Meta:       Meta{ResourceType: "Schema", Location: base + "/Schemas/" + SchemaUser}},
		{Schemas: []string{SchemaSchema}, ID: SchemaGroup, Name: "Group", Description: "Group",
			Attributes: []Attribute{groupName, externalID, members},
			Meta:       Meta{ResourceType: "Schema", Location: base + "/Schemas/" + SchemaGroup}},
	}
}
