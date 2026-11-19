// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// Bounds on the refusal log: one line per client, reason and token a minute,
// over at most this many such keys at once.
const (
	refusalWindow  = time.Minute
	refusalMaxKeys = 4096
)

// MsgAuthRefused is the message of the record a refused credential logs.
const MsgAuthRefused = "auth refused"

// authorizationReasons are refusals of a caller who did authenticate. A page
// may ask the service for something optional and carry on without it, so
// these are logged only when the request itself was answered 403. A request
// that presented nothing is logged only when it was answered 401, since a
// SCIM discovery document is served to anybody.
var authorizationReasons = map[string]bool{
	auth.RefusedScopeDenied:          true,
	auth.RefusedProjectDenied:        true,
	auth.RefusedRequireClaim:         true,
	auth.RefusedSecondFactorRequired: true,
	auth.RefusedSCIMOnly:             true,
}

// clientAddr is the address the request is attributed to, after the trusted
// proxy policy, or "" when the peer address could not be parsed.
func clientAddr(r *http.Request) string {
	if ip := ClientIPFrom(r.Context()); ip.IsValid() {
		return ip.String()
	}
	return ""
}

// logRefusal writes the refusal record, unless the same client was refused
// for the same reason and token within the window. It names the token by id
// and name, or by a reference for one that matched nothing, and never by its
// value.
func (rt *Router) logRefusal(r *http.Request, status int, tenant string, ref auth.Refusal) {
	if authorizationReasons[ref.Reason] && status != http.StatusForbidden ||
		ref.Reason == auth.RefusedNoCredentials && status != http.StatusUnauthorized {
		return
	}
	ip := clientAddr(r)
	ok, held := rt.refusals.Allow(ip + "\x00" + ref.Reason + "\x00" + ref.TokenID)
	if !ok {
		return
	}
	attrs := []any{
		"request_id", RequestIDFrom(r.Context()),
		"method", r.Method,
		"path", loggedPath(r.URL.Path),
		"status", status,
		"reason", ref.Reason,
		"tenant", tenant,
		"client_ip", ip,
		"user_agent", core.TruncateUserAgent(r.UserAgent()),
		"via", string(core.ViaFrom(r.Context()).Kind),
	}
	for _, kv := range [][2]string{
		{"token_id", ref.TokenID}, {"token_name", ref.TokenName},
		{"token_ref", ref.TokenRef}, {"session", ref.Session}, {"fingerprint", ref.Fingerprint},
	} {
		if kv[1] != "" {
			attrs = append(attrs, kv[0], kv[1])
		}
	}
	if held > 0 {
		attrs = append(attrs, "suppressed", held)
	}
	rt.cfg.Logger.Warn(MsgAuthRefused, attrs...)
}
