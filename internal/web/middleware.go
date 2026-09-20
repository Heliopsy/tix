package web

import (
	"net/http"
	"net/url"

	"github.com/thereisnotime/tix/internal/core"
)

// tenantParams are parameter names a caller might use to aim a request at
// another tenant. The interface takes the tenant from the session only.
var tenantParams = []string{"tenant", "tenant_id", "tenant_key"}

// wrap gives a route its authentication, tenant pinning and CSRF checks.
func (h *handler) wrap(rt route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(core.WithSource(r.Context(), core.SourceWeb))
		if _, ok := core.ActorFrom(r.Context()); !ok && !rt.Public {
			redirectToLogin(w, r)
			return
		}
		if r.Method == http.MethodPost {
			if err := h.checkCSRF(r); err != nil {
				h.fail(w, r, err)
				return
			}
		}
		r = withCSRF(r, h.issueCSRF(w, r))
		if err := rejectTenantOverride(r); err != nil {
			h.fail(w, r, err)
			return
		}
		if err := rt.fn(w, r); err != nil {
			h.fail(w, r, err)
		}
	})
}

// rejectTenantOverride refuses a request that names a tenant, rather than
// letting it look as though the interface honoured one.
func rejectTenantOverride(r *http.Request) error {
	for _, name := range tenantParams {
		if r.URL.Query().Has(name) {
			return core.Forbidden("the tenant is taken from the signed-in session, not from the request")
		}
		if r.PostForm != nil && r.PostForm.Has(name) {
			return core.Forbidden("the tenant is taken from the signed-in session, not from the request")
		}
	}
	return nil
}

// redirectToLogin sends an unauthenticated browser to the sign-in screen,
// remembering where it was going.
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	target := RouteLogin
	if r.Method == http.MethodGet && r.URL.Path != RouteLogin {
		target += "?next=" + url.QueryEscape(r.URL.RequestURI())
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
