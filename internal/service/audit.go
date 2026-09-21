package service

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// redacted is what replaces anything the audit log must not carry.
const redacted = "[redacted]"

// secretKeys are field names whose value never enters the audit log. Matching
// is on the lowercased key and is substring-based, so password_hash, token_hash
// and api_secret are all covered.
var secretKeys = []string{
	"password", "passwd", "secret", "token", "hash", "credential", "private",
	"passphrase", "dsn", "authorization", "cookie", "signature", "bearer", "salt",
}

// benignKeyNames are the field names that contain the word "key" and name an
// identifier rather than a credential. A project, workflow, tenant and custom
// field are all addressed by a key, so "key" on its own cannot be redacted.
var benignKeyNames = map[string]bool{
	"key": true, "keys": true, "new_key": true, "old_key": true,
	"project_key": true, "project_keys": true,
	"workflow_key": true, "workflow_keys": true,
	"tenant_key": true, "field_key": true, "sort_key": true, "key_path": true,
	"status_key": true, "state_key": true,
}

// credentialInURL matches the userinfo of a url, which is where a password
// reaches the audit log inside a value nobody thought of as a secret.
var credentialInURL = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)([^/\s:@]+):([^/\s@]*)@`)

// credentialInQuery matches a query parameter whose name marks its value as a
// credential, so a signed or tokenised url is stored without its token.
var credentialInQuery = regexp.MustCompile(`(?i)([?&][^=&\s]*(?:token|secret|password|passwd|key|auth|signature|sig|credential)[^=&\s]*=)([^&\s]+)`)

// authorizationValue matches an http authorization value, which arrives as a
// plain string in a header map rather than under a key of its own.
var authorizationValue = regexp.MustCompile(`(?i)\b(bearer|basic|token)\s+([A-Za-z0-9\-._~+/=]{8,})`)

// pemBlock matches an armoured private key pasted into a field.
var pemBlock = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`)

// redactedJSON marshals v for the audit log with secret-looking fields removed.
// The audit log is read by people and shipped to operators, so a credential
// landing in it is a disclosure even though the record itself is legitimate.
func redactedJSON(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, core.Internal("capturing audit state: %v", err)
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, core.Internal("capturing audit state: %v", err)
	}

	out, err := json.Marshal(redact(decoded))
	if err != nil {
		return nil, core.Internal("capturing audit state: %v", err)
	}
	return out, nil
}

// redact walks a decoded JSON value, replacing secret-looking fields and
// scrubbing the credentials that hide inside ordinary-looking values.
func redact(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if isSecretKey(k) {
				out[k] = redacted
				continue
			}
			out[k] = redact(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = redact(val)
		}
		return out
	case string:
		return redactValue(t)
	default:
		return v
	}
}

// redactValue removes the credentials a string can carry regardless of the
// field it arrived under. Key names alone are not enough: a webhook url with a
// password in its userinfo is a disclosure stored under the key "url".
func redactValue(s string) string {
	if s == "" {
		return s
	}
	s = pemBlock.ReplaceAllString(s, redacted)
	s = credentialInURL.ReplaceAllString(s, "${1}${2}:"+redacted+"@")
	s = credentialInQuery.ReplaceAllString(s, "${1}"+redacted)
	s = authorizationValue.ReplaceAllString(s, "${1} "+redacted)
	return s
}

// isSecretKey reports whether a field name marks its value as a credential.
func isSecretKey(k string) bool {
	lower := strings.ToLower(k)
	for _, s := range secretKeys {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return isSecretKeyName(lower)
}

// isSecretKeyName decides the "key" family, which cannot be matched by
// substring because a project, workflow and tenant are each named by one.
func isSecretKeyName(lower string) bool {
	if !strings.Contains(lower, "key") {
		return false
	}
	return !benignKeyNames[normalizeKeyName(lower)]
}

// normalizeKeyName renders a field name as lowercase words joined by
// underscores, so apiKey, api-key and API_KEY are one name.
func normalizeKeyName(lower string) string {
	var b strings.Builder
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return strings.Trim(b.String(), "_")
}
