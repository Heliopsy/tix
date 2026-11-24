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

type (
	appSignInKey struct{}
	enrollingKey struct{}
)

// enrolling marks ctx as the sign-in step that completes an enrolment, so an
// app's code waits for the reader to keep the recovery codes and open the
// app, rather than being minted now and expiring on the codes page.
func enrolling(ctx context.Context) context.Context {
	return context.WithValue(ctx, enrollingKey{}, true)
}

// fromBrowser reports whether ctx arrived through the browser interface,
// the one place an app's sign-in may run and finish.
func fromBrowser(ctx context.Context) bool { return core.SourceFrom(ctx) == core.SourceWeb }

// appOnlyInBrowser refuses an app's ticket or flow presented anywhere but
// the browser interface, where nothing would take the browser to the app.
func appOnlyInBrowser(ctx context.Context, app *auth.AppSignIn, field string) error {
	if app == nil || fromBrowser(ctx) {
		return nil
	}
	return core.OnField(core.Invalid("this sign-in was started by an app in the system browser and finishes "+
		"there; the app trades its code at %s", "/api/v1/auth/sso/exchange"), field)
}

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
	return redirect != "" && !core.AppSchemeRefused(redirect) && slices.Contains(l.appRedirects, redirect)
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

// ticketRefusal is a refused second factor step, sent back to the app when
// an app started the sign-in in this browser. A wrong or rate limited code
// stays on the form, where the person can try again.
func (l *Local) ticketRefusal(ctx context.Context, err error, raw string) error {
	if !fromBrowser(ctx) || codeRefused(err) {
		return err
	}
	peek, perr := auth.PeekTicket(l.totpKey, raw)
	if perr != nil || peek.App == nil {
		return err
	}
	var e *core.Error
	if sameError(err, auth.ErrSignInExpired()) && errors.As(err, &e) {
		err = staleFlow(e)
	}
	return l.appRefusal(err, peek.App)
}

// codeRefused reports whether err refused the code itself: wrong, spent or
// rate limited.
func codeRefused(err error) bool {
	return core.IsTOTPLocked(err) || sameError(err, auth.ErrTOTPInvalid())
}

// sameError reports whether err is the domain error want: the same kind and
// message.
func sameError(err, want error) bool {
	var got, w *core.Error
	return errors.As(err, &got) && errors.As(want, &w) && got.Kind == w.Kind && got.Message == w.Message
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
// exchanges the code. A step that completes an enrolment records it and
// answers a handoff ticket instead, which mints the code once the reader
// has kept the recovery codes.
func (l *Local) handOff(ctx context.Context, scope core.TenantScope, actor *core.Actor, origin core.SessionOrigin,
	app *auth.AppSignIn, also func(*mutation) (bool, error),
) (*core.LoginResult, error) {
	if !l.appAllowed(app.Redirect) {
		return nil, core.OnField(core.Invalid("app_redirect is no longer one this server hands a sign-in to"),
			"app_redirect")
	}
	if done, _ := ctx.Value(enrollingKey{}).(bool); done {
		return l.deferHandOff(ctx, scope, actor, origin, app, also)
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

// deferHandOff records what an enrolling step records and answers the
// signed handoff the recovery codes page sends back to mint the code.
func (l *Local) deferHandOff(ctx context.Context, scope core.TenantScope, actor *core.Actor,
	origin core.SessionOrigin, app *auth.AppSignIn, also func(*mutation) (bool, error),
) (*core.LoginResult, error) {
	var (
		ok     bool
		userID string
	)
	if err := l.write(ctx, actor, func(m *mutation) error {
		var err error
		if ok, err = also(m); err != nil || !ok {
			return err
		}
		member, err := m.tx.GetActor(ctx, actor.ID)
		if err != nil {
			return err
		}
		userID = member.UserID
		return nil
	}); err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	ticket, err := auth.MintHandoffTicket(l.totpKey, auth.Ticket{UserID: userID, TenantID: scope.TenantID,
		Surface: core.SurfaceWeb, ExpiresAt: l.clock.Now().Add(auth.SSOHandoffTicketTTL), Method: origin.Method,
		App: app})
	if err != nil {
		return nil, err
	}
	return &core.LoginResult{AppHandoff: ticket}, nil
}

// resumeHandoff mints the app's code once the reader of an enrolment's
// recovery codes opens the app, and answers the app's redirect carrying it.
func (l *Local) resumeHandoff(ctx context.Context, scope core.TenantScope, raw string) (*core.LoginResult, error) {
	ticket, err := auth.ParseHandoffTicket(l.totpKey, raw, l.clock.Now())
	if err != nil {
		auth.NoteRefusal(ctx, auth.Refusal{Reason: auth.RefusedSignInExpired})
		var e *core.Error
		if ticket.App == nil || !errors.As(err, &e) {
			return nil, err
		}
		return nil, l.appRefusal(staleFlow(e), ticket.App)
	}
	if err := appOnlyInBrowser(ctx, ticket.App, "app_handoff"); err != nil {
		return nil, err
	}
	out, err := l.mintHandoff(withAppSignIn(ctx, ticket.App), scope, ticket)
	if err != nil {
		return nil, l.appRefusal(err, ticket.App)
	}
	return out, nil
}

// mintHandoff stores the code a handoff ticket stands for, for the person it
// names while they still hold a membership here. The exchange checks the
// rest, as it does for a code handed over at once.
func (l *Local) mintHandoff(ctx context.Context, scope core.TenantScope, ticket auth.Ticket) (*core.LoginResult, error) {
	if ticket.TenantID != scope.TenantID && l.servedHere(ctx, ticket.TenantID) {
		scope = core.TenantScope{TenantID: ticket.TenantID}
	}
	if ticket.TenantID != scope.TenantID {
		return nil, staleFlow(core.Unauthenticated("this sign-in was for another tenant; start again"))
	}
	cand, err := l.memberCandidate(ctx, scope, ticket.UserID)
	if err != nil {
		return nil, err
	}
	if cand.user == nil || cand.user.DisabledAt != nil || !cand.member() {
		auth.NoteRefusal(ctx, auth.Refusal{Reason: auth.RefusedUnknownAccount})
		return nil, core.Forbidden("this account can no longer sign in here")
	}
	return l.handOff(ctx, scope, cand.sessionActor(scope), factorOrigin(ticket.Method), ticket.App,
		func(*mutation) (bool, error) { return true, nil })
}

// maxHandoffInput bounds the code and verifier an exchange reads: a code is
// 43 characters and a verifier at most 128.
const maxHandoffInput = 256

// ExchangeSSOCode trades an app sign-in's one-time code and the PKCE
// verifier of its challenge for a session.
//
// A code that is not found still commits the take, so the expired codes it
// pruned stay gone.
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
		if core.IsKind(err, core.KindNotFound) {
			h, err = nil, nil
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, refuseSSOCode(ctx)
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
