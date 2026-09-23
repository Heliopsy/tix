// SPDX-License-Identifier: AGPL-3.0-or-later

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Delivery request headers.
const (
	HeaderEvent     = "X-Tix-Event"
	HeaderDelivery  = "X-Tix-Delivery"
	HeaderTimestamp = "X-Tix-Timestamp"
	HeaderSignature = "X-Tix-Signature"
)

// SignaturePrefix names the algorithm carried by the signature header.
const SignaturePrefix = "sha256="

// TimestampLayout is the layout of the timestamp header.
const TimestampLayout = time.RFC3339Nano

// FormatTimestamp renders an instant for the timestamp header.
func FormatTimestamp(t time.Time) string { return t.UTC().Format(TimestampLayout) }

// SignedMaterial returns the bytes a signature covers: the timestamp and the
// body joined by a dot, so a captured body cannot be replayed under another
// timestamp.
func SignedMaterial(timestamp string, body []byte) []byte {
	out := make([]byte, 0, len(timestamp)+1+len(body))
	out = append(out, timestamp...)
	out = append(out, '.')
	return append(out, body...)
}

// Sign returns the signature header value for a body sent at a timestamp.
func Sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(SignedMaterial(timestamp, body))
	return SignaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

// Verify reports whether a signature matches the body and timestamp under the
// shared secret, comparing in constant time.
func Verify(secret, timestamp string, body []byte, signature string) bool {
	return hmac.Equal([]byte(Sign(secret, timestamp, body)), []byte(signature))
}
