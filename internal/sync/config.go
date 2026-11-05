// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// EnvPrefix is the environment namespace external source configuration is read
// from. Credentials live here and nowhere else: no credential on SourceConfig
// is ever written to the store, a snapshot, an audit entry, an event or an
// error. A setting stored on the source takes precedence over its variable.
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

// settingEnv maps a stored setting to the environment suffix it stands in for.
// No credential suffix is listed, so a stored value can never supply one.
var settingEnv = map[string]string{
	core.SyncSettingURL:      EnvURL,
	core.SyncSettingProject:  EnvProject,
	core.SyncSettingQuery:    EnvQuery,
	core.SyncSettingFile:     EnvFile,
	core.SyncSettingPageSize: EnvPageSize,
}

// SettingEnv returns the environment suffix a stored setting replaces.
func SettingEnv(key string) (string, bool) {
	suffix, ok := settingEnv[key]
	return suffix, ok
}

// StoredFirst returns an environment reader that answers a source's stored
// settings for the variables they replace and getenv for everything else, so
// a stored value wins and the environment fills what the source leaves empty.
func StoredFirst(src core.SyncSource, getenv func(string) string) func(string) string {
	stored := map[string]string{}
	for key, value := range src.Config {
		if suffix, ok := settingEnv[key]; ok && strings.TrimSpace(value) != "" {
			stored[EnvName(src.Name, suffix)] = value
		}
	}
	if strings.TrimSpace(src.MappingPath) != "" {
		stored[EnvName(src.Name, EnvMapping)] = src.MappingPath
	}
	return func(name string) string {
		if value, ok := stored[name]; ok {
			return value
		}
		if getenv == nil {
			return ""
		}
		return getenv(name)
	}
}

// WithCredentials returns a copy carrying the given credentials, for a caller
// that resolves them outside the environment.
func (c SourceConfig) WithCredentials(token, user, password string) SourceConfig {
	c.token, c.user, c.password = token, user, password
	return c
}
