// SPDX-License-Identifier: AGPL-3.0-or-later

// Package retention decides what has expired and prunes it on a schedule.
package retention

import (
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// Class names one retention-governed table.
type Class string

// Retention classes. Each is governed independently of the others.
const (
	ClassEvents     Class = "events"
	ClassAudit      Class = "audit_entries"
	ClassDeliveries Class = "webhook_deliveries"
)

// Classes returns every retention class in a stable order.
func Classes() []Class { return []Class{ClassEvents, ClassAudit, ClassDeliveries} }

// Window returns the window the policy sets for a class, and whether the class
// is known.
func Window(p core.RetentionPolicy, c Class) (time.Duration, bool) {
	switch c {
	case ClassEvents:
		return p.Events.D(), true
	case ClassAudit:
		return p.AuditEntries.D(), true
	case ClassDeliveries:
		return p.WebhookDeliveries.D(), true
	default:
		return 0, false
	}
}

// Cutoff returns the instant before which records of a class have expired.
func Cutoff(p core.RetentionPolicy, c Class, now time.Time) (time.Time, bool) {
	w, ok := Window(Effective(p), c)
	if !ok {
		return time.Time{}, false
	}
	return now.Add(-w).UTC(), true
}

// Cutoffs returns the cutoff for every class under one policy.
func Cutoffs(p core.RetentionPolicy, now time.Time) map[Class]time.Time {
	out := make(map[Class]time.Time, len(Classes()))
	for _, c := range Classes() {
		if at, ok := Cutoff(p, c, now); ok {
			out[c] = at
		}
	}
	return out
}

// Validate rejects a policy the store must not be allowed to hold.
func Validate(p core.RetentionPolicy) error {
	for _, f := range []struct {
		name string
		d    core.Duration
	}{
		{"events", p.Events},
		{"audit_entries", p.AuditEntries},
		{"webhook_deliveries", p.WebhookDeliveries},
	} {
		if f.d < 0 {
			return core.Invalid("retention window for %s must not be negative", f.name)
		}
	}
	return nil
}

// Effective fills unset windows from the shipped default.
func Effective(p core.RetentionPolicy) core.RetentionPolicy {
	def := core.DefaultRetention(p.TenantID)
	if p.Events <= 0 {
		p.Events = def.Events
	}
	if p.AuditEntries <= 0 {
		p.AuditEntries = def.AuditEntries
	}
	if p.WebhookDeliveries <= 0 {
		p.WebhookDeliveries = def.WebhookDeliveries
	}
	return p
}

// Resolve layers a configured default under a tenant's stored policy. A window
// the tenant set explicitly wins, because the stored policy is per tenant and
// authoritative for a shared deployment; configuration only supplies the
// windows the tenant never moved off the shipped default.
func Resolve(stored, configured core.RetentionPolicy) core.RetentionPolicy {
	def := core.DefaultRetention(stored.TenantID)
	out := Effective(stored)
	for _, f := range []struct {
		stored     core.Duration
		configured core.Duration
		def        core.Duration
		dst        *core.Duration
	}{
		{stored.Events, configured.Events, def.Events, &out.Events},
		{stored.AuditEntries, configured.AuditEntries, def.AuditEntries, &out.AuditEntries},
		{stored.WebhookDeliveries, configured.WebhookDeliveries, def.WebhookDeliveries, &out.WebhookDeliveries},
	} {
		if f.stored > 0 && f.stored != f.def {
			continue
		}
		if f.configured > 0 {
			*f.dst = f.configured
		}
	}
	return out
}

// IsDefault reports whether every window of p equals the shipped default.
func IsDefault(p core.RetentionPolicy) bool {
	def := core.DefaultRetention(p.TenantID)
	e := Effective(p)
	return e.Events == def.Events &&
		e.AuditEntries == def.AuditEntries &&
		e.WebhookDeliveries == def.WebhookDeliveries
}

// Merge overlays the windows next actually sets onto current, so changing one
// class never disturbs another.
func Merge(current, next core.RetentionPolicy) core.RetentionPolicy {
	out := current
	if next.TenantID != "" {
		out.TenantID = next.TenantID
	}
	if next.Events > 0 {
		out.Events = next.Events
	}
	if next.AuditEntries > 0 {
		out.AuditEntries = next.AuditEntries
	}
	if next.WebhookDeliveries > 0 {
		out.WebhookDeliveries = next.WebhookDeliveries
	}
	return out
}
