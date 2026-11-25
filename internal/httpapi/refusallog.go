// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/logging"
)

// Bounds on the refusal log: one line per client, reason and token a minute,
// over at most this many such keys at once.
const (
	refusalWindow  = time.Minute
	refusalMaxKeys = 4096
)

// MsgAuthRefused is the message of the record a refused credential logs.
const MsgAuthRefused = "auth refused"

// MsgAuthRefusalsHeldBack is the message of the record that reports refusals
// held back when no later refusal record carried their count.
const MsgAuthRefusalsHeldBack = "auth refusals held back"

// refusalIPv6Prefix is how much of an IPv6 address keys the refusal log. One
// host is usually handed a whole /64, so a client cycling through it is one
// client.
const refusalIPv6Prefix = 64

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

// refusalLogged reports whether a refusal noted on a request answered status
// is one: a policy refusal only on a 403, no credential only on a 401, and
// nothing on a server error, which is not the client's to fix.
func refusalLogged(status int, reason string) bool {
	switch {
	case status >= http.StatusInternalServerError:
		return false
	case authorizationReasons[reason]:
		return status == http.StatusForbidden
	case reason == auth.RefusedNoCredentials:
		return status == http.StatusUnauthorized
	}
	return true
}

// refusalClient is the client part of the refusal log's key: the address, or
// its /64 for IPv6.
func refusalClient(r *http.Request) string {
	ip := ClientIPFrom(r.Context())
	if !ip.IsValid() {
		return ""
	}
	if ip = ip.Unmap(); ip.Is6() {
		return netip.PrefixFrom(ip, refusalIPv6Prefix).Masked().String()
	}
	return ip.String()
}

// logRefusal writes the refusal record, unless the same client was refused
// for the same reason and token within the window. It names the token by id
// and name, or by a reference for one that matched nothing, and never by its
// value.
func (rt *Router) logRefusal(r *http.Request, status int, tenant string, ref auth.Refusal) {
	if !refusalLogged(status, ref.Reason) {
		return
	}
	ok, held := rt.refusals.Allow(refusalClient(r) + "\x00" + ref.Reason + "\x00" + ref.TokenID)
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
		"client_ip", clientAddr(r),
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

// logHeldBack reports a count the refusal log held back and no later record
// carried, for the client, reason and token it was counted under, or as an
// overflow when every key was taken.
func (rt *Router) logHeldBack(key string, held int) {
	if key == logging.OverflowKey {
		rt.cfg.Logger.Warn(MsgAuthRefusalsHeldBack, "overflow", true, "suppressed", held)
		return
	}
	client, rest, _ := strings.Cut(key, "\x00")
	reason, tokenID, _ := strings.Cut(rest, "\x00")
	attrs := []any{"client_ip", client, "reason", reason}
	if tokenID != "" {
		attrs = append(attrs, "token_id", tokenID)
	}
	rt.cfg.Logger.Warn(MsgAuthRefusalsHeldBack, append(attrs, "suppressed", held)...)
}

// ReportHeldBackRefusals reports the refusal log's held-back counts once a
// minute, and every count still held when ctx ends, so a client that stops
// does not take its count with it.
func (rt *Router) ReportHeldBackRefusals(ctx context.Context) error {
	return rt.refusals.Run(ctx)
}
