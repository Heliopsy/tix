// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// TenantLine is one rendered row of the tenant screen, tagged with what it is
// so the frame can style it without parsing its text back.
type TenantLine struct {
	Text     string
	Heading  bool
	Dim      bool
	Selected bool
}

// TenantRowKind names what a selectable row of the tenant screen stands for.
// One cursor runs over the domains and the members together, so a row carries
// which of the two it is and the removal never has to guess.
type TenantRowKind int

// The selectable rows.
const (
	tenantRowDomain TenantRowKind = iota
	tenantRowMember
	tenantRowMembership
)

// TenantRow is one row the removal key can act on.
type TenantRow struct {
	Kind TenantRowKind
	// Key addresses the subject for the service call: a hostname, or an actor
	// identifier.
	Key string
	// Label names the subject the way the reader sees it on screen, which is
	// what the confirmation has to say out loud.
	Label string
}

// TenantAdminState is everything the tenant screen renders from. Each listing
// carries its own refusal, because the three reads need three authorities and a
// reader refused one still gets the rest of the screen.
type TenantAdminState struct {
	Current   string
	Handle    string
	CanSwitch bool
	Tenant    *core.Tenant
	Tenants   []core.Tenant
	Domains   []core.Domain
	Members   []core.Membership
	// Memberships are the reader's own tenants, listed for a switch when
	// there is more than one. Reachable reports which a credential reaches,
	// nil meaning all, and OverSSH answers each with the ssh command instead.
	Memberships []core.AccountMembership
	Reachable   TenantReachable
	OverSSH     bool
	// Handles maps an actor identifier to the handle a reader recognises, for
	// every member the listing names.
	Handles map[string]string

	TenantErr  string
	TenantsErr string
	DomainErr  string
	MemberErr  string

	// Selected indexes TenantRows, not any one listing.
	Selected int
	// Loaded reports that the reads have come back, so a screen drawn before
	// they do says it is reading rather than that the tenant holds nothing.
	Loaded bool
}

// tenantLabelWidth aligns the attribute names, wide enough for the longest of
// them so no value starts in a different column from the one above it.
const tenantLabelWidth = 14

// tenantRowWidth aligns a hostname or a handle against the note beside it.
const tenantRowWidth = 28

// attrTheme is the tenant attribute the edit form answers from its own list.
// The name is free text and is gathered by the prompt under attrName, which the
// project screen already names.
const attrTheme = "theme"

// attrMFA and attrMFAGrace are the second factor policy and its grace, both
// answered from lists: the policy has two values, and a grace is one of a few
// lengths anybody actually picks. Any other length is for tix tenant edit.
const (
	attrMFA      = "second factor"
	attrMFAGrace = "grace"

	attrArtifactMax   = "artifact size"
	attrArtifactTypes = "artifact types"

	attrSessionTTL      = "sign-in lasts"
	attrSessionIdle     = "idle timeout"
	attrSessionRemember = "keep signed in"
)

// MFAGraceOptions are the grace lengths the edit form offers.
var MFAGraceOptions = []string{"0", "1d", "3d", "7d", "14d", "30d"}

// noTheme is what the theme list calls the absence of one. The service takes an
// empty string, which no list of alternatives can show.
const noTheme = "none"

// TenantRows are the rows the cursor runs over: every domain, then every
// member, then the reader's tenants when there is more than one. A listing
// that was refused contributes none, so the cursor never lands on a row the
// screen is not drawing.
func TenantRows(state TenantAdminState) []TenantRow {
	out := make([]TenantRow, 0, len(state.Domains)+len(state.Members)+len(state.Memberships))
	for _, d := range state.Domains {
		out = append(out, TenantRow{Kind: tenantRowDomain, Key: d.Hostname, Label: d.Hostname})
	}
	for _, m := range state.Members {
		out = append(out, TenantRow{Kind: tenantRowMember, Key: m.ActorID,
			Label: MemberLabel(m.ActorID, state.Handles)})
	}
	for _, m := range listedMemberships(state) {
		out = append(out, TenantRow{Kind: tenantRowMembership, Key: m.TenantKey, Label: m.TenantKey})
	}
	return out
}

// listedMemberships are the reader's tenants the screen offers, none when
// there is only the one this session is already in.
func listedMemberships(state TenantAdminState) []core.AccountMembership {
	if len(state.Memberships) < 2 {
		return nil
	}
	return state.Memberships
}

// MemberLabel names a member the way a reader recognises them. An identifier
// the directory could not resolve is shown as it is, because a blank row reads
// as a row that failed to draw.
func MemberLabel(actorID string, handles map[string]string) string {
	if handle := handles[actorID]; handle != "" {
		return "@" + handle
	}
	return actorID
}

// TenantAdminView renders the tenant, the tenants this session can see, the
// hostnames that resolve to it and the actors who belong to it, followed by the
// switch the screen has always offered.
func TenantAdminView(state TenantAdminState) []TenantLine {
	rows := TenantRows(state)
	out := tenantAttributes(state)
	out = append(out, TenantLine{})
	out = append(out, tenantListing(state)...)
	out = append(out, TenantLine{})
	out = append(out, domainLines(state, rows)...)
	out = append(out, TenantLine{})
	out = append(out, memberLines(state, rows)...)
	out = append(out, TenantLine{})
	if lines := membershipLines(state, rows); len(lines) > 0 {
		out = append(out, lines...)
		out = append(out, TenantLine{})
	}
	return append(out, switchLines(state)...)
}

// tenantAttributes state the tenant's own mutable attributes, which are exactly
// the ones the edit form offers to change, and who this session is here.
func tenantAttributes(state TenantAdminState) []TenantLine {
	out := []TenantLine{{Text: "tenant:", Heading: true}}
	if state.Tenant == nil {
		return append(out, tenantAbsent(state.TenantErr, state.Loaded)...)
	}
	t := *state.Tenant
	out = append(out,
		tenantRow("key", t.Key),
		tenantRow("name", orNone(t.Name)),
		tenantRow(attrTheme, orNone(t.Theme)),
		tenantRow(attrMFA, mfaLine(t)),
		tenantRow(attrArtifactMax, orInherit(artifactMaxLine(t))),
		tenantRow(attrArtifactTypes, orInherit(strings.Join(t.ArtifactTypes, ", "))),
		tenantRow(attrSessionTTL, orInherit(durationLine(t.SessionTTL))),
		tenantRow(attrSessionIdle, orInherit(durationLine(t.SessionIdleTimeout))),
		tenantRow(attrSessionRemember, orInherit(rememberLine(t))))
	if state.Handle != "" {
		out = append(out, tenantRow("actor", "@"+state.Handle))
	}
	return out
}

// tenantListing names the tenants this session can see. It is one row in the
// ordinary case, because listing tenants is scoped to the caller's own, and a
// listing that says how many there are cannot be mistaken for a broken read.
func tenantListing(state TenantAdminState) []TenantLine {
	out := []TenantLine{{Text: "tenants (" + strconv.Itoa(len(state.Tenants)) + "):", Heading: true}}
	if len(state.Tenants) == 0 {
		return append(out, tenantAbsent(state.TenantsErr, state.Loaded)...)
	}
	for _, t := range state.Tenants {
		out = append(out, TenantLine{Text: "  " + pad(t.Key, tenantLabelWidth) + orNone(t.Name)})
	}
	return out
}

// domainLines list the hostnames that resolve to this tenant.
func domainLines(state TenantAdminState, rows []TenantRow) []TenantLine {
	out := []TenantLine{{Text: "domains (" + strconv.Itoa(len(state.Domains)) + "):", Heading: true}}
	if len(state.Domains) == 0 {
		return append(out, tenantAbsent(state.DomainErr, state.Loaded)...)
	}
	for i, d := range state.Domains {
		selected := selectedRow(rows, state.Selected) == i
		out = append(out, TenantLine{
			Text:     SelectionMarker(selected) + pad(d.Hostname, tenantRowWidth) + domainNote(d),
			Selected: selected,
		})
	}
	return out
}

// memberLines list the actors who belong to this tenant, with the role each
// holds.
func memberLines(state TenantAdminState, rows []TenantRow) []TenantLine {
	out := []TenantLine{{Text: "members (" + strconv.Itoa(len(state.Members)) + "):", Heading: true}}
	if len(state.Members) == 0 {
		return append(out, tenantAbsent(state.MemberErr, state.Loaded)...)
	}
	for i, m := range state.Members {
		selected := selectedRow(rows, state.Selected) == len(state.Domains)+i
		label := MemberLabel(m.ActorID, state.Handles)
		out = append(out, TenantLine{
			Text:     SelectionMarker(selected) + pad(label, tenantRowWidth) + string(m.Role),
			Selected: selected,
		})
	}
	return out
}

// membershipLines list the reader's tenants with the role held in each and
// what choosing one does: switch, or the login or ssh command that reaches it.
func membershipLines(state TenantAdminState, rows []TenantRow) []TenantLine {
	listed := listedMemberships(state)
	if len(listed) == 0 {
		return nil
	}
	out := []TenantLine{{Text: "memberships (" + strconv.Itoa(len(listed)) + "):", Heading: true}}
	first := len(state.Domains) + len(state.Members)
	for i, m := range listed {
		selected := selectedRow(rows, state.Selected) == first+i
		out = append(out, TenantLine{
			Text: SelectionMarker(selected) + pad(m.TenantKey, tenantLabelWidth) +
				pad(string(m.Role), 8) + membershipNote(state, m.TenantKey),
			Selected: selected,
		})
	}
	return out
}

// membershipNote says what choosing one of the reader's tenants does.
func membershipNote(state TenantAdminState, tenant string) string {
	switch {
	case strings.EqualFold(tenant, state.Current):
		return "current"
	case state.OverSSH:
		return SSHTenantHint(tenant)
	case !state.CanSwitch:
		return "tix tenant use " + tenant
	case state.Reachable != nil && !state.Reachable(tenant):
		return "no credential: tix login --tenant " + tenant
	default:
		return "enter switches"
	}
}

// selectedRow bounds a selection to the rows that exist, answering -1 when
// there are none, so a stale index cannot mark a row the screen no longer draws.
func selectedRow(rows []TenantRow, sel int) int {
	if sel < 0 || sel >= len(rows) {
		return -1
	}
	return sel
}

// domainNote says what a domain is beyond its hostname: whether it has been
// verified, and the certificate it presents.
func domainNote(d core.Domain) string {
	verified := "unverified"
	if d.VerifiedAt != nil {
		verified = "verified"
	}
	mode := string(d.CertMode)
	if mode == "" {
		mode = string(core.CertNone)
	}
	return verified + ", cert " + mode
}

// tenantAbsent says why a listing is not shown, distinguishing a refusal from
// an empty tenant and both from a read that has not come back.
func tenantAbsent(reason string, loaded bool) []TenantLine {
	switch {
	case reason != "":
		return []TenantLine{{Text: "  not shown: " + reason, Dim: true}}
	case !loaded:
		return []TenantLine{{Text: "  reading...", Dim: true}}
	default:
		return []TenantLine{{Text: "  none", Dim: true}}
	}
}

// switchLines are what the screen has always said about switching tenant,
// under a heading of their own now that there is a screen above them. The first
// four lines of TenantViewLines are its own heading and the current tenant,
// which the attributes above already state.
func switchLines(state TenantAdminState) []TenantLine {
	out := []TenantLine{{Text: "switch:", Heading: true}}
	if state.OverSSH {
		return append(out,
			TenantLine{Text: "  Over ssh the username picks the tenant. Reconnect as", Dim: true},
			TenantLine{Text: "  " + SSHTenantHint("TENANT") + " to work in another of your tenants.", Dim: true})
	}
	raw := TenantViewLines(state.Current, state.Handle, state.CanSwitch)
	for _, l := range raw {
		if l == "tenant" || l == "" || isTenantHeader(l) {
			continue
		}
		out = append(out, TenantLine{Text: l, Dim: true})
	}
	return out
}

// isTenantHeader reports whether a line of TenantViewLines is one of the two
// facts the attributes section now states above it.
func isTenantHeader(line string) bool {
	return strings.HasPrefix(line, "  current:") || strings.HasPrefix(line, "  actor:")
}

// tenantRow is one aligned attribute line.
func tenantRow(label, value string) TenantLine {
	return TenantLine{Text: "  " + pad(label, tenantLabelWidth) + value}
}

// ThemeOptions are the palettes a tenant may present itself with: the names
// the build carries plus the word for having none, so clearing a theme is a
// choice on the same list rather than an attribute with no way back, and
// custom, which asks for any #rrggbb accent.
func ThemeOptions() []string {
	names := core.ThemeNames()
	out := make([]string, 0, len(names)+2)
	out = append(out, noTheme)
	out = append(out, names...)
	return append(out, customColour)
}

// ThemeValue turns a chosen theme back into what the service takes.
func ThemeValue(option string) string {
	if option == noTheme {
		return ""
	}
	return option
}

// TenantEditForm offers the tenant attributes an edit can change. The theme is
// answered here, from the palettes the build carries; the name is free text and
// hands over to the single-line prompt, seeded with the value it would replace.
func TenantEditForm(t core.Tenant) Form {
	attributes := []string{attrName, attrTheme, attrMFA, attrMFAGrace, attrArtifactMax, attrArtifactTypes,
		attrSessionTTL, attrSessionIdle, attrSessionRemember}
	theme := t.Theme
	if _, custom := core.CustomTheme(theme); custom {
		theme = customColour
	} else if theme == "" {
		theme = noTheme
	}
	return Form{
		Kind: formTenant, Title: "edit tenant " + t.Key,
		Fields: []FormField{
			{Key: "attribute", Label: "attribute", Options: attributes, Value: attributes[0]},
			{Key: attrTheme, Label: "theme", Options: ThemeOptions(), Value: theme,
				Needs: FieldCondition{Key: "attribute", Value: attrTheme}},
			{Key: attrMFA, Label: "policy", Options: mfaPolicyOptions(), Value: mfaPolicyValue(t),
				Needs: FieldCondition{Key: "attribute", Value: attrMFA}},
			{Key: attrMFAGrace, Label: "grace", Options: MFAGraceOptions, Value: graceValue(t),
				Needs: FieldCondition{Key: "attribute", Value: attrMFAGrace}},
		},
	}
}

// The subjects one key adds, named once so the form and the branch it releases
// cannot spell one differently.
const (
	addDomain = "domain"
	addMember = "member"
)

// TenantAddForm asks which of the two additions the key meant, offering only
// what this reader may perform.
func TenantAddForm(mayDomain, mayMember bool) Form {
	var subjects []string
	if mayDomain {
		subjects = append(subjects, addDomain)
	}
	if mayMember {
		subjects = append(subjects, addMember)
	}
	if len(subjects) == 0 {
		return Form{}
	}
	return Form{
		Kind: formTenantAdd, Title: "add to tenant",
		Fields: []FormField{{Key: "subject", Label: "subject", Options: subjects, Value: subjects[0]}},
	}
}

// RoleOptions are the roles a membership may hold, least authority first, so
// the answer a reader accepts without reading is the narrowest one.
func RoleOptions() []string {
	return []string{string(core.RoleViewer), string(core.RoleMember), string(core.RoleAdmin)}
}

// MemberOptions are the tenant's directory as the member form offers it. It is
// the assignee picker's list without the word for nobody, because a membership
// has to name somebody.
func MemberOptions(actors []core.Actor) []string {
	out := make([]string, 0, len(actors))
	for _, a := range actors {
		if label := ActorLabel(a); label != "" && !slices.Contains(out, label) {
			out = append(out, label)
		}
	}
	return out
}

// MemberForm picks the actor a membership is for and the role it holds. Both
// answers are drawn from fixed sets: a reader knows a colleague by handle and
// has never seen the identifier the service stores, and a role is one of three.
func MemberForm(actors []core.Actor) Form {
	handles := MemberOptions(actors)
	if len(handles) == 0 {
		return Form{}
	}
	return Form{
		Kind: formMember, Title: "add member",
		Fields: []FormField{
			{Key: "actor", Label: "actor", Options: handles, Value: handles[0]},
			{Key: "role", Label: "role", Options: RoleOptions(), Value: string(core.RoleMember)},
		},
	}
}

// DomainNote says what a terminal does not gather about a domain, so a reader
// who needs a certificate learns it here rather than by finding the domain
// serving none.
const DomainNote = "a certificate is a pair of paths on the server; tix domain add takes them"

// mfaPolicyOptions are the policies the edit form offers, in their wire form.
func mfaPolicyOptions() []string {
	out := make([]string, 0, len(core.MFAPolicies))
	for _, p := range core.MFAPolicies {
		out = append(out, string(p))
	}
	return out
}

// mfaPolicyValue is the policy the form starts on.
func mfaPolicyValue(t core.Tenant) string {
	if t.MFARequired() {
		return string(core.MFAPolicyRequired)
	}
	return string(core.MFAPolicyOff)
}

// graceValue is the grace the form starts on, the nearest it offers.
func graceValue(t core.Tenant) string {
	if t.MFAGrace == 0 {
		return MFAGraceOptions[0]
	}
	text := t.MFAGrace.Human()
	for _, o := range MFAGraceOptions {
		if o == text {
			return o
		}
	}
	return MFAGraceOptions[0]
}

// mfaLine states the tenant's second factor policy on the tenant screen.
func mfaLine(t core.Tenant) string {
	if !t.MFARequired() {
		return "off"
	}
	if t.MFAGrace == 0 {
		return "required"
	}
	return "required, grace " + t.MFAGrace.Human()
}

// artifactMaxLine is the tenant's own artifact size limit, empty when it
// inherits the server's.
func artifactMaxLine(t core.Tenant) string {
	switch n := t.ArtifactMaxBytes; {
	case n == 0:
		return ""
	case n%(1<<20) == 0:
		return strconv.FormatInt(n>>20, 10) + "MiB"
	case n%(1<<10) == 0:
		return strconv.FormatInt(n>>10, 10) + "KiB"
	default:
		return strconv.FormatInt(n, 10)
	}
}

// orInherit names an unset tenant limit as the server's.
func orInherit(s string) string {
	if s == "" {
		return "server's"
	}
	return s
}

// durationLine is one of the tenant's own session durations, empty when it
// inherits the server's.
func durationLine(d core.Duration) string {
	if d <= 0 {
		return ""
	}
	return d.Human()
}

// rememberLine is the tenant's own "keep me signed in" setting: off, a
// duration, or empty when it inherits the server's.
func rememberLine(t core.Tenant) string {
	if t.SessionRememberOff {
		return core.SessionRememberOff
	}
	return durationLine(t.SessionRememberTTL)
}
