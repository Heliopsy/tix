// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"errors"
	"slices"
	"time"
)

// Surface names an interactive sign-in path a second factor can be required
// on. It is not Source: Source records how any operation arrived, while this
// names the three paths that mint a session by proving a human is present. An
// API token carries no interactive channel and so appears here not at all.
type Surface string

// The interactive sign-in surfaces.
const (
	SurfaceWeb Surface = "web"
	SurfaceCLI Surface = "cli"
	SurfaceSSH Surface = "ssh"
)

// Surfaces lists every sign-in surface a requirement can be set on.
var Surfaces = []Surface{SurfaceWeb, SurfaceCLI, SurfaceSSH}

// Valid reports whether s names a known sign-in surface.
func (s Surface) Valid() bool { return slices.Contains(Surfaces, s) }

// SurfaceOf maps the access path an operation arrived through to the sign-in
// surface a tenant's requirement is written against. The terminal interface is
// reached over SSH, which is the surface an administrator configures.
func SurfaceOf(src Source) (Surface, bool) {
	switch src {
	case SourceWeb:
		return SurfaceWeb, true
	case SourceCLI, SourceAPI:
		return SurfaceCLI, true
	case SourceTUI:
		return SurfaceSSH, true
	default:
		return "", false
	}
}

// SurfaceFrom is the sign-in surface of the call ctx carries. Anything that
// arrived over the SSH listener is on the ssh surface, whatever it runs there:
// a command run over SSH has the cli source, and judging it on the cli surface
// would let it walk past a requirement written for ssh.
func SurfaceFrom(ctx context.Context) (Surface, bool) {
	if ViaFrom(ctx).Kind == ViaSSH {
		return SurfaceSSH, true
	}
	return SurfaceOf(SourceFrom(ctx))
}

// UserTOTP is one user's enrolled second factor. The secret is held sealed and
// is never rendered, so the type carries no serialisation tags: nothing may
// put it on a wire or in an audit entry.
type UserTOTP struct {
	UserID       string
	SecretCipher string
	SecretNonce  string
	// ConfirmedAt is nil until the user has proved possession. An unconfirmed
	// enrolment satisfies no requirement to hold a second factor.
	ConfirmedAt *time.Time
	// LastStep is the highest time step this enrolment has accepted. A code
	// whose step is at or below it is refused, which is what makes a code
	// single use inside an acceptance window that spans three steps.
	LastStep    int64
	Failures    int
	LockedUntil *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Confirmed reports whether the enrolment has been proved and so counts.
func (e *UserTOTP) Confirmed() bool { return e != nil && e.ConfirmedAt != nil }

// RecoveryCode is one single-use substitute for a TOTP code, stored only as a
// hash. A spent code is marked rather than removed, so the audit trail can
// name which one was used.
type RecoveryCode struct {
	ID        string
	UserID    string
	CodeHash  string
	CreatedAt time.Time
	UsedAt    *time.Time
}

// TOTPStatus reports a user's enrolment without disclosing anything that could
// produce a code.
type TOTPStatus struct {
	// Enrolled is true when the caller holds a confirmed second factor.
	Enrolled bool `json:"enrolled" yaml:"enrolled"`
	// Pending is an enrolment that was begun and never confirmed. It grants
	// nothing; it is reported so a user who abandoned enrolment is told why
	// their authenticator entry does not work.
	Pending bool `json:"pending" yaml:"pending"`
	// ConfirmedAt is when the enrolment was confirmed, RFC 3339, absent if never.
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty" yaml:"confirmed_at,omitempty"`
	// RecoveryCodesLeft is how many unused recovery codes the caller holds.
	RecoveryCodesLeft int `json:"recovery_codes_left" yaml:"recovery_codes_left"`
	// RequiredWeb is true when the tenant requires a second factor for web sign-in.
	RequiredWeb bool `json:"required_web" yaml:"required_web"`
	// RequiredCLI is true when the tenant requires a second factor for command line sign-in.
	RequiredCLI bool `json:"required_cli" yaml:"required_cli"`
	// RequiredSSH is true when the tenant requires a second factor over SSH.
	RequiredSSH bool `json:"required_ssh" yaml:"required_ssh"`
	// Policy is the tenant's second factor policy, off or required.
	Policy MFAPolicy `json:"policy" yaml:"policy"`
	// EnrollmentRequired is true while the policy refuses the caller
	// everything but enrolment, because they hold no factor.
	EnrollmentRequired bool `json:"enrollment_required" yaml:"enrollment_required"`
	// EnrollmentDue is when the policy starts refusing the caller, RFC 3339,
	// set while it is required and the caller holds no factor.
	EnrollmentDue *time.Time `json:"enrollment_due,omitempty" yaml:"enrollment_due,omitempty"`
	// VerificationLocked is true while too many wrong codes have locked verification.
	VerificationLocked bool `json:"verification_locked" yaml:"verification_locked"`
	// VerificationLockEnd is when the lock lifts, RFC 3339, set only while locked.
	VerificationLockEnd *time.Time `json:"verification_lock_end,omitempty" yaml:"verification_lock_end,omitempty"`
}

// TOTPEnrolment is what a started enrolment hands back: the secret to put in
// an authenticator, in both the forms a person might use. It takes effect only
// once a code produced from it is confirmed.
type TOTPEnrolment struct {
	// Secret is the base32 shared secret, for typing into an authenticator.
	Secret string `json:"secret" yaml:"secret"`
	// URI is the otpauth:// URI carrying the secret, for a QR code.
	URI string `json:"uri" yaml:"uri"`
	// Issuer is the name the authenticator files the entry under, the tenant's name or "tix".
	Issuer string `json:"issuer" yaml:"issuer"`
	// Account is the label the authenticator shows beside the code.
	Account string `json:"account" yaml:"account"`
}

// TOTPRecoveryCodes carries codes the caller must show exactly once. They are
// stored hashed, so nothing can produce them again.
type TOTPRecoveryCodes struct {
	// Codes are the single-use recovery codes, shown this once.
	Codes []string `json:"codes" yaml:"codes"`
}

// TOTPChallengeKind says what a sign-in still needs before a session exists.
type TOTPChallengeKind string

// The two things a second-factor challenge can ask for.
const (
	// ChallengeCode asks an enrolled user for a code.
	ChallengeCode TOTPChallengeKind = "code"
	// ChallengeEnrol takes an unenrolled user through enrolment, because a
	// tenant that turns the requirement on must not lock out the people it
	// applies to, the administrator who turned it on first among them.
	ChallengeEnrol TOTPChallengeKind = "enrol"
	// ChallengePasswordChange asks a user whose account requires it for a
	// new password. It carries no ticket: the next step sends the email and
	// current password again together with NewPassword.
	ChallengePasswordChange TOTPChallengeKind = "password_change"
)

// TOTPChallenge is the second step of a sign-in. Ticket names the first step
// that was already passed and is presented again with the code; it proves the
// password was accepted and carries no authority of its own.
type TOTPChallenge struct {
	// Kind says whether a code is asked for, enrolment must happen first, or
	// a new password must be chosen.
	Kind TOTPChallengeKind `json:"kind" yaml:"kind"`
	// Ticket is presented with the code in the next step; it expires after
	// five minutes. Empty for a password change.
	Ticket string `json:"ticket" yaml:"ticket"`
	// Enrolment is set only for ChallengeEnrol.
	Enrolment *TOTPEnrolment `json:"enrolment,omitempty" yaml:"enrolment,omitempty"`
}

// LoginInput is one step of an interactive sign-in. The first step carries the
// email, the password and the surface; the second carries the ticket the
// challenge returned and the code.
//
// There is deliberately no way to pass a code with a password in one call: a
// surface that could do that would be a surface that could take the code from
// a flag or an environment variable, which puts a live second factor in shell
// history and in the process table.
type LoginInput struct {
	// Email identifies the user, in the first step.
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
	// Password is the user's password, in the first step.
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	// NewPassword replaces the password in the first step, sent only after a
	// password_change challenge. It must differ from Password and be at
	// least 12 characters.
	NewPassword string `json:"new_password,omitempty" yaml:"new_password,omitempty"`
	// Surface is the sign-in path the session is for, which decides whether a
	// second factor is required.
	Surface Surface `json:"surface" yaml:"surface"`
	// Ticket is the challenge ticket, in the second step.
	Ticket string `json:"ticket,omitempty" yaml:"ticket,omitempty"`
	// Code is the TOTP or recovery code, in the second step.
	Code string `json:"code,omitempty" yaml:"code,omitempty"`
}

// LoginResult is either a session or the challenge standing between the caller
// and one. Exactly one of the two is set.
type LoginResult struct {
	// Session is the new session, set when sign-in is complete.
	Session *Session `json:"session,omitempty" yaml:"session,omitempty"`
	// Challenge is set when a second factor or a new password is still needed.
	Challenge *TOTPChallenge `json:"challenge,omitempty" yaml:"challenge,omitempty"`
	// RecoveryCodes is set once, when a sign-in completed by enrolling. The
	// caller must show them and cannot ask for them again.
	RecoveryCodes []string `json:"recovery_codes,omitempty" yaml:"recovery_codes,omitempty"`
	// SecondFactorDue is set on a session minted for a person who holds no
	// factor under the tenant's second factor policy: when the policy starts
	// refusing them, RFC 3339. A time already past means the session can do
	// nothing but enrol.
	SecondFactorDue *time.Time `json:"second_factor_due,omitempty" yaml:"second_factor_due,omitempty"`
}

// TOTPReport is what diagnostics say about the second factor. It names no
// user and carries no secret.
type TOTPReport struct {
	// KeyConfigured reports whether a secret-sealing key is set. Without one
	// enrolment is refused outright rather than falling back to plaintext.
	KeyConfigured bool `json:"key_configured" yaml:"key_configured"`
	// Enrolments counts confirmed enrolments across the installation, because
	// the key that opens them is installation-wide and so is the consequence
	// of losing it.
	Enrolments int `json:"enrolments" yaml:"enrolments"`
	// Unrecoverable is enrolments present with no key: every one of them is
	// undecryptable until an administrator clears it and the user enrols again.
	Unrecoverable bool `json:"unrecoverable" yaml:"unrecoverable"`

	// TenantKey is the key of the tenant the rest of the report covers.
	TenantKey string `json:"tenant_key" yaml:"tenant_key"`
	// RequireWeb is true when the tenant requires a second factor for web sign-in.
	RequireWeb bool `json:"require_web" yaml:"require_web"`
	// RequireCLI is true when the tenant requires a second factor for command line sign-in.
	RequireCLI bool `json:"require_cli" yaml:"require_cli"`
	// RequireSSH is true when the tenant requires a second factor over SSH.
	RequireSSH bool `json:"require_ssh" yaml:"require_ssh"`
	// Admins is how many members of the tenant hold the admin role.
	Admins int `json:"admins" yaml:"admins"`
	// EnrolledAdmins is how many of those admins hold a confirmed second factor.
	EnrolledAdmins int `json:"enrolled_admins" yaml:"enrolled_admins"`
}

// RequiresAnywhere reports whether this tenant demands a factor on any surface.
func (r TOTPReport) RequiresAnywhere() bool {
	return r.RequireWeb || r.RequireCLI || r.RequireSSH
}

// RequiresEverywhere reports whether this tenant demands a factor on all three.
func (r TOTPReport) RequiresEverywhere() bool {
	return r.RequireWeb && r.RequireCLI && r.RequireSSH
}

// LockedOutRisk is the configuration that ends with nobody able to sign in: a
// factor required on every surface with fewer than two administrators holding
// one. A single enrolled administrator who loses the device and the codes
// leaves a tenant with no way back in.
func (r TOTPReport) LockedOutRisk() bool {
	return r.RequiresEverywhere() && r.EnrolledAdmins < 2
}

// DetailReason names why a refusal happened when the kind alone cannot say.
const DetailReason = "reason"

// ReasonTOTPLocked marks a refusal that is rate limiting rather than a wrong
// code, so a surface says "wait" rather than "that code is wrong".
const ReasonTOTPLocked = "totp_locked"

// IsTOTPLocked reports whether err refused a code for rate limiting.
func IsTOTPLocked(err error) bool {
	var domain *Error
	if !errors.As(err, &domain) {
		return false
	}
	reason, _ := domain.Details[DetailReason].(string)
	return reason == ReasonTOTPLocked
}
