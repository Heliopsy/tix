package config

import (
	"net/url"
	"regexp"
	"strings"
)

// Redacted replaces a secret value in displayed output.
const Redacted = "***"

var dsnKeyValueSecret = regexp.MustCompile(`(?i)\b(password|passwd|pwd|token|secret)\s*=\s*[^\s;]+`)

func redact(level secrecy, value string) string {
	switch {
	case value == "":
		return ""
	case level == secretOpaque:
		return Redacted
	case level == secretDSN:
		return RedactDSN(value)
	default:
		return value
	}
}

// RedactDSN removes credentials from a connection string while keeping it readable.
func RedactDSN(dsn string) string {
	out := dsn
	if parsed, err := url.Parse(dsn); err == nil && parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(parsed.User.Username(), Redacted)
			out = decodeRedaction(parsed.String())
		}
	}
	return dsnKeyValueSecret.ReplaceAllStringFunc(out, func(match string) string {
		name, _, _ := strings.Cut(match, "=")
		return strings.TrimSpace(name) + "=" + Redacted
	})
}

func decodeRedaction(s string) string {
	return strings.Replace(s, url.QueryEscape(Redacted), Redacted, 1)
}
