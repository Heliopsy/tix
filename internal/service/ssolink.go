// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sso"
	"github.com/heliopsy/tix/internal/store"
)

// Reasons an SSO sign-in is refused after the provider vouched for somebody,
// recorded on sso.denied and returned as the error's reason detail.
// #nosec G101 -- refusal reasons, not credentials.
const (
	ssoDeniedDomain      = "email_domain"
	ssoDeniedNoEmail     = "no_email"
	ssoDeniedUnverified  = "email_unverified"
	ssoDeniedNotMember   = "not_member"
	ssoDeniedElsewhere   = "account_elsewhere"
	ssoDeniedDisabled    = "disabled"
	ssoDeniedGlobalAdmin = "global_admin"
	ssoDeniedToken       = "token"
)

// ssoLinked is what linking resolved: the account, its actor here and role.
type ssoLinked struct {
	user     *core.User
	actor    *core.Actor
	role     core.Role
	identity *core.UserIdentity
	created  bool
	linked   bool
}

// signInIdentity is everything after the protocol: the domain allowlist,
// linking, just-in-time creation, the role mapping, the global admin
// refusal, the second factor and the session. Every kind, SAML included,
// reaches it with an Identity.
func (l *Local) signInIdentity(ctx context.Context, scope core.TenantScope, p core.SSOProvider,
	ident sso.Identity, surface core.Surface,
) (*core.LoginResult, error) {
	if reason, msg := domainRefusal(p, ident); reason != "" {
		return nil, l.deny(ctx, scope, p, ident, reason, msg)
	}
	found, err := l.resolveIdentity(ctx, scope, p, ident)
	if err != nil {
		return nil, err
	}
	if found.reason != "" {
		return nil, l.deny(ctx, scope, p, ident, found.reason, found.message)
	}
	linked, err := l.linkIdentity(ctx, scope, p, ident, found)
	if err != nil {
		return nil, err
	}
	cand := candidate{user: linked.user, actor: linked.actor, role: linked.role, tenant: scope.TenantID}
	sessionActor := cand.sessionActor(scope)
	method := core.LoginMethodSSO(p.Key)
	if p.TrustMFA && ident.MFA {
		return l.mintSession(ctx, scope, sessionActor, linked.user, "", factorOrigin(method), nil)
	}
	challenge, err := l.secondFactorFor(ctx, scope, surface, sessionActor, linked.user, method)
	if err != nil {
		return nil, err
	}
	if challenge != nil {
		return &core.LoginResult{Challenge: challenge}, nil
	}
	return l.mintSession(ctx, scope, sessionActor, linked.user, "", core.SessionOrigin{Method: method}, nil)
}

// domainRefusal applies the provider's allowed domains.
func domainRefusal(p core.SSOProvider, ident sso.Identity) (string, string) {
	if len(p.AllowedDomains) == 0 {
		return "", ""
	}
	_, domain, ok := strings.Cut(ident.Email, "@")
	if !ok || domain == "" {
		return ssoDeniedNoEmail, "the identity provider sent no email, and this provider admits only emails at " +
			strings.Join(p.AllowedDomains, ", ")
	}
	if !slices.Contains(p.AllowedDomains, domain) {
		return ssoDeniedDomain, "this provider admits only emails at " + strings.Join(p.AllowedDomains, ", ")
	}
	return "", ""
}

// ssoResolution is the account an identity names, or why it names none.
type ssoResolution struct {
	user     *core.User
	identity *core.UserIdentity
	cand     candidate
	create   bool
	reason   string
	message  string
}

func refusal(reason, message string) (ssoResolution, error) {
	return ssoResolution{reason: reason, message: message}, nil
}

// resolveIdentity applies the linking rules: subject, then a verified email
// of a member of this tenant, then just-in-time creation for an email nobody
// holds. It writes nothing.
func (l *Local) resolveIdentity(ctx context.Context, scope core.TenantScope, p core.SSOProvider,
	ident sso.Identity,
) (ssoResolution, error) {
	var linked *core.UserIdentity
	if err := l.read(ctx, core.SystemActor(scope.TenantID), func(tx store.Tx) error {
		found, err := tx.GetUserIdentity(ctx, p.ID, ident.Subject)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		linked = found
		return err
	}); err != nil {
		return ssoResolution{}, err
	}
	if linked != nil {
		cand, err := l.memberCandidate(ctx, scope, linked.UserID)
		if err != nil {
			return ssoResolution{}, err
		}
		if cand.user == nil || !cand.member() {
			return refusal(ssoDeniedNotMember, "this account is no longer a member of this tenant")
		}
		return l.vetted(ssoResolution{user: cand.user, identity: linked, cand: cand})
	}
	if ident.Email == "" {
		return refusal(ssoDeniedNoEmail, "the identity provider sent no email for this account, so it cannot be "+
			"matched to a person here; check the provider's email claim")
	}
	if !ident.EmailVerified {
		return refusal(ssoDeniedUnverified, "the identity provider has not verified "+ident.Email+
			", so it is not used to find or create an account; verify it at the provider")
	}
	account, err := l.accountByEmail(ctx, ident.Email)
	if err != nil {
		return ssoResolution{}, err
	}
	if account == nil {
		if !p.AutoCreate {
			return refusal(ssoDeniedNotMember, "there is no account for "+ident.Email+
				" in this tenant; ask an administrator to invite you, then sign in with "+p.DisplayName+" again")
		}
		return ssoResolution{create: true}, nil
	}
	cand, err := l.memberCandidate(ctx, scope, account.ID)
	if err != nil {
		return ssoResolution{}, err
	}
	if !cand.member() {
		return refusal(ssoDeniedElsewhere, ident.Email+" already has a tix account that is not a member of this "+
			"tenant; ask an administrator to invite it, then sign in with "+p.DisplayName+" again")
	}
	return l.vetted(ssoResolution{user: cand.user, cand: cand})
}

// vetted refuses an account SSO may not sign in: a disabled one, and a global
// admin, whose authority reaches every tenant while the provider vouching
// for them is administered from one.
func (l *Local) vetted(r ssoResolution) (ssoResolution, error) {
	if r.user.DisabledAt != nil || r.cand.actor.DisabledAt != nil {
		return refusal(ssoDeniedDisabled, "this account is disabled")
	}
	if r.user.GlobalAdmin {
		return refusal(ssoDeniedGlobalAdmin, "a global admin signs in with a password or an SSH key, not through "+
			"a tenant's provider")
	}
	return r, nil
}

func (l *Local) accountByEmail(ctx context.Context, email string) (*core.User, error) {
	var out *core.User
	err := l.store.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		found, _, err := u.GetUserByEmail(ctx, email)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		out = found
		return err
	})
	return out, err
}

// memberCandidate reads an account and its standing in this tenant.
func (l *Local) memberCandidate(ctx context.Context, scope core.TenantScope, userID string) (candidate, error) {
	out := candidate{tenant: scope.TenantID}
	err := l.read(ctx, core.SystemActor(scope.TenantID), func(tx store.Tx) error {
		user, err := tx.GetUser(ctx, userID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		actor, err := tx.GetActorByUser(ctx, userID)
		if core.IsKind(err, core.KindNotFound) {
			out.user = user
			return nil
		}
		if err != nil {
			return err
		}
		role, err := userRole(ctx, tx, actor.ID)
		if err != nil {
			return err
		}
		out.user, out.actor, out.role = user, actor, role
		return nil
	})
	return out, err
}

// mappedRole is the highest role the person's groups map to, or "".
func mappedRole(p core.SSOProvider, groups []string) core.Role {
	best := -1
	for group, role := range p.RoleMapping {
		if !slices.ContainsFunc(groups, func(g string) bool { return strings.EqualFold(g, group) }) {
			continue
		}
		if i := slices.Index(core.Roles, role); i > best {
			best = i
		}
	}
	if best < 0 {
		return ""
	}
	return core.Roles[best]
}

// linkIdentity writes what the sign-in resolved, in one transaction: the
// account a just-in-time sign-in creates, the identity a first sign-in links,
// the role the groups map to, the identity's last sign-in, and the audit
// entries for each.
func (l *Local) linkIdentity(ctx context.Context, scope core.TenantScope, p core.SSOProvider, ident sso.Identity,
	found ssoResolution,
) (ssoLinked, error) {
	out := ssoLinked{user: found.user, actor: found.cand.actor, role: found.cand.role, identity: found.identity}
	mapped := mappedRole(p, ident.Groups)
	writer := core.SystemActor(scope.TenantID)
	if out.actor != nil {
		writer = found.cand.sessionActor(scope)
	}
	err := l.write(ctx, writer, func(m *mutation) error {
		if found.create {
			if err := l.createSSOAccount(ctx, m, p, ident, mapped, &out); err != nil {
				return err
			}
		}
		if out.identity == nil {
			out.identity = &core.UserIdentity{ProviderID: p.ID, Subject: ident.Subject, UserID: out.user.ID,
				Email: ident.Email, LastLoginAt: &m.now}
			if err := m.tx.CreateUserIdentity(ctx, out.identity); err != nil {
				return err
			}
			out.linked = true
			if err := m.Record(auditSSOLink, eventSSOLinked, subjectSSOIdentity, out.identity.ID, "", nil,
				identityTrail(p, out.identity), identityTrail(p, out.identity)); err != nil {
				return err
			}
		} else if err := m.tx.TouchUserIdentity(ctx, out.identity.ID, ident.Email, m.now); err != nil {
			return err
		}
		before := out.role
		if mapped != "" && mapped != out.role && !found.create {
			if err := setUserRole(ctx, m.tx, out.actor.ID, mapped); err != nil {
				return err
			}
			out.role = mapped
		}
		trail := identityTrail(p, out.identity)
		trail["role"] = string(out.role)
		trail["created"], trail["linked"] = out.created, out.linked
		trail["mfa_asserted"] = ident.MFA
		var prior map[string]any
		if before != out.role {
			prior = map[string]any{"role": string(before)}
		}
		return m.Record(auditSSOLogin, eventSSOLogin, subjectSSOIdentity, out.identity.ID, "", prior, trail, trail)
	})
	return out, err
}

// identityTrail is an identity as an audit entry records it.
func identityTrail(p core.SSOProvider, i *core.UserIdentity) map[string]any {
	return map[string]any{"provider": p.Key, "provider_id": p.ID, "user_id": i.UserID, "email": i.Email,
		"subject": i.Subject}
}

var handleUnsafe = regexp.MustCompile(`[^a-z0-9._-]+`)

// ssoHandle is the handle a created account gets: the provider's username,
// else the email's local part, made safe and unique in the tenant.
func ssoHandle(ctx context.Context, tx store.Tx, ident sso.Identity) (string, error) {
	base := strings.ToLower(ident.Username)
	if base == "" {
		base, _, _ = strings.Cut(ident.Email, "@")
	}
	base = strings.Trim(handleUnsafe.ReplaceAllString(strings.ToLower(base), "-"), "-.")
	if base == "" {
		base = "user"
	}
	if len(base) > 32 {
		base = base[:32]
	}
	for n := 1; n <= 100; n++ {
		handle := base
		if n > 1 {
			handle = base + "-" + itoa(n)
		}
		switch _, err := tx.GetActorByHandle(ctx, handle); {
		case core.IsKind(err, core.KindNotFound):
			return handle, nil
		case err != nil:
			return "", err
		}
	}
	return "", core.Conflict("no free handle near %q in this tenant", base)
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// createSSOAccount creates the account, actor and membership a just-in-time
// sign-in asks for. The account has no password: it signs in through
// providers, or after an administrator sets one.
func (l *Local) createSSOAccount(ctx context.Context, m *mutation, p core.SSOProvider, ident sso.Identity,
	mapped core.Role, out *ssoLinked,
) error {
	role := mapped
	if role == "" {
		role = p.DefaultRole
	}
	handle, err := ssoHandle(ctx, m.tx, ident)
	if err != nil {
		return err
	}
	userID := l.ids.New()
	user := &core.User{ID: userID, Email: ident.Email, DisplayName: ident.Name}
	if err := m.tx.CreateUser(ctx, user, ""); err != nil {
		return err
	}
	actor := &core.Actor{ID: userID, UserID: userID, Kind: core.ActorUser, Handle: handle, DisplayName: ident.Name}
	if err := m.tx.CreateActor(ctx, actor); err != nil {
		return err
	}
	if err := setUserRole(ctx, m.tx, actor.ID, role); err != nil {
		return err
	}
	if err := m.logAccount(ctx, userID, core.AccountCreated, map[string]any{"role": string(role), "via": p.Key}); err != nil {
		return err
	}
	if err := m.Record(auditSSOUserCreated, eventSSOUserCreated, "user", userID, "", nil, user,
		map[string]any{"email": ident.Email, "handle": handle, "role": string(role), "provider": p.Key}); err != nil {
		return err
	}
	out.user, out.actor, out.role, out.created = user, actor, role, true
	return nil
}

// deny records sso.denied and returns the refusal the person sees.
func (l *Local) deny(ctx context.Context, scope core.TenantScope, p core.SSOProvider, ident sso.Identity,
	reason, message string,
) error {
	l.recordDenied(ctx, scope, p, ident, reason, message)
	return core.Forbidden("single sign-on refused: %s", message).WithDetail("reason", "sso_"+reason)
}

// recordDenied writes sso.denied. A failure to record it does not change the
// answer, which is a refusal either way.
func (l *Local) recordDenied(ctx context.Context, scope core.TenantScope, p core.SSOProvider, ident sso.Identity,
	reason, message string,
) {
	trail := map[string]any{"provider": p.Key, "provider_id": p.ID, "reason": reason, "message": message}
	if ident.Email != "" {
		trail["email"] = ident.Email
	}
	if ident.Subject != "" {
		trail["subject"] = ident.Subject
	}
	_ = l.write(ctx, core.SystemActor(scope.TenantID), func(m *mutation) error {
		return m.Record(auditSSODenied, eventSSODenied, subjectSSOProvider, p.ID, "", nil, trail, trail)
	})
}
