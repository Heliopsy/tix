// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import "context"

// HeaderTenant names the tenant a request is for, on a host that serves
// several. A host bound to one tenant refuses it when it names another.
const HeaderTenant = "X-Tix-Tenant"

// LoginMethodSwitch is the sign-in method of a session minted by moving from
// another tenant's session on the same host.
const LoginMethodSwitch = "switch"

// SessionOrigin is how a session was minted: its sign-in method and whether
// that sign-in passed a second factor.
type SessionOrigin struct {
	// Method is LoginMethodPassword, LoginMethodSSHKey or LoginMethodSwitch.
	Method string
	// SecondFactor reports that the sign-in passed a second factor.
	SecondFactor bool
	// UserAgent is the user agent the sign-in request sent, truncated.
	UserAgent string
}

// ServedHost describes the host a request arrived on, as the transport
// resolved it once before any credential was examined.
type ServedHost struct {
	// Host is the request's hostname, lowercased and without its port.
	Host string
	// Shared reports that the host serves several tenants: no tenant domain
	// claims it, or the domain claiming it is marked shared.
	Shared bool
	// Owner is the tenant the host serves when neither a credential nor the
	// tenant header names one: the domain's tenant, or the default tenant.
	Owner string
	// Named reports that a credential or the tenant header chose the tenant,
	// rather than the host falling back to Owner.
	Named bool
}

type servedHostKey struct{}

// WithServedHost returns a context carrying the host a request arrived on.
// Only a transport sets it.
func WithServedHost(ctx context.Context, h ServedHost) context.Context {
	return context.WithValue(ctx, servedHostKey{}, h)
}

// ServedHostFrom returns the host a request arrived on, if a transport said.
func ServedHostFrom(ctx context.Context) (ServedHost, bool) {
	h, ok := ctx.Value(servedHostKey{}).(ServedHost)
	return h, ok
}

// Serves reports whether a tenant whose own address is host may be served on
// the shared host h: it is the host's owner, it has no address of its own, or
// its address is this host.
func (h ServedHost) Serves(tenantID, host string) bool {
	return h.Shared && (tenantID == h.Owner || host == "" || host == h.Host)
}

// SwitchTenantInput moves the caller's session to another tenant served on
// the same host.
type SwitchTenantInput struct {
	// Tenant is the key of the tenant to move to. Required.
	Tenant string `json:"tenant" yaml:"tenant"`
	// Code is a second factor code or a recovery code, sent again after a
	// code challenge when the target requires a factor this session lacks.
	Code string `json:"code,omitempty" yaml:"code,omitempty"`
	// Surface is the sign-in path the new session is for; empty is the web.
	Surface Surface `json:"surface,omitempty" yaml:"surface,omitempty"`
}

// UpdateDomainInput changes a tenant domain. A nil field is left as it is.
type UpdateDomainInput struct {
	// Shared marks the domain's host as serving several tenants: the tenant a
	// credential or the X-Tix-Tenant header names, and the domain's own
	// tenant otherwise.
	Shared *bool `json:"shared,omitempty" yaml:"shared,omitempty"`
}
