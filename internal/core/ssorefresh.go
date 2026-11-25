// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"slices"
	"time"
)

// Outcomes of reading a SAML provider's IdP metadata again.
const (
	SAMLRefreshUpdated   = "updated"
	SAMLRefreshUnchanged = "unchanged"
	SAMLRefreshFailed    = "failed"
)

// The IdP settings a metadata refresh fills, as IdPPinned names them.
const (
	PinIdPEntityID     = "idp_entity_id"
	PinIdPSSOURL       = "idp_sso_url"
	PinIdPCertificates = "idp_certificates"
)

// DefaultSAMLMetadataRefresh is how often tix serve reads a SAML provider's
// IdP metadata again, auth.saml_metadata_refresh.
const DefaultSAMLMetadataRefresh = Duration(12 * time.Hour)

// SAMLCertificateOverlap is how long a certificate the IdP's metadata no
// longer names keeps verifying, so a rollover in flight still signs in.
const SAMLCertificateOverlap = 7 * 24 * time.Hour

// SAMLRefreshRetry is how soon a failed refresh is tried again, when the
// refresh interval is longer.
const SAMLRefreshRetry = time.Hour

// SAMLMetadataRefresh is the outcome of the last read of a SAML provider's
// IdP metadata, on save or on schedule.
type SAMLMetadataRefresh struct {
	// At is when the metadata was last read.
	At time.Time `json:"at" yaml:"at"`
	// Result is updated, unchanged or failed.
	Result string `json:"result" yaml:"result"`
	// Error is why the read failed; the stored settings stayed.
	Error string `json:"error,omitempty" yaml:"error,omitempty"`
	// Added and Removed count the signing certificates the read changed.
	Added   int `json:"added,omitempty" yaml:"added,omitempty"`
	Removed int `json:"removed,omitempty" yaml:"removed,omitempty"`
	// Next is when the server reads it again; absent when refresh is off.
	Next *time.Time `json:"next,omitempty" yaml:"next,omitempty"`
}

// RetiringCertificate is a signing certificate the IdP's metadata stopped
// naming, kept until Until.
type RetiringCertificate struct {
	// Fingerprint is the SHA-256 of the certificate, in hex.
	Fingerprint string `json:"fingerprint" yaml:"fingerprint"`
	// Since is the first refresh that found it gone from the metadata.
	Since time.Time `json:"since" yaml:"since"`
	// Until is when it stops being trusted.
	Until time.Time `json:"until" yaml:"until"`
}

// CertificateFingerprint is the SHA-256 of a PEM certificate, in hex, or
// empty when it is not one.
func CertificateFingerprint(text string) string {
	block, _ := pem.Decode([]byte(NormalizeCertificate(text)))
	if block == nil {
		return ""
	}
	sum := sha256.Sum256(block.Bytes)
	return hex.EncodeToString(sum[:])
}

// Pinned reports whether an admin set the IdP setting named field.
func (p SSOProvider) Pinned(field string) bool { return slices.Contains(p.IdPPinned, field) }

// RefreshesMetadata reports whether the scheduled refresh reads this
// provider: an enabled SAML provider with an IdP metadata address.
func (p SSOProvider) RefreshesMetadata() bool {
	return p.Kind == SSOKindSAML && p.Enabled && p.IdPMetadataURL != ""
}

// MetadataRefreshDue reports whether a refresh every interval should read
// the provider's metadata at now. A failed read is retried after
// SAMLRefreshRetry when that is sooner.
func (p SSOProvider) MetadataRefreshDue(now time.Time, interval time.Duration) bool {
	if interval <= 0 || !p.RefreshesMetadata() {
		return false
	}
	last := p.MetadataRefresh
	if last == nil || last.At.IsZero() {
		return true
	}
	wait := interval
	if last.Result == SAMLRefreshFailed {
		wait = min(interval, SAMLRefreshRetry)
	}
	return !now.Before(last.At.Add(wait))
}
