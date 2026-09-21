package config

import (
	"net/url"
	"regexp"
	"strings"
)

// Redacted replaces a secret value in displayed output.
const Redacted = "***"

// dsnKeyValueSecret matches a credential carried as a key and value in a
// connection string.
//
// The leading [a-z_]* is what catches a prefixed parameter such as
// sslpassword, which both PostgreSQL drivers accept: a word boundary alone sits
// between two word characters there and never matches. The value stops at & as
// well as whitespace and a semicolon, so redacting one query parameter does not
// swallow every parameter after it.
var dsnKeyValueSecret = regexp.MustCompile(`(?i)\b[a-z_]*(?:password|passwd|pwd|token|secret)\s*=\s*[^\s;&]+`)

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
