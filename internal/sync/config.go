package sync

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/thereisnotime/tix/internal/core"
)

// EnvPrefix is the environment namespace external source configuration is read
// from. Credentials live here and nowhere else: nothing on this struct is ever
// written to the store, a snapshot, an audit entry, an event or an error.
const EnvPrefix = "TIX_SYNC_"

// Environment suffixes, one per configurable setting.
const (
	EnvMapping  = "MAPPING"
	EnvFile     = "FILE"
	EnvURL      = "URL"
	EnvToken    = "TOKEN"
	EnvUser     = "USER"
	EnvPassword = "PASSWORD"
	EnvQuery    = "QUERY"
	EnvProject  = "PROJECT"
	EnvPageSize = "PAGE_SIZE"
)

var envSanitizer = regexp.MustCompile(`[^A-Z0-9]+`)

// SourceConfig is everything an adapter needs to reach its external system.
type SourceConfig struct {
	MappingPath string
	File        string
	BaseURL     string
	Query       string
	Project     string
	PageSize    int

	token    string
	user     string
	password string
}

// Token returns the bearer credential, which no other accessor discloses.
func (c SourceConfig) Token() string { return c.token }

// BasicAuth returns the user and password credential pair.
func (c SourceConfig) BasicAuth() (string, string) { return c.user, c.password }

// HasCredential reports whether any credential was supplied.
func (c SourceConfig) HasCredential() bool {
	return c.token != "" || c.user != "" || c.password != ""
}

// Redacted describes the configuration without any credential material. It is
// the only representation that may be logged, audited or rendered.
func (c SourceConfig) Redacted() map[string]any {
	out := map[string]any{"credential": "unset"}
	if c.HasCredential() {
		out["credential"] = "[redacted]"
	}
	for k, v := range map[string]string{
		"mapping_path": c.MappingPath,
		"file":         c.File,
		"base_url":     c.BaseURL,
		"query":        c.Query,
		"project":      c.Project,
	} {
		if v != "" {
			out[k] = v
		}
	}
	if c.PageSize > 0 {
		out["page_size"] = c.PageSize
	}
	return out
}

// String renders the configuration without its credentials, so an accidental
// interpolation into a log line or an error cannot disclose one.
func (c SourceConfig) String() string {
	var b strings.Builder
	b.WriteString("sync source config{")
	for i, k := range sortedKeys(c.Redacted()) {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(Text(c.Redacted()[k]))
	}
	b.WriteString("}")
	return b.String()
}

// MarshalJSON writes the redacted form, so a config can never be serialized
// into a snapshot or an event payload with its credentials intact.
func (c SourceConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Redacted())
}

// MarshalYAML writes the redacted form.
func (c SourceConfig) MarshalYAML() (any, error) { return c.Redacted(), nil }

// EnvName returns the environment variable a source's setting is read from.
func EnvName(sourceName, suffix string) string {
	slug := envSanitizer.ReplaceAllString(strings.ToUpper(sourceName), "_")
	return EnvPrefix + strings.Trim(slug, "_") + "_" + suffix
}

// LoadSourceConfig reads a source's configuration from the environment.
func LoadSourceConfig(sourceName string, getenv func(string) string) (SourceConfig, error) {
	if getenv == nil {
		return SourceConfig{}, core.Internal("no environment reader supplied")
	}
	read := func(suffix string) string {
		return strings.TrimSpace(getenv(EnvName(sourceName, suffix)))
	}
	cfg := SourceConfig{
		MappingPath: read(EnvMapping),
		File:        read(EnvFile),
		BaseURL:     strings.TrimRight(read(EnvURL), "/"),
		Query:       read(EnvQuery),
		Project:     read(EnvProject),
		token:       read(EnvToken),
		user:        read(EnvUser),
		password:    read(EnvPassword),
	}
	if raw := read(EnvPageSize); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return SourceConfig{}, core.Invalid("%s must be a positive integer",
				EnvName(sourceName, EnvPageSize))
		}
		cfg.PageSize = n
	}
	return cfg, nil
}

// WithCredentials returns a copy carrying the given credentials, for a caller
// that resolves them outside the environment.
func (c SourceConfig) WithCredentials(token, user, password string) SourceConfig {
	c.token, c.user, c.password = token, user, password
	return c
}
