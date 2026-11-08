// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// peopleLimit is how many users one screen reads. The rest is a question for
// `tix user ls --cursor`, and the status bar says so when there is more.
const peopleLimit = 100

// reasonAccountInOtherTenants is the detail the service attaches to an account
// change a tenant administrator may not make, because the person also belongs
// to another tenant.
const reasonAccountInOtherTenants = "account_in_other_tenants"

// peopleState is everything the people screen renders from. It is one field
// on the model so the screen keeps to its own state.
type peopleState struct {
	users []core.User
	more  bool
	// roles maps a user to the role their membership grants here. rolesErr
	// says why it could not be read, so a missing role is not mistaken for
	// a person with none.
	roles    map[string]core.Role
	rolesErr string
	invites  []core.Invite

	usersErr   string
	invitesErr string
	loaded     bool

	// sel indexes PeopleRows, not either listing.
	sel int
	off int
	// shown is the user read in full by enter, and issued the invite link
	// minted by the last action, held until the reader acts again or leaves.
	shown  *core.User
	issued *core.IssuedInvite
}

// PeopleRowKind names what a selectable row of the people screen stands for.
type PeopleRowKind int

// The selectable rows.
const (
	peopleRowUser PeopleRowKind = iota
	peopleRowInvite
)

// PeopleRow is one row the screen's keys act on.
type PeopleRow struct {
	Kind PeopleRowKind
	// ID addresses the subject for the service call.
	ID string
	// Label names the subject as the reader sees it, which is what a
	// confirmation has to say out loud.
	Label string
}

// PeopleRows are the rows the cursor runs over: every user, then every
// pending invite.
func PeopleRows(s peopleState) []PeopleRow {
	out := make([]PeopleRow, 0, len(s.users)+len(s.invites))
	for _, u := range s.users {
		out = append(out, PeopleRow{Kind: peopleRowUser, ID: u.ID, Label: u.Email})
	}
	for _, inv := range s.invites {
		out = append(out, PeopleRow{Kind: peopleRowInvite, ID: inv.ID, Label: inv.Email})
	}
	return out
}

// peopleRowWidth aligns an address against the columns beside it.
const peopleRowWidth = 30

// peopleNameWidth aligns a display name against the role beside it.
const peopleNameWidth = 20

// PeopleView renders the screen: the link just issued, the users, the pending
// invites, and the user read in full.
func PeopleView(s peopleState, style output.TimeStyle, now time.Time) []TenantLine {
	var out []TenantLine
	if s.issued != nil {
		out = append(append(out, IssuedLines(*s.issued, style)...), TenantLine{})
	}
	out = append(out, userLines(s)...)
	out = append(out, TenantLine{})
	out = append(out, inviteLines(s, style, now)...)
	if s.shown != nil {
		out = append(append(out, TenantLine{}), ShownUserLines(*s.shown, s.roles[s.shown.ID], style)...)
	}
	return out
}

// userLines list the tenant's users with role, state and second factor.
func userLines(s peopleState) []TenantLine {
	out := []TenantLine{{Text: "users (" + strconv.Itoa(len(s.users)) + "):", Heading: true}}
	if len(s.users) == 0 {
		return append(out, tenantAbsent(s.usersErr, s.loaded)...)
	}
	for i, u := range s.users {
		selected := s.sel == i
		out = append(out, TenantLine{
			Text:     SelectionMarker(selected) + PeopleUserRow(u, s.roles[u.ID], s.rolesErr != ""),
			Selected: selected,
			Dim:      u.DisabledAt != nil && !selected,
		})
	}
	if s.rolesErr != "" {
		out = append(out, TenantLine{Text: "  roles not shown: " + s.rolesErr, Dim: true})
	}
	return out
}

// PeopleUserRow states one user on one line: address, name, role, state and
// whether they hold a second factor.
func PeopleUserRow(u core.User, role core.Role, rolesHidden bool) string {
	roleText := string(role)
	switch {
	case rolesHidden:
		roleText = "?"
	case role == "":
		roleText = "no role"
	}
	parts := []string{pad(u.Email, peopleRowWidth), pad(orDash(u.DisplayName), peopleNameWidth),
		pad(roleText, 8), userState(u)}
	if u.TOTPEnrolled {
		parts = append(parts, "2fa")
	}
	if u.RequirePasswordChange {
		parts = append(parts, "must change password")
	}
	return strings.TrimRight(strings.Join(parts, " "), " ")
}

// userState is active or disabled, the two states the browser marks.
func userState(u core.User) string {
	if u.DisabledAt != nil {
		return "disabled"
	}
	return "active  "
}

// orDash stands in for an empty value in a column, where "none" would read as
// a name.
func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// inviteLines list the invites neither accepted nor revoked, expired ones
// included so they can be re-issued.
func inviteLines(s peopleState, style output.TimeStyle, now time.Time) []TenantLine {
	out := []TenantLine{{Text: "pending invites (" + strconv.Itoa(len(s.invites)) + "):", Heading: true}}
	if len(s.invites) == 0 {
		return append(out, tenantAbsent(s.invitesErr, s.loaded)...)
	}
	for i, inv := range s.invites {
		selected := s.sel == len(s.users)+i
		out = append(out, TenantLine{
			Text:     SelectionMarker(selected) + InviteRow(inv, style, now),
			Selected: selected,
			Dim:      !inv.Pending(now) && !selected,
		})
	}
	return out
}

// InviteRow states one invite on one line: address, role, when it stops
// working and who issued it.
func InviteRow(inv core.Invite, style output.TimeStyle, now time.Time) string {
	state := "expires " + style.Format(inv.ExpiresAt)
	if !inv.ExpiresAt.After(now) {
		state = "expired"
	}
	parts := []string{pad(inv.Email, peopleRowWidth), pad(string(inv.Role), 8), state}
	if inv.InvitedByHandle != "" {
		parts = append(parts, "by @"+inv.InvitedByHandle)
	}
	if inv.GrantsMembership {
		parts = append(parts, "account of another tenant")
	}
	return strings.Join(parts, "  ")
}

// IssuedLines state an invite link the one time it can be shown. Only its
// hash is kept, so there is no way to show it again.
func IssuedLines(issued core.IssuedInvite, style output.TimeStyle) []TenantLine {
	link := issued.URL
	if link == "" {
		link = issued.Path
	}
	if link == "" {
		link = core.InvitePath(issued.Token)
	}
	out := []TenantLine{
		{Text: "invite link for " + issued.Email + ", shown once:", Heading: true},
		{Text: "  " + link},
		{Text: "  send it yourself: tix sends no email. It works once and expires " +
			style.Format(issued.ExpiresAt) + ".", Dim: true},
	}
	if issued.URL == "" {
		out = append(out, TenantLine{Text: "  put the address people open tix at in front of the path.", Dim: true})
	}
	return out
}

// ShownUserLines state one user in full, as `tix user show` does.
func ShownUserLines(u core.User, role core.Role, style output.TimeStyle) []TenantLine {
	out := []TenantLine{{Text: "user " + u.Email + ":", Heading: true}}
	row := func(label, value string) { out = append(out, tenantRow(label, value)) }
	row("id", u.ID)
	row("name", orNone(u.DisplayName))
	row("role", orNone(string(role)))
	row("state", strings.TrimSpace(userState(u)))
	row("second factor", yesNo(u.TOTPEnrolled, "enrolled", "none"))
	row("new password", yesNo(u.RequirePasswordChange, "required at next sign-in", "not required"))
	if u.SSOProvider != "" {
		row("sso", u.SSOProvider+" "+u.SSOSubject)
	}
	if u.GlobalAdmin {
		row("global admin", "yes")
	}
	row("created", style.Format(u.CreatedAt))
	row("updated", style.Format(u.UpdatedAt))
	return out
}

// yesNo picks one of two words.
func yesNo(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

// PeopleCount is what the status bar says the screen holds.
func PeopleCount(s peopleState) string {
	count := counted(len(s.users), "user", "users") + ", " +
		counted(len(s.invites), "pending invite", "pending invites")
	if s.more {
		return count + "; more with tix user ls --cursor"
	}
	return count
}

// The two ways the create form lets somebody in, named once so the form and
// the branch that reads it cannot spell one differently.
const (
	modePassword = "password"
	modeInvite   = "invite link"
)

// noRoleChange is what the edit form's role field offers when the person's
// role here could not be read, so saving another change never sets one nobody
// chose.
const noRoleChange = "unchanged"

// InviteExpiryOptions are the lifetimes an invite form offers, the same ones
// the browser does.
func InviteExpiryOptions() []string { return []string{"1d", "3d", "7d", "14d", "30d"} }

// defaultInviteExpiry is what the form proposes, the service's own default.
const defaultInviteExpiry = "7d"

// PeopleCreateNote says what the two modes mean, which the browser says in its
// own tip.
const PeopleCreateNote = "an invite link lets them choose their own password; tix sends no email, so you pass it on"

// PeopleCreateForm gathers a new user or an invite on one form, offering only
// the modes this reader may perform.
func PeopleCreateForm(mayUser, mayInvite bool) Form {
	var modes []string
	if mayUser {
		modes = append(modes, modePassword)
	}
	if mayInvite {
		modes = append(modes, modeInvite)
	}
	if len(modes) == 0 {
		return Form{}
	}
	return Form{
		Kind: formPeopleNew, Title: "new user", Note: PeopleCreateNote,
		Fields: []FormField{
			{Key: "mode", Label: "sign in by", Options: modes, Value: modes[0]},
			{Key: "email", Label: "email", Kind: FieldText, Limit: 254},
			{Key: "name", Label: "display name", Kind: FieldText, Limit: 200},
			{Key: "handle", Label: "handle", Kind: FieldText, Limit: 64},
			{Key: "role", Label: "role", Options: RoleOptions(), Value: string(core.RoleViewer)},
			{Key: "password", Label: "password", Kind: FieldText, Secret: true, Limit: 256,
				Needs: FieldCondition{Key: "mode", Value: modePassword}},
			{Key: "change", Label: "change at first sign-in", Options: switchOptions(), Value: yesValue,
				Needs: FieldCondition{Key: "mode", Value: modePassword}},
			{Key: "expires", Label: "expires in", Options: InviteExpiryOptions(), Value: defaultInviteExpiry,
				Needs: FieldCondition{Key: "mode", Value: modeInvite}},
		},
	}
}

// CreateUserFrom is what an answered create form asks CreateUser for. The
// forced change is sent only with a password, as the browser sends it, since
// the service refuses one on an account that has none.
func CreateUserFrom(f Form) core.CreateUserInput {
	password := f.Value("password")
	return core.CreateUserInput{
		Email:                 strings.TrimSpace(f.Value("email")),
		Password:              password,
		DisplayName:           strings.TrimSpace(f.Value("name")),
		Handle:                strings.TrimSpace(f.Value("handle")),
		Role:                  core.Role(f.Value("role")),
		RequirePasswordChange: password != "" && f.Yes("change"),
	}
}

// CreateInviteFrom is what an answered create form asks CreateInvite for.
func CreateInviteFrom(f Form) (core.CreateInviteInput, error) {
	ttl, err := core.ParseDuration(f.Value("expires"))
	if err != nil {
		return core.CreateInviteInput{}, err
	}
	return core.CreateInviteInput{
		Email:       strings.TrimSpace(f.Value("email")),
		DisplayName: strings.TrimSpace(f.Value("name")),
		Handle:      strings.TrimSpace(f.Value("handle")),
		Role:        core.Role(f.Value("role")),
		ExpiresIn:   ttl,
	}, nil
}

// PeopleEditNote says what an empty password field means.
const PeopleEditNote = "a password left empty is left as it is; disabling signs them out everywhere and revokes their tokens"

// PeopleEditForm offers what one update can change about a user, opening on
// the values they hold now so an untouched field is never sent.
func PeopleEditForm(u core.User, role core.Role) Form {
	roles := RoleOptions()
	value := string(role)
	if role == "" {
		roles = append([]string{noRoleChange}, roles...)
		value = noRoleChange
	}
	return Form{
		Kind: formPeopleEdit, Title: "edit " + u.Email, Note: PeopleEditNote,
		Fields: []FormField{
			{Key: "name", Label: "display name", Value: u.DisplayName, Kind: FieldText, Limit: 200},
			{Key: "role", Label: "role", Options: roles, Value: value},
			{Key: "disabled", Label: "disabled", Options: switchOptions(),
				Value: yesNo(u.DisabledAt != nil, yesValue, noValue)},
			{Key: "change", Label: "new password at next sign-in", Options: switchOptions(),
				Value: yesNo(u.RequirePasswordChange, yesValue, noValue)},
			{Key: "password", Label: "set password", Kind: FieldText, Secret: true, Limit: 256},
		},
	}
}

// UpdateUserFrom is what an answered edit form asks UpdateUser for: the fields
// the reader changed and nothing else. Sending an unchanged field is not
// harmless, because a password or a forced change for somebody who also
// belongs to another tenant is refused, and would take the rest of the edit
// with it. It reports false when nothing changed.
func UpdateUserFrom(u core.User, role core.Role, f Form) (core.UpdateUserInput, bool) {
	var in core.UpdateUserInput
	changed := false
	if name := strings.TrimSpace(f.Value("name")); name != u.DisplayName {
		in.DisplayName, changed = &name, true
	}
	if r := f.Value("role"); r != noRoleChange && core.Role(r) != role {
		next := core.Role(r)
		in.Role, changed = &next, true
	}
	if off := f.Yes("disabled"); off != (u.DisabledAt != nil) {
		in.Disabled, changed = &off, true
	}
	if req := f.Yes("change"); req != u.RequirePasswordChange {
		in.RequirePasswordChange, changed = &req, true
	}
	if pw := f.Value("password"); pw != "" {
		in.Password, changed = &pw, true
	}
	return in, changed
}

// ReissueForm asks how long a re-issued link works.
func ReissueForm(inv core.Invite) Form {
	return Form{
		Kind: formInviteReissue, Title: "re-issue the invite for " + inv.Email,
		Note: "the old link stops working; the new one is shown once",
		Fields: []FormField{
			{Key: "expires", Label: "expires in", Options: InviteExpiryOptions(), Value: defaultInviteExpiry},
		},
	}
}

// PeopleFailure explains a refused change. The refusal of an account change
// for somebody who belongs to another tenant too is said in words naming them,
// because the service's own message names nobody and a bare "forbidden" reads
// as the administrator lacking authority they actually hold.
func PeopleFailure(verb, subject string, err error) string {
	if accountInOtherTenants(err) {
		return "cannot " + verb + ": " + subject + " also belongs to another tenant, so only they or a " +
			"global admin can change their password, second factor or account"
	}
	return "cannot " + verb + ": " + err.Error()
}

// accountInOtherTenants reports whether a refusal carries the cross-tenant
// reason.
func accountInOtherTenants(err error) bool {
	var domain *core.Error
	if !errors.As(err, &domain) {
		return false
	}
	reason, _ := domain.Details["reason"].(string)
	return reason == reasonAccountInOtherTenants
}
