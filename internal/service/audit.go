package service

import (
	"encoding/json"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// secretKeys are field names whose value never enters the audit log. Matching
// is on the lowercased key and is substring-based, so password_hash, token_hash
// and api_secret are all covered.
var secretKeys = []string{
	"password", "secret", "token", "hash", "credential", "private", "passphrase", "dsn",
}

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

// redact walks a decoded JSON value, replacing secret-looking fields.
func redact(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if isSecretKey(k) {
				out[k] = "[redacted]"
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
	default:
		return v
	}
}

func isSecretKey(k string) bool {
	lower := strings.ToLower(k)
	for _, s := range secretKeys {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
