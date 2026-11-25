// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strings"
	"time"
)

// DefaultSessionTTL is how long a sign-in lasts when the operator sets nothing.
const DefaultSessionTTL = Duration(12 * time.Hour)

// DefaultSessionRememberTTL is how long a "keep me signed in" sign-in lasts
// when the operator sets nothing.
const DefaultSessionRememberTTL = Duration(30 * 24 * time.Hour)

// DefaultSessionMaxLifetime is how long an ordinary sign-in may be kept alive
// by use before it has to sign in again.
const DefaultSessionMaxLifetime = Duration(30 * 24 * time.Hour)

// DefaultSessionRememberMaxLifetime is how long a "keep me signed in" sign-in
// may be kept alive by use before it has to sign in again.
const DefaultSessionRememberMaxLifetime = Duration(90 * 24 * time.Hour)

// MinSessionDuration is the shortest lifetime or idle timeout accepted. Use is
// recorded at most once a minute, so anything shorter could not be honoured.
const MinSessionDuration = Duration(time.Minute)

// SessionTouchInterval is how stale a session's last-seen time may grow
// before a request records it again.
const SessionTouchInterval = time.Minute

// SessionPolicy is how long a new sign-in on a tenant lasts and how long it
// may sit unused: the server's session settings narrowed by the tenant's own.
type SessionPolicy struct {
	// TTL is how long a sign-in lasts from the moment it is made, and again
	// from each use while MaxLifetime allows.
	TTL Duration `json:"ttl" yaml:"ttl"`
	// IdleTimeout ends a session unused for this long; zero is off.
	IdleTimeout Duration `json:"idle_timeout" yaml:"idle_timeout"`
	// RememberTTL is how long a "keep me signed in" sign-in lasts; zero means
	// the choice is not offered.
	RememberTTL Duration `json:"remember_ttl" yaml:"remember_ttl"`
	// MaxLifetime is how long after the sign-in a session in use may be
	// renewed to; zero means a session ends TTL after the sign-in, used or not.
	MaxLifetime Duration `json:"max_lifetime" yaml:"max_lifetime"`
	// RememberMaxLifetime is MaxLifetime for a "keep me signed in" sign-in,
	// which is renewed by RememberTTL.
	RememberMaxLifetime Duration `json:"remember_max_lifetime" yaml:"remember_max_lifetime"`
	// ServerTTL is the server's session.ttl, which a tenant may only shorten.
	ServerTTL Duration `json:"server_ttl" yaml:"server_ttl"`
	// ServerIdleTimeout is the server's session.idle_timeout, zero when off.
	ServerIdleTimeout Duration `json:"server_idle_timeout" yaml:"server_idle_timeout"`
	// ServerRememberTTL is the server's session.remember_ttl, zero when off.
	ServerRememberTTL Duration `json:"server_remember_ttl" yaml:"server_remember_ttl"`
}

// RememberOffered reports whether "keep me signed in" would grant anything
// beyond an ordinary sign-in.
func (p SessionPolicy) RememberOffered() bool { return p.RememberTTL > p.TTL }

// Lifetime is how long a new sign-in lasts and whether it is remembered.
func (p SessionPolicy) Lifetime(remember bool) (Duration, bool) {
	if remember && p.RememberOffered() {
		return p.RememberTTL, true
	}
	return p.TTL, false
}

// Renewal is how far each use pushes a session's expiry, and how long after
// the sign-in it may be pushed to. A zero limit means it is never renewed.
func (p SessionPolicy) Renewal(remembered bool) (window, limit Duration) {
	if remembered && p.RememberTTL > 0 {
		return p.RememberTTL, p.RememberMaxLifetime
	}
	return p.TTL, p.MaxLifetime
}

// EffectiveSessionPolicy narrows the server's session settings by a tenant's
// own. A tenant value the server has since tightened past is clamped.
func EffectiveSessionPolicy(server SessionPolicy, t Tenant) SessionPolicy {
	out := SessionPolicy{
		TTL: server.TTL, IdleTimeout: server.IdleTimeout, RememberTTL: server.RememberTTL,
		MaxLifetime: server.MaxLifetime, RememberMaxLifetime: server.RememberMaxLifetime,
		ServerTTL: server.TTL, ServerIdleTimeout: server.IdleTimeout, ServerRememberTTL: server.RememberTTL,
	}
	if t.SessionTTL > 0 && t.SessionTTL < out.TTL {
		out.TTL = t.SessionTTL
	}
	if t.SessionIdleTimeout > 0 && (out.IdleTimeout == 0 || t.SessionIdleTimeout < out.IdleTimeout) {
		out.IdleTimeout = t.SessionIdleTimeout
	}
	switch {
	case t.SessionRememberOff:
		out.RememberTTL = 0
	case t.SessionRememberTTL > 0 && t.SessionRememberTTL < out.RememberTTL:
		out.RememberTTL = t.SessionRememberTTL
	}
	return out
}

// SessionRememberOff is how a tenant's "keep me signed in" setting is written
// when the tenant turned it off.
const SessionRememberOff = "off"

// ParseSessionRemember reads a tenant's "keep me signed in" setting as typed:
// empty inherits the server's, off or 0 turns it off, anything else is a
// duration.
func ParseSessionRemember(raw string) (d Duration, off bool, err error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "":
		return 0, false, nil
	case SessionRememberOff, "0":
		return 0, true, nil
	}
	if d, err = ParseDuration(s); err != nil {
		return 0, false, Invalid("keep me signed in: %v", err).WithDetail(DetailField, "session_remember_ttl")
	}
	if d <= 0 {
		return 0, true, nil
	}
	return d, false, nil
}

// SessionRememberText renders a tenant's "keep me signed in" setting the way
// ParseSessionRemember reads it.
func SessionRememberText(d Duration, off bool) string {
	switch {
	case off:
		return SessionRememberOff
	case d > 0:
		return d.String()
	default:
		return ""
	}
}
