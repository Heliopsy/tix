// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// Origin is the address the links built from this request go under: the
// configured public address on a host that serves several tenants, and the
// request's own scheme and host on a tenant's own domain or when no public
// address is configured. The task screen's Share menu, the hand-off dialog,
// calendar feeds, invite links and the Connect screen all build from it.
func Origin(r *http.Request) string {
	if base := PublicOrigin(r); base != "" {
		return base
	}
	return SchemeFrom(r.Context()) + "://" + r.Host
}

// PublicOrigin is the configured public address when it applies to this
// request, or "". It does not apply on a host bound to one tenant: that host
// is the tenant's own address, and a link to it is a link to the tenant.
func PublicOrigin(r *http.Request) string {
	base, _ := r.Context().Value(ctxKeyPublicURL).(string)
	if base == "" {
		return ""
	}
	if served, ok := core.ServedHostFrom(r.Context()); ok && !served.Shared {
		return ""
	}
	return base
}

// WithPublicURL returns a context carrying the configured public address, as
// the router's middleware sets it on every request. Empty sets nothing.
func WithPublicURL(ctx context.Context, base string) context.Context {
	if base == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyPublicURL, base)
}
