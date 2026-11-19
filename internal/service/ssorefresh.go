// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sso"
	"github.com/heliopsy/tix/internal/store"
)

// auditSSOProviderRefresh records a metadata refresh that changed what a
// provider trusts.
const auditSSOProviderRefresh = "sso.provider.refresh"

// samlRefreshErrorMax bounds the failure text kept on a provider.
const samlRefreshErrorMax = 500

// RefreshSAMLMetadata reads a SAML provider's IdP metadata again from its
// stored address and merges what changed. A failed read is recorded and
// returned, and the provider keeps the settings it had.
func (l *Local) RefreshSAMLMetadata(ctx context.Context, ref string) (*core.SSOProvider, error) {
	actor, err := l.authorize(ctx, authz.ActionSSOAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var before *core.SSOProvider
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		before, err = lookupSSOProvider(ctx, tx, ref)
		return err
	}); err != nil {
		return nil, err
	}
	if before.Kind != core.SSOKindSAML || before.IdPMetadataURL == "" {
		return nil, core.OnField(core.Precondition("provider %q has no IdP metadata URL to read; set one with "+
			"tix tenant sso edit %s --idp-metadata-url URL", before.Key, before.Key), "idp_metadata_url")
	}
	md, readErr := l.readIdPMetadata(ctx, samlMetadataSource{addr: before.IdPMetadataURL}, before.IdPEntityID)
	var out core.SSOProvider
	err = l.write(ctx, actor, func(m *mutation) error {
		current, err := m.tx.GetSSOProvider(ctx, before.ID)
		if err != nil {
			return err
		}
		if current.IdPMetadataURL != before.IdPMetadataURL {
			out = *current
			return nil
		}
		next, changed := refreshedProvider(*current, md, readErr, m.now)
		if readErr == nil && next.MetadataRefresh.Result == core.SAMLRefreshFailed {
			readErr = core.Invalid("%s", next.MetadataRefresh.Error)
		}
		out = next
		if !changed {
			return m.tx.RecordSSOProviderRefresh(ctx, &next)
		}
		if err := m.tx.UpdateSSOProvider(ctx, &next); err != nil {
			return err
		}
		return m.Record(auditSSOProviderRefresh, eventSSOProviderUpdated, subjectSSOProvider, next.ID, "",
			current.Redacted(), next.Redacted(), map[string]any{"key": next.Key, "kind": string(next.Kind),
				"certificates_added": next.MetadataRefresh.Added, "certificates_removed": next.MetadataRefresh.Removed})
	})
	if err != nil {
		return nil, err
	}
	if readErr != nil {
		return nil, readErr
	}
	l.withAddresses(ctx, &out)
	return &out, nil
}

// refreshedProvider is p after a read of its metadata: the settings an admin
// did not pin taken from md, the certificates merged, and the outcome
// recorded. A failed read, or metadata that would leave the provider
// invalid, changes nothing but the outcome.
func refreshedProvider(p core.SSOProvider, md *sso.IdPMetadata, readErr error, now time.Time,
) (core.SSOProvider, bool) {
	status := &core.SAMLMetadataRefresh{At: now, Result: core.SAMLRefreshUnchanged}
	failed := func(err error) (core.SSOProvider, bool) {
		status.Result, status.Error = core.SAMLRefreshFailed, refreshErrorText(err)
		p.MetadataRefresh = status
		return p, false
	}
	if readErr != nil {
		return failed(readErr)
	}
	if md == nil || len(md.Certificates) == 0 {
		return failed(core.Invalid("the IdP metadata names no signing certificate"))
	}
	next := p
	if !p.Pinned(core.PinIdPEntityID) && md.EntityID != "" {
		next.IdPEntityID = md.EntityID
	}
	if !p.Pinned(core.PinIdPSSOURL) && md.SSOURL != "" {
		next.IdPSSOURL = md.SSOURL
	}
	if !p.Pinned(core.PinIdPCertificates) {
		next.IdPCertificates, next.IdPRetiring = mergeIdPCertificates(p.IdPCertificates, p.IdPRetiring,
			md.Certificates, now)
	}
	if err := next.Validate(); err != nil {
		return failed(err)
	}
	status.Added = countMissing(next.IdPCertificates, p.IdPCertificates)
	status.Removed = countMissing(p.IdPCertificates, next.IdPCertificates)
	changed := status.Added > 0 || status.Removed > 0 || next.IdPEntityID != p.IdPEntityID ||
		next.IdPSSOURL != p.IdPSSOURL
	if changed {
		status.Result = core.SAMLRefreshUpdated
	}
	next.MetadataRefresh = status
	return next, changed
}

// mergeIdPCertificates is the certificate set after a refresh: every
// certificate the metadata publishes, then each one it stopped publishing
// until its overlap ends.
func mergeIdPCertificates(current []string, retiring []core.RetiringCertificate, published []string, now time.Time,
) ([]string, []core.RetiringCertificate) {
	certs := make([]string, 0, len(published)+len(current))
	seen := map[string]bool{}
	for _, c := range published {
		fp := core.CertificateFingerprint(c)
		if fp == "" || seen[fp] {
			continue
		}
		seen[fp] = true
		certs = append(certs, core.NormalizeCertificate(c))
	}
	var kept []core.RetiringCertificate
	for _, c := range current {
		fp := core.CertificateFingerprint(c)
		if fp == "" || seen[fp] {
			continue
		}
		seen[fp] = true
		r := core.RetiringCertificate{Fingerprint: fp, Since: now, Until: now.Add(core.SAMLCertificateOverlap)}
		if i := slices.IndexFunc(retiring, func(r core.RetiringCertificate) bool { return r.Fingerprint == fp }); i >= 0 {
			r = retiring[i]
		}
		if now.Before(r.Until) {
			certs = append(certs, c)
			kept = append(kept, r)
		}
	}
	return certs, kept
}

// countMissing counts the certificates of a that b does not hold.
func countMissing(a, b []string) int {
	have := map[string]bool{}
	for _, c := range b {
		have[core.CertificateFingerprint(c)] = true
	}
	n := 0
	for _, c := range a {
		if !have[core.CertificateFingerprint(c)] {
			n++
		}
	}
	return n
}

// sameCertificates reports whether a and b hold the same certificates.
func sameCertificates(a, b []string) bool {
	return countMissing(a, b) == 0 && countMissing(b, a) == 0
}

// refreshErrorText is a failed refresh as a provider records it.
func refreshErrorText(err error) string {
	text := strings.TrimSpace(err.Error())
	if len(text) > samlRefreshErrorMax {
		text = text[:samlRefreshErrorMax]
	}
	return text
}

// withRefreshSchedule dates the next scheduled read of a provider's
// metadata, when the server reads it on a schedule.
func (l *Local) withRefreshSchedule(p *core.SSOProvider) {
	if p.MetadataRefresh == nil {
		return
	}
	status := *p.MetadataRefresh
	status.Next = nil
	if l.samlRefresh > 0 && p.RefreshesMetadata() {
		wait := l.samlRefresh
		if status.Result == core.SAMLRefreshFailed {
			wait = min(wait, core.SAMLRefreshRetry)
		}
		next := status.At.Add(wait)
		status.Next = &next
	}
	p.MetadataRefresh = &status
}

// samlPins works out which IdP settings an edit leaves pinned. A value given
// that differs from the stored one, or that was already pinned, is the
// admin's; one given empty, or not given while metadata is read on this
// save, is the metadata's.
func samlPins(existing core.SSOProvider, in core.UpdateSSOProviderInput, metadata bool,
) (entity, ssoURL, certs bool, pins []string) {
	given := func(field string, v *string, stored string) bool {
		return v != nil && strings.TrimSpace(*v) != "" && (strings.TrimSpace(*v) != stored || existing.Pinned(field))
	}
	entity = given(core.PinIdPEntityID, in.IdPEntityID, existing.IdPEntityID)
	ssoURL = given(core.PinIdPSSOURL, in.IdPSSOURL, existing.IdPSSOURL)
	certs = in.IdPCertificates != nil && len(*in.IdPCertificates) > 0 &&
		(!sameCertificates(*in.IdPCertificates, existing.IdPCertificates) || existing.Pinned(core.PinIdPCertificates))
	keep := func(field string, explicit, cleared bool) {
		if explicit || (!cleared && !metadata && existing.Pinned(field)) {
			pins = append(pins, field)
		}
	}
	keep(core.PinIdPEntityID, entity, in.IdPEntityID != nil && strings.TrimSpace(*in.IdPEntityID) == "")
	keep(core.PinIdPSSOURL, ssoURL, in.IdPSSOURL != nil && strings.TrimSpace(*in.IdPSSOURL) == "")
	keep(core.PinIdPCertificates, certs, in.IdPCertificates != nil && len(*in.IdPCertificates) == 0)
	return entity, ssoURL, certs, pins
}

// createPins names the IdP settings a new provider was given explicitly.
func createPins(in core.CreateSSOProviderInput) []string {
	var pins []string
	if strings.TrimSpace(in.IdPEntityID) != "" {
		pins = append(pins, core.PinIdPEntityID)
	}
	if strings.TrimSpace(in.IdPSSOURL) != "" {
		pins = append(pins, core.PinIdPSSOURL)
	}
	if len(in.IdPCertificates) > 0 {
		pins = append(pins, core.PinIdPCertificates)
	}
	return pins
}

// keepRetiring drops the retiring marks of certificates p no longer holds,
// and every mark once an admin pinned the certificates.
func keepRetiring(p *core.SSOProvider) {
	if p.Pinned(core.PinIdPCertificates) {
		p.IdPRetiring = nil
		return
	}
	have := map[string]bool{}
	for _, c := range p.IdPCertificates {
		have[core.CertificateFingerprint(c)] = true
	}
	p.IdPRetiring = slices.DeleteFunc(slices.Clone(p.IdPRetiring), func(r core.RetiringCertificate) bool {
		return !have[r.Fingerprint]
	})
	if len(p.IdPRetiring) == 0 {
		p.IdPRetiring = nil
	}
}

// editedMetadata settles the refresh state an edit leaves: metadata read on
// save is a refresh, the certificates it replaced retire at once, and a
// provider without a metadata address has nothing to refresh.
func editedMetadata(existing *core.SSOProvider, next *core.SSOProvider, read bool, now time.Time) {
	switch {
	case next.IdPMetadataURL == "":
		next.MetadataRefresh, next.IdPRetiring = nil, nil
		return
	case read:
		status := &core.SAMLMetadataRefresh{At: now, Result: core.SAMLRefreshUnchanged,
			Added:   countMissing(next.IdPCertificates, existing.IdPCertificates),
			Removed: countMissing(existing.IdPCertificates, next.IdPCertificates)}
		if status.Added > 0 || status.Removed > 0 || next.IdPEntityID != existing.IdPEntityID ||
			next.IdPSSOURL != existing.IdPSSOURL {
			status.Result = core.SAMLRefreshUpdated
		}
		next.MetadataRefresh = status
	}
	keepRetiring(next)
}
