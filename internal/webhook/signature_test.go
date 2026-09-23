// SPDX-License-Identifier: AGPL-3.0-or-later

package webhook

import (
	"strings"
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	const secret = "top-secret"
	timestamp := FormatTimestamp(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	body := []byte(`{"id":"e1","type":"task.created"}`)
	sig := Sign(secret, timestamp, body)

	if !strings.HasPrefix(sig, SignaturePrefix) {
		t.Fatalf("signature %q lacks the algorithm prefix", sig)
	}
	if string(SignedMaterial(timestamp, body)) != timestamp+"."+string(body) {
		t.Fatal("signed material must be the timestamp and body joined by a dot")
	}

	cases := []struct {
		name      string
		secret    string
		timestamp string
		body      []byte
		signature string
		want      bool
	}{
		{"exact", secret, timestamp, body, sig, true},
		{"tampered body", secret, timestamp, []byte(`{"id":"e1","type":"task.deleted"}`), sig, false},
		{"truncated body", secret, timestamp, body[:len(body)-1], sig, false},
		{"replayed under a new timestamp", secret, FormatTimestamp(time.Date(2026, 1, 1, 0, 5, 0, 0, time.UTC)), body, sig, false},
		{"wrong secret", "guess", timestamp, body, sig, false},
		{"empty signature", secret, timestamp, body, "", false},
		{"missing prefix", secret, timestamp, body, strings.TrimPrefix(sig, SignaturePrefix), false},
		{"wrong length", secret, timestamp, body, sig + "00", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Verify(c.secret, c.timestamp, c.body, c.signature); got != c.want {
				t.Fatalf("Verify = %v, want %v", got, c.want)
			}
		})
	}
}

func TestSignIsStableAndTimestampIsUTC(t *testing.T) {
	body := []byte("payload")
	ts := FormatTimestamp(time.Date(2026, 3, 1, 12, 0, 0, 0, time.FixedZone("east", 2*60*60)))
	if !strings.HasSuffix(ts, "Z") {
		t.Fatalf("timestamp %q is not utc", ts)
	}
	first, second := Sign("s", ts, body), Sign("s", ts, append([]byte(nil), body...))
	if first != second {
		t.Fatal("signing is not deterministic")
	}
	if first == Sign("t", ts, body) {
		t.Fatal("different secrets produced the same signature")
	}
}
