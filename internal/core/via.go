// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"
)

// ViaKind names the access channel a change arrived through. It is finer than
// Source: Source says which surface, ViaKind says how the caller reached it.
type ViaKind string

// Access channels.
const (
	ViaWeb       ViaKind = "web"
	ViaAPI       ViaKind = "api"
	ViaCLILocal  ViaKind = "cli-local"
	ViaCLIRemote ViaKind = "cli-remote"
	ViaSSH       ViaKind = "ssh"
	ViaMCP       ViaKind = "mcp"
	ViaSync      ViaKind = "sync"
	ViaImport    ViaKind = "import"
	ViaSystem    ViaKind = "system"
	ViaUnknown   ViaKind = "unknown"
)

// ViaKinds lists every access channel, in the order a filter offers them.
var ViaKinds = []ViaKind{
	ViaWeb, ViaAPI, ViaCLILocal, ViaCLIRemote, ViaSSH, ViaMCP,
	ViaSync, ViaImport, ViaSystem, ViaUnknown,
}

// Valid reports whether k is a known access channel.
func (k ViaKind) Valid() bool { return slices.Contains(ViaKinds, k) }

// Client hints a tix client sends in its X-Tix-Client header. A hint only
// refines a bearer-token request from api to one of these; it never grants a
// channel the credential did not already establish.
const (
	ClientHintCLI = "cli"
	ClientHintMCP = "mcp"
)

// MaxViaUserAgent bounds the user agent an entry keeps.
const MaxViaUserAgent = 160

// Via is the access channel of one change and what identified the caller on
// it. It is set once by the entry point the request arrived through and never
// read from a request body. It holds no secret: a token is named by its ID and
// name, a browser session by an opaque reference, never by the value
// presented.
type Via struct {
	// Kind is the channel: web, api, cli-local, cli-remote, ssh, mcp, sync,
	// import or system, and unknown for an entry recorded before channels were.
	Kind ViaKind `json:"kind" yaml:"kind"`
	// Origin is the channel that started a sync run or an import, whose own
	// entries record sync or import as their Kind.
	Origin ViaKind `json:"origin,omitempty" yaml:"origin,omitempty"`
	// TokenID is the API token the caller authenticated with.
	TokenID string `json:"token_id,omitempty" yaml:"token_id,omitempty"`
	// TokenName is that token's name when the change was made.
	TokenName string `json:"token_name,omitempty" yaml:"token_name,omitempty"`
	// Session is an opaque reference to the browser session, a digest that is
	// neither the cookie nor the hash a lookup accepts.
	Session string `json:"session,omitempty" yaml:"session,omitempty"`
	// KeyFingerprint is the SHA256 fingerprint of the SSH key a terminal
	// session signed in with.
	KeyFingerprint string `json:"key_fingerprint,omitempty" yaml:"key_fingerprint,omitempty"`
	// ClientAddr is the client address, as the trusted proxies report it.
	ClientAddr string `json:"client_addr,omitempty" yaml:"client_addr,omitempty"`
	// UserAgent is the HTTP user agent, truncated.
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
	// Host is the machine a local command ran on, when cli.record_host is on.
	Host string `json:"host,omitempty" yaml:"host,omitempty"`
	// OSUser is the operating-system user a local command ran as, when
	// cli.record_host is on.
	OSUser string `json:"os_user,omitempty" yaml:"os_user,omitempty"`
	// Job names the background work a system change came from, such as
	// lease-sweep or housekeeping.
	Job string `json:"job,omitempty" yaml:"job,omitempty"`
}

// Known returns v with an empty kind read as unknown.
func (v Via) Known() Via {
	if !v.Kind.Valid() {
		v.Kind = ViaUnknown
	}
	return v
}

// Derive returns the channel of work that v started, such as a sync run or an
// import, keeping who started it and through what.
func (v Via) Derive(kind ViaKind) Via {
	if v.Kind == kind {
		return v
	}
	if v.Kind != "" && v.Kind != ViaUnknown {
		v.Origin = v.Kind
	}
	v.Kind = kind
	return v
}

// Refine applies a client hint to a token request: api becomes cli-remote or
// mcp. Any other channel, and any other hint, is returned unchanged, so a
// header can never claim web, ssh or system.
func (v Via) Refine(hint string) Via {
	if v.Kind != ViaAPI {
		return v
	}
	switch strings.ToLower(strings.TrimSpace(hint)) {
	case ClientHintCLI:
		v.Kind = ViaCLIRemote
	case ClientHintMCP:
		v.Kind = ViaMCP
	}
	return v
}

// Public returns the part of v an event carries: the channel and the token's
// name. Network addresses, user agents, sessions, key fingerprints and host
// details stay in the audit log, which needs audit:read to see.
func (v Via) Public() Via {
	return Via{Kind: v.Kind, Origin: v.Origin, TokenID: v.TokenID, TokenName: v.TokenName, Job: v.Job}
}

// TruncateUserAgent bounds a user agent to MaxViaUserAgent bytes without
// splitting a character.
func TruncateUserAgent(ua string) string {
	ua = strings.TrimSpace(ua)
	if len(ua) <= MaxViaUserAgent {
		return ua
	}
	cut := MaxViaUserAgent
	for cut > 0 && !utf8.RuneStart(ua[cut]) {
		cut--
	}
	return ua[:cut]
}

type viaKey struct{}

// WithVia returns a context recording the access channel. Only an entry point
// calls it, from what the transport itself established.
func WithVia(ctx context.Context, v Via) context.Context {
	return context.WithValue(ctx, viaKey{}, v)
}

// ViaFrom returns the access channel in ctx, or unknown when no entry point
// set one.
func ViaFrom(ctx context.Context) Via {
	v, _ := ctx.Value(viaKey{}).(Via)
	return v.Known()
}
