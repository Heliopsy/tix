// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// A native app signs in through a provider by running the browser flow in
// the system browser. Where the browser flow would mint its session, an app
// flow stores a one-time code bound to the app's PKCE challenge and sends the
// browser to the app's redirect with it; the app trades the code and its
// verifier for a session at ExchangeSSOCode.

// Reasons, on a refusal's reason detail, that only an app sign-in reads:
// the provider ended the sign-in, and a flow that is stale (expired, from
// another browser or tenant, or already used).
const (
	ssoReasonProviderEnded = "sso_provider_ended"
	ssoReasonFlowStale     = "sso_flow_stale"
)

type appSignInKey struct{}

// withAppSignIn marks ctx as completing a native app's sign-in, so the
// session it would mint becomes a code handed to the app instead.
func withAppSignIn(ctx context.Context, app *auth.AppSignIn) context.Context {
	if app == nil {
		return ctx
	}
	return context.WithValue(ctx, appSignInKey{}, app)
}

// appSignInFrom is the app sign-in ctx completes, or nil.
func appSignInFrom(ctx context.Context) *auth.AppSignIn {
	app, _ := ctx.Value(appSignInKey{}).(*auth.AppSignIn)
	return app
}

// appAllowed reports whether redirect is exactly one of the configured app
// redirects. There is no prefix or pattern match: an address is listed or
// refused.
func (l *Local) appAllowed(redirect string) bool {
	return redirect != "" && slices.Contains(l.appRedirects, redirect)
}

// appSignIn checks the app half of a begin and answers what the flow keeps,
// or nil for a browser's own sign-in.
func (l *Local) appSignIn(in core.BeginSSOInput, providerID string) (*auth.AppSignIn, error) {
	if !in.App() {
		return nil, nil
	}
	field := func(msg, name string) error { return core.OnField(core.Invalid("%s", msg), name) }
	switch {
	case len(l.appRedirects) == 0:
		return nil, field("signing in from an app is turned off on this server", "app_redirect")
	case in.AppRedirect == "":
		return nil, field("an app sign-in needs app_redirect", "app_redirect")
	case !l.appAllowed(in.AppRedirect):
		return nil, field("app_redirect is not one of the app addresses this server hands a sign-in to "+
			"(auth.sso_app_redirects)", "app_redirect")
	case in.CodeChallenge == "":
		return nil, field("an app sign-in needs a code_challenge", "code_challenge")
	case in.CodeChallengeMethod != core.SSOCodeChallengeS256:
		return nil, field("code_challenge_method must be S256", "code_challenge_method")
	case !auth.ValidChallenge(in.CodeChallenge):
		return nil, field("code_challenge is not an S256 challenge: the base64url SHA-256 of the verifier, "+
			"without padding", "code_challenge")
	case utf8.RuneCountInString(in.AppState) > core.MaxAppStateLength:
		return nil, field("app_state is longer than 256 characters", "app_state")
	}
	return &auth.AppSignIn{Redirect: in.AppRedirect, Challenge: in.CodeChallenge, State: in.AppState,
		ProviderID: providerID}, nil
}

// appRefusal is err with the app's redirect, carrying the reason, as the
// address the browser goes to instead of the error screen. An app redirect
// taken off the list since the sign-in began gets the error screen.
func (l *Local) appRefusal(err error, app *auth.AppSignIn) error {
	if app == nil || !l.appAllowed(app.Redirect) {
		return err
	}
	out := core.Internal("the sign-in could not be completed")
	var src *core.Error
	if errors.As(err, &src) {
		clone := *src
		clone.Details = make(map[string]any, len(src.Details)+1)
		for k, v := range src.Details {
			clone.Details[k] = v
		}
		out = &clone
	} else {
		out = out.Wrap(err)
	}
	return out.WithDetail(core.DetailAppRedirect, app.Refusal(appReason(err)))
}

// appReason is the reason code the app is told a sign-in ended with.
func appReason(err error) string {
	var e *core.Error
	reason := ""
	if errors.As(err, &e) {
		reason, _ = e.Details["reason"].(string)
	}
	switch strings.TrimPrefix(reason, "sso_") {
	case ssoDeniedNotMember, ssoDeniedElsewhere, ssoDeniedNoEmail, ssoDeniedUnverified:
		return core.AppSignInNotLinked
	case ssoDeniedDisabled, ssoDeniedGlobalAdmin, ssoDeniedDomain, "provider_ended":
		return core.AppSignInAccessDenied
	case "flow_stale":
		return core.AppSignInExpired
	}
	switch core.KindOf(err) {
	case core.KindUnauthenticated, core.KindUpstream:
		return core.AppSignInProviderError
	case core.KindForbidden, core.KindNotFound:
		return core.AppSignInAccessDenied
	}
	return core.AppSignInServerError
}

// staleFlow marks a flow refused as expired, from another browser or tenant,
// or already used.
func staleFlow(err *core.Error) error { return err.WithDetail("reason", ssoReasonFlowStale) }

// openFlow opens a sealed flow. A flow that expired, but was an app's, is
// refused with the app's redirect, since the seal still vouches for it.
func (l *Local) openFlow(sealed string) (auth.SSOFlow, error) {
	flow, err := auth.OpenSSOFlow(l.totpKey, sealed, l.clock.Now())
	if err == nil {
		return flow, nil
	}
	if peek, perr := auth.PeekSSOFlow(l.totpKey, sealed); perr == nil && peek.App != nil {
		var e *core.Error
		if errors.As(err, &e) {
			err = staleFlow(e)
		}
		return auth.SSOFlow{}, l.appRefusal(err, peek.App)
	}
	return auth.SSOFlow{}, err
}

// handOff stores the code an app sign-in hands the app where the browser
// flow would mint its session, with whatever else the sign-in records, and
// answers the app's redirect carrying it. The session is minted when the app
// exchanges the code.
func (l *Local) handOff(ctx context.Context, scope core.TenantScope, actor *core.Actor, origin core.SessionOrigin,
	app *auth.AppSignIn, also func(*mutation) (bool, error),
) (*core.LoginResult, error) {
	if !l.appAllowed(app.Redirect) {
		return nil, core.OnField(core.Invalid("app_redirect is no longer one this server hands a sign-in to"),
			"app_redirect")
	}
	code, err := auth.NewHandoffCode()
	if err != nil {
		return nil, err
	}
	ok := false
	if err := l.write(ctx, actor, func(m *mutation) error {
		if ok, err = also(m); err != nil || !ok {
			return err
		}
		member, err := m.tx.GetActor(ctx, actor.ID)
		if err != nil {
			return err
		}
		return m.tx.CreateSSOHandoff(ctx, &store.SSOHandoff{ProviderID: app.ProviderID, CodeHash: auth.HashToken(code),
			CodeChallenge: app.Challenge, UserID: member.UserID, Method: origin.Method,
			SecondFactor: origin.SecondFactor, ExpiresAt: m.now.Add(auth.SSOHandoffTTL)}, m.now)
	}); err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &core.LoginResult{AppRedirect: app.Answer(url.Values{"code": {code}})}, nil
}

// maxHandoffInput bounds the code and verifier an exchange reads: a code is
// 43 characters and a verifier at most 128.
const maxHandoffInput = 256

// ExchangeSSOCode trades an app sign-in's one-time code and the PKCE
// verifier of its challenge for a session.
func (l *Local) ExchangeSSOCode(ctx context.Context, in core.SSOExchangeInput) (*core.LoginResult, error) {
	scope, err := core.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	code, verifier := strings.TrimSpace(in.Code), strings.TrimSpace(in.CodeVerifier)
	switch {
	case code == "":
		return nil, core.OnField(core.Invalid("code is required"), "code")
	case verifier == "":
		return nil, core.OnField(core.Invalid("code_verifier is required"), "code_verifier")
	case len(code) > maxHandoffInput || len(verifier) > maxHandoffInput:
		return nil, refuseSSOCode(ctx)
	}
	var h *store.SSOHandoff
	err = l.write(ctx, core.SystemActor(scope.TenantID), func(m *mutation) error {
		var err error
		h, err = m.tx.TakeSSOHandoff(ctx, auth.HashToken(code), m.now)
		return err
	})
	if core.IsKind(err, core.KindNotFound) {
		return nil, refuseSSOCode(ctx)
	}
	if err != nil {
		return nil, err
	}
	if !auth.VerifyPKCE(h.CodeChallenge, verifier) {
		return nil, refuseSSOCode(ctx)
	}
	cand, err := l.memberCandidate(ctx, scope, h.UserID)
	if err != nil {
		return nil, err
	}
	if cand.user == nil || !cand.member() {
		return nil, refuseSSOCode(ctx)
	}
	if found, _ := l.vetted(ssoResolution{user: cand.user, cand: cand}); found.reason != "" {
		return nil, refuseSSOCode(ctx)
	}
	if _, err := l.liveProvider(ctx, scope, h.ProviderID, "code"); err != nil {
		return nil, refuseSSOCode(ctx)
	}
	return l.mintSession(withRemember(ctx, true), scope, cand.sessionActor(scope), cand.user, "",
		core.SessionOrigin{Method: h.Method, SecondFactor: h.SecondFactor}, nil)
}

// refuseSSOCode tells the request log an exchange was refused and answers
// what a wrong password does.
func refuseSSOCode(ctx context.Context) error {
	auth.NoteRefusal(ctx, auth.Refusal{Reason: auth.RefusedSSOCode})
	return errInvalidCredentials()
}
