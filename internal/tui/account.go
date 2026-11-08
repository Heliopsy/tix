// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/onboard"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/qr"
)

// The account screen: who the session is signed in as, the second factor,
// the reader's own API tokens and SSH keys, and the newest page of the account
// log. It is the browser's Settings, Tokens, SSH keys and Your account screens
// in one view, because a terminal session is one person and what they come
// here to do is short.

// accountLogLimit is how much of the account log one screen reads. Older
// entries are a question for tix account log --cursor, and the view says so.
const accountLogLimit = 50

// accountState is everything the account screen holds between frames.
type accountState struct {
	read *accountMsg
	sel  int
	// reveal is a value shown once: an issued token or a set of recovery
	// codes. It stays until the reader dismisses it or reopens the view, and
	// never goes to the status bar, where the next message would replace it
	// before it was copied.
	reveal *AccountReveal
	// enrolment is a second factor begun and not yet confirmed. Its secret is
	// handed over once, so it is held here until the code is confirmed or the
	// reader abandons it.
	enrolment *core.TOTPEnrolment
	// picker is the token scope picker, open between the token form and the
	// call that issues the token.
	picker *ScopePicker
}

// accountMsg carries the screen's reads. Each listing carries its own refusal,
// because the reads need different authorities and a reader refused one still
// gets the rest of the screen.
type accountMsg struct {
	tokens    []core.APIToken
	tokensErr string
	keys      []core.SSHKey
	keysErr   string
	totp      *core.TOTPStatus
	totpErr   string
	log       []core.AccountLogEntry
	logMore   bool
	logErr    string
}

// accountDoneMsg reports one of the screen's writes.
type accountDoneMsg struct {
	status    string
	failed    string
	err       error
	reveal    *AccountReveal
	enrolment *core.TOTPEnrolment
	// issued closes the scope picker, confirmed the enrolment: each only once
	// the call it was waiting for succeeded, so a refusal leaves the reader
	// where they were with what they had entered.
	issued    bool
	confirmed bool
}

// AccountReveal is a value the screen shows exactly once.
type AccountReveal struct {
	Title string
	Lines []string
}

// AccountLine is one rendered row of the account screen. Raw marks a line
// drawn with its own escapes, the QR code, which no style may wrap or cut.
type AccountLine struct {
	Text     string
	Heading  bool
	Dim      bool
	Selected bool
	Raw      bool
}

// AccountRowKind names what a selectable row of the account screen stands for.
type AccountRowKind int

// The selectable rows: the second factor, then tokens, keys and log entries.
const (
	accountRowFactor AccountRowKind = iota
	accountRowToken
	accountRowKey
	accountRowLog
)

// AccountRow is one row the cursor can rest on.
type AccountRow struct {
	Kind AccountRowKind
	// ID addresses a token or key for the service call.
	ID string
	// Label names the subject the way the confirmation has to say it.
	Label string
	// Active is false for a revoked or expired credential, which has nothing
	// left to revoke.
	Active bool
	// Retiring marks a rotated token still inside its grace window: it can be
	// revoked, but its replacement is the one to rotate.
	Retiring bool
}

// AccountState is everything the account screen renders from.
type AccountState struct {
	Handle   string
	Tenant   string
	Loaded   bool
	Selected int
	Now      time.Time
	Time     output.TimeStyle
	Reveal   *AccountReveal

	Tokens    []core.APIToken
	TokensErr string
	Keys      []core.SSHKey
	KeysErr   string
	TOTP      *core.TOTPStatus
	TOTPErr   string
	Log       []core.AccountLogEntry
	LogMore   bool
	LogErr    string
}

// accountLabelWidth aligns the attribute names of the identity and factor
// sections.
const accountLabelWidth = 14

// accountNameWidth aligns a token's name or a key's label against what follows.
const accountNameWidth = 20

// AccountRows are the rows the cursor runs over, in the order they are drawn.
// A section that was refused contributes none.
func AccountRows(s AccountState) []AccountRow {
	var out []AccountRow
	if s.TOTP != nil {
		out = append(out, AccountRow{Kind: accountRowFactor, Label: "your second factor", Active: s.TOTP.Enrolled})
	}
	for _, t := range s.Tokens {
		out = append(out, AccountRow{Kind: accountRowToken, ID: t.ID, Label: "token " + t.Name,
			Active: t.Active(s.Now), Retiring: t.Retiring(s.Now)})
	}
	for _, k := range s.Keys {
		out = append(out, AccountRow{Kind: accountRowKey, ID: k.ID, Label: "key " + keyName(k), Active: k.Active()})
	}
	for range s.Log {
		out = append(out, AccountRow{Kind: accountRowLog})
	}
	return out
}

// keyName is how a key is named on screen: its label, or its fingerprint when
// it carries none.
func keyName(k core.SSHKey) string {
	if strings.TrimSpace(k.Label) != "" {
		return k.Label
	}
	return k.Fingerprint
}

// AccountView renders the screen: a value shown once, then who the session is,
// the second factor, the tokens, the keys and the account log.
func AccountView(s AccountState) []AccountLine {
	var out []AccountLine
	if s.Reveal != nil {
		out = append(out, AccountLine{Text: s.Reveal.Title, Heading: true})
		for _, l := range s.Reveal.Lines {
			out = append(out, AccountLine{Text: "  " + l})
		}
		out = append(out, AccountLine{Text: "  esc dismisses this; it is not shown again", Dim: true}, AccountLine{})
	}
	out = append(out, AccountLine{Text: "signed in as:", Heading: true},
		accountAttr("actor", "@"+orNone(s.Handle)), accountAttr("tenant", orUnknown(s.Tenant)), AccountLine{})
	sel, row := s.Selected, 0
	mark := func() bool { selected := row == sel; row++; return selected }
	out = append(out, factorLines(s, mark)...)
	out = append(out, AccountLine{})
	out = append(out, tokenLines(s, mark)...)
	out = append(out, AccountLine{})
	out = append(out, keyLines(s, mark)...)
	out = append(out, AccountLine{})
	return append(out, logLines(s, mark)...)
}

// accountAttr is one aligned attribute line.
func accountAttr(label, value string) AccountLine {
	return AccountLine{Text: "  " + pad(label, accountLabelWidth) + value}
}

// accountAbsent says why a section is not shown, distinguishing a refusal from
// an empty listing and both from a read that has not come back.
func accountAbsent(reason string, loaded bool) []AccountLine {
	switch {
	case reason != "":
		return []AccountLine{{Text: "  not shown: " + reason, Dim: true}}
	case !loaded:
		return []AccountLine{{Text: "  reading...", Dim: true}}
	default:
		return []AccountLine{{Text: "  none", Dim: true}}
	}
}

// factorLines state the second factor: whether it is on, how many recovery
// codes are left, where the tenant requires one and whether verification is
// locked.
func factorLines(s AccountState, mark func() bool) []AccountLine {
	out := []AccountLine{{Text: "second factor:", Heading: true}}
	if s.TOTP == nil {
		return append(out, accountAbsent(s.TOTPErr, s.Loaded)...)
	}
	t := *s.TOTP
	selected := mark()
	out = append(out, AccountLine{Text: SelectionMarker(selected) + pad("status", accountLabelWidth) + FactorState(t, s.Time),
		Selected: selected})
	out = append(out, accountAttr("required", FactorRequired(t)))
	if t.VerificationLocked && t.VerificationLockEnd != nil {
		out = append(out, accountAttr("locked", "too many wrong codes; until "+s.Time.Format(*t.VerificationLockEnd)))
	}
	return out
}

// FactorState says in words whether the second factor is on.
func FactorState(t core.TOTPStatus, style output.TimeStyle) string {
	switch {
	case t.Enrolled:
		state := "on"
		if t.ConfirmedAt != nil {
			state += " since " + style.Format(*t.ConfirmedAt)
		}
		return state + ", " + counted(t.RecoveryCodesLeft, "recovery code", "recovery codes") + " left"
	case t.Pending:
		return "off; an enrolment was begun and never confirmed, and it grants nothing"
	default:
		return "off"
	}
}

// FactorRequired names the surfaces the tenant asks a second factor on.
func FactorRequired(t core.TOTPStatus) string {
	var on []string
	if t.RequiredWeb {
		on = append(on, "web")
	}
	if t.RequiredCLI {
		on = append(on, "cli")
	}
	if t.RequiredSSH {
		on = append(on, "ssh")
	}
	if len(on) == 0 {
		return "not required by this tenant"
	}
	return "by this tenant to sign in on " + strings.Join(on, ", ")
}

// tokenLines list the reader's own API tokens, revoked and expired ones
// included, with what each may do and when it stops.
func tokenLines(s AccountState, mark func() bool) []AccountLine {
	out := []AccountLine{{Text: "tokens (" + strconv.Itoa(len(s.Tokens)) + "):", Heading: true}}
	if len(s.Tokens) == 0 {
		return append(out, accountAbsent(s.TokensErr, s.Loaded)...)
	}
	for _, t := range s.Tokens {
		selected := mark()
		out = append(out, AccountLine{Text: SelectionMarker(selected) + pad(t.Name, accountNameWidth) + TokenSummary(t, s.Now, s.Time),
			Selected: selected, Dim: !t.Active(s.Now)})
	}
	return out
}

// TokenSummary states a token on one line: its scopes, when it stops working,
// when it was last used, and whether it still works at all.
func TokenSummary(t core.APIToken, now time.Time, style output.TimeStyle) string {
	scopes := make([]string, 0, len(t.Scopes))
	for _, sc := range t.Scopes {
		scopes = append(scopes, string(sc))
	}
	parts := []string{strings.Join(scopes, " ")}
	if t.ProjectID != "" {
		parts = append(parts, "one project")
	}
	switch {
	case t.Retiring(now):
		parts = append(parts, "rotated, stops "+style.Format(*t.RevokedAt))
	case t.RevokedAt != nil:
		parts = append(parts, "revoked "+style.Format(*t.RevokedAt))
	case t.ExpiresAt == nil:
		parts = append(parts, "never expires")
	case !t.ExpiresAt.After(now):
		parts = append(parts, "expired "+style.Format(*t.ExpiresAt))
	default:
		parts = append(parts, "expires "+style.Format(*t.ExpiresAt))
	}
	return strings.Join(append(parts, "last used "+usedAt(t.LastUsedAt, style)), " · ")
}

// usedAt renders a last-use time, or never.
func usedAt(at *time.Time, style output.TimeStyle) string {
	if at == nil {
		return "never"
	}
	return style.Format(*at)
}

// keyLines list the reader's own SSH keys, revoked ones included: a key that
// stopped working is the one somebody came here about.
func keyLines(s AccountState, mark func() bool) []AccountLine {
	out := []AccountLine{{Text: "ssh keys (" + strconv.Itoa(len(s.Keys)) + "):", Heading: true}}
	if len(s.Keys) == 0 {
		return append(out, accountAbsent(s.KeysErr, s.Loaded)...)
	}
	for _, k := range s.Keys {
		selected := mark()
		label := k.Label
		if strings.TrimSpace(label) == "" {
			label = "(no label)"
		}
		out = append(out, AccountLine{Text: SelectionMarker(selected) + pad(label, accountNameWidth) + KeySummary(k, s.Time),
			Selected: selected, Dim: !k.Active()})
	}
	return out
}

// KeySummary states a key on one line: its fingerprint, when it was enrolled,
// when it was last used, and whether it was revoked.
func KeySummary(k core.SSHKey, style output.TimeStyle) string {
	parts := []string{k.Fingerprint, "added " + style.Format(k.CreatedAt), "last used " + usedAt(k.LastUsedAt, style)}
	if k.RevokedAt != nil {
		parts = append(parts, "revoked "+style.Format(*k.RevokedAt))
	}
	return strings.Join(parts, " · ")
}

// logLines list the newest page of the account log, the way the browser's
// Your account screen does.
func logLines(s AccountState, mark func() bool) []AccountLine {
	out := []AccountLine{{Text: "account log, newest first:", Heading: true}}
	if len(s.Log) == 0 {
		if s.LogErr == "" && s.Loaded {
			return append(out, AccountLine{Text: "  nothing has changed yet", Dim: true})
		}
		return append(out, accountAbsent(s.LogErr, s.Loaded)...)
	}
	for _, e := range s.Log {
		selected := mark()
		out = append(out, AccountLine{Text: SelectionMarker(selected) + AccountLogRow(e, s.Time), Selected: selected})
	}
	if s.LogMore {
		out = append(out, AccountLine{Text: "  older entries: tix account log --cursor", Dim: true})
	}
	return out
}

// GroupKey breaks a shared secret into groups of four, which is how an
// authenticator asks for it when it is typed rather than scanned.
func GroupKey(secret string) string {
	var out strings.Builder
	for i, r := range secret {
		if i > 0 && i%4 == 0 {
			out.WriteString(" ")
		}
		out.WriteRune(r)
	}
	return out.String()
}

// EnrolmentQR draws the otpauth URI as a terminal QR code, or says why it was
// not drawn. A code needs colour, because the renderer states its own colours
// and a terminal stripping escapes would get a block of identical characters,
// and it needs room for the whole symbol, because half a symbol does not scan.
func EnrolmentQR(uri string, colour bool, width, height int) ([]string, string) {
	symbol, err := qr.Encode(uri)
	if err != nil {
		return nil, "the enrolment will not fit in a scannable code; type the key instead"
	}
	if !colour {
		return nil, "this terminal gets no colour, so no scannable code is drawn; type the key instead"
	}
	lines := strings.Split(strings.TrimSuffix(symbol.Terminal(), "\n"), "\n")
	needW := symbol.Size() + 2*qr.QuietZone + 2
	needH := len(lines) + enrolmentTextLines
	if width < needW || height < needH {
		return nil, fmt.Sprintf("a scannable code needs a terminal at least %dx%d; type the key, or enlarge the terminal",
			needW, needH+ChromeHeight)
	}
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return lines, ""
}

// enrolmentTextLines is what the enrolment screen draws besides the code: the
// heading, the key, the account, the URI and the instruction.
const enrolmentTextLines = 7

// EnrolmentView renders a begun enrolment: the code where one could be drawn,
// and always the key and the URI as text, so enrolling never depends on it.
func EnrolmentView(e core.TOTPEnrolment, code []string, note string) []AccountLine {
	out := []AccountLine{{Text: "second factor enrolment:", Heading: true}}
	for _, l := range code {
		out = append(out, AccountLine{Text: l, Raw: true})
	}
	out = append(out,
		accountAttr("key", GroupKey(e.Secret)),
		accountAttr("account", e.Account+" on "+e.Issuer),
		accountAttr("uri", e.URI))
	if note != "" {
		out = append(out, AccountLine{Text: "  " + note, Dim: true})
	}
	return append(out,
		AccountLine{Text: "  Scan the code or type the key into an authenticator app,", Dim: true},
		AccountLine{Text: "  then press enter to type the code it shows; esc abandons the enrolment.", Dim: true})
}

// PickRow is one row of the token scope picker: a group heading when Scope is
// empty, a scope otherwise.
type PickRow struct {
	Group string
	Scope core.Scope
	Note  string
	Held  bool
}

// ScopePicker is the browser's scope picker in the terminal: the same groups in
// the same order, each scope with the same note, and a scope the reader does
// not hold drawn and marked rather than left out.
type ScopePicker struct {
	Name   string
	Expiry onboard.TokenExpiry
	Rows   []PickRow
	Ticked map[core.Scope]bool
	// Sel indexes Rows, and only ever rests on a scope.
	Sel int
}

// everythingGroup is the group * is offered under, to a reader who holds it.
const everythingGroup = "Everything"

// allNote is what * allows, with the warning the browser draws beside it.
const allNote = "Every scope you hold, now and later. Pick the narrowest set that works instead where you can."

// NewScopePicker builds the picker for an actor, with chosen ticked.
func NewScopePicker(actor *core.Actor, name string, expiry onboard.TokenExpiry, chosen []core.Scope) ScopePicker {
	p := ScopePicker{Name: name, Expiry: expiry, Ticked: map[core.Scope]bool{}}
	held := func(s core.Scope) bool { return actor != nil && actor.HasScope(s) }
	if held(core.ScopeAll) {
		p.Rows = append(p.Rows, PickRow{Group: everythingGroup},
			PickRow{Group: everythingGroup, Scope: core.ScopeAll, Note: allNote, Held: true})
	}
	for _, g := range onboard.ScopeGroups {
		p.Rows = append(p.Rows, PickRow{Group: g.Name})
		for _, s := range g.Scopes {
			p.Rows = append(p.Rows, PickRow{Group: g.Name, Scope: s, Note: onboard.ScopeNotes[s], Held: held(s)})
		}
	}
	for _, s := range chosen {
		if held(s) {
			p.Ticked[s] = true
		}
	}
	p.Sel = p.step(-1, 1)
	return p
}

// step finds the next scope row from one past from, in direction dir, staying
// put at either end.
func (p ScopePicker) step(from, dir int) int {
	for i := from + dir; i >= 0 && i < len(p.Rows); i += dir {
		if p.Rows[i].Scope != "" {
			return i
		}
	}
	if from < 0 || from >= len(p.Rows) {
		return 0
	}
	return from
}

// Move steps the cursor to the previous or next scope.
func (p ScopePicker) Move(dir int) ScopePicker {
	p.Sel = p.step(p.Sel, dir)
	return p
}

// Toggle ticks or clears the scope under the cursor, refusing one the reader
// does not hold.
func (p ScopePicker) Toggle() (ScopePicker, error) {
	row := p.Rows[p.Sel]
	if !row.Held {
		return p, core.Forbidden("you do not hold %s, so a token you issue cannot carry it", row.Scope)
	}
	ticked := copyTicks(p.Ticked)
	ticked[row.Scope] = !ticked[row.Scope]
	p.Ticked = ticked
	return p, nil
}

// ToggleGroup ticks every held scope of the group under the cursor, or clears
// them all when every one is already ticked: the browser's select-all.
func (p ScopePicker) ToggleGroup() ScopePicker {
	group := p.Rows[p.Sel].Group
	all := true
	for _, r := range p.Rows {
		if r.Group == group && r.Scope != "" && r.Held && !p.Ticked[r.Scope] {
			all = false
		}
	}
	ticked := copyTicks(p.Ticked)
	for _, r := range p.Rows {
		if r.Group == group && r.Scope != "" && r.Held {
			ticked[r.Scope] = !all
		}
	}
	p.Ticked = ticked
	return p
}

// copyTicks copies the ticked set, so a model value is never mutated through a
// map it shares with an earlier one.
func copyTicks(in map[core.Scope]bool) map[core.Scope]bool {
	out := make(map[core.Scope]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Scopes are the ticked scopes, in the order the picker draws them.
func (p ScopePicker) Scopes() []core.Scope {
	var out []core.Scope
	for _, r := range p.Rows {
		if r.Scope != "" && p.Ticked[r.Scope] {
			out = append(out, r.Scope)
		}
	}
	return out
}

// scopeWidth aligns the notes beside the scope names.
const scopeWidth = 18

// Lines renders the picker under a heading naming the token it is for.
func (p ScopePicker) Lines() []AccountLine {
	expiry := "expires in " + p.Expiry.Label
	if p.Expiry.Days == 0 {
		expiry = "never expires"
	}
	out := []AccountLine{{Text: "scopes for token " + p.Name + ", which " + expiry + ":", Heading: true}}
	for i, r := range p.Rows {
		if r.Scope == "" {
			out = append(out, AccountLine{}, AccountLine{Text: "  " + r.Group + ":", Heading: true})
			continue
		}
		box := "[ ]"
		switch {
		case !r.Held:
			box = "[-]"
		case p.Ticked[r.Scope]:
			box = "[x]"
		}
		note := r.Note
		if !r.Held {
			note = "(you do not hold it) " + note
		}
		text := SelectionMarker(i == p.Sel) + "  " + box + " " + pad(string(r.Scope), scopeWidth) + note
		out = append(out, AccountLine{Text: text, Selected: i == p.Sel, Dim: !r.Held})
	}
	return out
}

// The subjects the account screen's add key offers, named once so the form and
// the branch it releases cannot spell one differently.
const (
	addToken    = "API token"
	addKey      = "SSH key"
	addFactor   = "second factor"
	addRecovery = "recovery codes"
	presetNone  = "nothing"
)

// AccountAddForm asks what the add key meant, offering only what this reader
// may do and what the second factor's state allows: enrolling while none is
// on, replacing the recovery codes once one is.
func AccountAddForm(may func(string) bool, totp *core.TOTPStatus) Form {
	var subjects []string
	if may("CreateToken") {
		subjects = append(subjects, addToken)
	}
	if may("EnrolSSHKey") {
		subjects = append(subjects, addKey)
	}
	if totp != nil && !totp.Enrolled && may("BeginTOTPEnrolment") {
		subjects = append(subjects, addFactor)
	}
	if totp != nil && totp.Enrolled && may("RegenerateRecoveryCodes") {
		subjects = append(subjects, addRecovery)
	}
	if len(subjects) == 0 {
		return Form{}
	}
	return Form{
		Kind: formAccountAdd, Title: "add to your account",
		Fields: []FormField{{Key: "subject", Label: "add", Options: subjects, Value: subjects[0]}},
	}
}

// TokenForm gathers a token's name, its expiry from the browser's choices and
// the preset the scope picker starts from.
func TokenForm(actor *core.Actor) Form {
	expiries := make([]string, 0, len(onboard.TokenExpiries))
	value := ""
	for _, e := range onboard.TokenExpiries {
		expiries = append(expiries, e.Label)
		if e.Key == onboard.DefaultTokenExpiry {
			value = e.Label
		}
	}
	presets := []string{presetNone}
	for _, p := range onboard.Presets {
		if actor != nil && heldAll(actor, p.Scopes) {
			presets = append(presets, p.Name)
		}
	}
	return Form{
		Kind: formToken, Title: "issue a token",
		Note: "the scopes are picked next; the value is shown once",
		Fields: []FormField{
			{Key: "name", Label: "name", Kind: FieldText, Limit: 128},
			{Key: "expires", Label: "expires in", Options: expiries, Value: value},
			{Key: "preset", Label: "start from", Options: presets, Value: presetNone},
		},
	}
}

// heldAll reports whether the actor holds every scope.
func heldAll(actor *core.Actor, scopes []core.Scope) bool {
	for _, s := range scopes {
		if !actor.HasScope(s) {
			return false
		}
	}
	return true
}

// TokenExpiryFor resolves a chosen expiry label back to the choice it names.
func TokenExpiryFor(label string) (onboard.TokenExpiry, bool) {
	for _, e := range onboard.TokenExpiries {
		if e.Label == label {
			return e, true
		}
	}
	return onboard.TokenExpiry{}, false
}

// PresetScopes are the scopes a preset named on the token form ticks.
func PresetScopes(name string) []core.Scope {
	for _, p := range onboard.Presets {
		if p.Name == name {
			return p.Scopes
		}
	}
	return nil
}

// SSHKeyForm gathers a public key and a label.
func SSHKeyForm() Form {
	return Form{
		Kind: formSSHKey, Title: "enrol an ssh key",
		Note: "paste the line ssh-keygen wrote to the .pub file; the private key never leaves your machine",
		Fields: []FormField{
			{Key: "key", Label: "public key", Kind: FieldText, Limit: 16384},
			{Key: "label", Label: "label", Kind: FieldText, Limit: 128},
		},
	}
}

// TOTPCodeForm asks for the code an authenticator shows.
func TOTPCodeForm() Form {
	return Form{
		Kind: formTOTPCode, Title: "confirm the second factor",
		Note:   "the six digits the authenticator shows for this account",
		Fields: []FormField{{Key: "code", Label: "code", Kind: FieldText, Limit: 64}},
	}
}

// recoveryTitle introduces recovery codes shown once.
const recoveryTitle = "recovery codes, shown once: each signs you in once if you lose the authenticator"

// openAccount shows the account screen and reads what it states. Anything held
// from an earlier visit is dropped, a value shown once among it.
func (m Model) openAccount() (Model, tea.Cmd) {
	next := m.enterView(viewAccount)
	if next.view != viewAccount {
		return next, nil
	}
	next.acct = accountState{}
	return next, next.loadAccount()
}

// loadAccount reads the screen's sections, each on its own, asking only the
// ones this reader's authority reaches.
func (m Model) loadAccount() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, may := m.svc, m.ctx, m.mayPerform
	const refused = "your authority does not reach it"
	return func() tea.Msg {
		out := accountMsg{tokensErr: refused, keysErr: refused, totpErr: refused, logErr: refused}
		if may("ListTokens") {
			out.tokens, out.tokensErr = nil, ""
			if tokens, err := svc.ListTokens(ctx, ""); err != nil {
				out.tokensErr = err.Error()
			} else {
				out.tokens = tokens
			}
		}
		if may("ListSSHKeys") {
			out.keys, out.keysErr = nil, ""
			if keys, err := svc.ListSSHKeys(ctx, ""); err != nil {
				out.keysErr = err.Error()
			} else {
				out.keys = keys
			}
		}
		if may("TOTPStatus") {
			out.totpErr = ""
			if status, err := svc.TOTPStatus(ctx); err != nil {
				out.totpErr = err.Error()
			} else {
				out.totp = status
			}
		}
		if may("GetAccountLog") {
			out.logErr = ""
			page := core.Page{Limit: accountLogLimit, Direction: core.Descending}
			if log, next, err := svc.GetAccountLog(ctx, page); err != nil {
				out.logErr = err.Error()
			} else {
				out.log, out.logMore = log, next != ""
			}
		}
		return out
	}
}

// onAccount adopts a read, bounding the cursor to the rows that came back.
func (m Model) onAccount(msg accountMsg) (Model, tea.Cmd) {
	m.acct.read = &msg
	m.acct.sel = clamp(m.acct.sel, 0, len(m.accountRows())-1)
	return m, nil
}

// accountState is what the account screen renders from.
func (m Model) accountState() AccountState {
	s := AccountState{Tenant: m.tenantKey, Selected: m.acct.sel, Now: m.now(), Time: m.timeStyle, Reveal: m.acct.reveal}
	if m.actor != nil {
		s.Handle = m.actor.Handle
	}
	if r := m.acct.read; r != nil {
		s.Loaded = true
		s.Tokens, s.TokensErr, s.Keys, s.KeysErr = r.tokens, r.tokensErr, r.keys, r.keysErr
		s.TOTP, s.TOTPErr, s.Log, s.LogMore, s.LogErr = r.totp, r.totpErr, r.log, r.logMore, r.logErr
	}
	return s
}

// accountRows are the rows the cursor runs over.
func (m Model) accountRows() []AccountRow { return AccountRows(m.accountState()) }

// selectedAccountRow is the row the removal key acts on.
func (m Model) selectedAccountRow() (AccountRow, bool) {
	rows := m.accountRows()
	if m.acct.sel < 0 || m.acct.sel >= len(rows) {
		return AccountRow{}, false
	}
	return rows[m.acct.sel], true
}

// accountRemovable reports whether the selected row has something to revoke
// or remove, which is what the footer asks before offering the key.
func (m Model) accountRemovable() bool {
	row, ok := m.selectedAccountRow()
	return ok && row.Kind != accountRowLog && row.Active
}

// handleAccountKey routes a key to whichever part of the screen holds the
// keyboard: a begun enrolment, the scope picker, or the listing.
func (m Model) handleAccountKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case m.acct.enrolment != nil:
		return m.handleEnrolmentKey(msg)
	case m.acct.picker != nil:
		return m.handlePickerKey(msg)
	}
	rows := len(m.accountRows())
	switch {
	case key.Matches(msg, m.keys.Back):
		if m.acct.reveal != nil {
			m.acct.reveal = nil
			return m, nil
		}
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		m.acct.sel = clamp(m.acct.sel-1, 0, rows-1)
	case key.Matches(msg, m.keys.Down):
		m.acct.sel = clamp(m.acct.sel+1, 0, rows-1)
	case key.Matches(msg, m.keys.Top):
		m.acct.sel = 0
	case key.Matches(msg, m.keys.Bottom):
		m.acct.sel = max(0, rows-1)
	default:
		return m.handleAccountAction(msg)
	}
	return m, nil
}

// handleAccountAction runs the screen's actions, refusing a key this reader's
// authority does not reach rather than letting the service refuse it.
func (m Model) handleAccountAction(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	for _, a := range m.keys.accountBindings() {
		if key.Matches(msg, a.binding) && !a.permitted(m.permits()) {
			return m, nil
		}
	}
	switch {
	case key.Matches(msg, m.keys.New):
		form := AccountAddForm(m.mayPerform, m.accountState().TOTP)
		if !form.Open() {
			m.err = "there is nothing this session may add to the account"
			return m, nil
		}
		m.form, m.err = form, ""
		return m, nil
	case key.Matches(msg, m.keys.Delete):
		return m.confirmAccountRemoval()
	case key.Matches(msg, m.keys.Renew):
		return m.openRotateToken()
	}
	return m, nil
}

// openRotateToken asks how long the selected token's old value may keep
// working, refusing a row that is not a live token with nothing to replace.
func (m Model) openRotateToken() (Model, tea.Cmd) {
	row, ok := m.selectedAccountRow()
	switch {
	case !ok || row.Kind != accountRowToken:
		m.err = "select a token to rotate"
		return m, nil
	case row.Retiring:
		m.err = row.Label + " was already rotated; rotate its replacement"
		return m, nil
	case !row.Active:
		m.err = row.Label + " no longer works, so there is nothing to rotate"
		return m, nil
	case !m.mayPerform("RotateToken"):
		m.err = "this session may not rotate tokens"
		return m, nil
	}
	m.err = ""
	return m.openForm(RotateTokenForm(row.Label))
}

// RotateTokenForm asks how long a rotated token's old value keeps working,
// from the browser's choices, stopping it at once unless told otherwise.
func RotateTokenForm(label string) Form {
	graces := make([]string, 0, len(onboard.RotateGraces))
	for _, g := range onboard.RotateGraces {
		graces = append(graces, g.Label)
	}
	return Form{
		Kind: formRotateToken, Title: "rotate " + label,
		Note:   "a new token with the same name, scopes and lifetime replaces it; its value is shown once",
		Fields: []FormField{{Key: "grace", Label: "old token", Options: graces, Value: graces[0]}},
	}
}

// confirmAccountRemoval names the selected token, key or factor and waits for
// agreement, refusing a row with nothing left to remove.
func (m Model) confirmAccountRemoval() (Model, tea.Cmd) {
	row, ok := m.selectedAccountRow()
	switch {
	case !ok, row.Kind == accountRowLog:
		m.err = "select a token, a key or the second factor to remove"
		return m, nil
	case !row.Active && row.Kind == accountRowFactor:
		m.err = "no second factor is on, so there is nothing to remove"
		return m, nil
	case !row.Active:
		m.err = row.Label + " no longer works, so there is nothing to revoke"
		return m, nil
	}
	c := Confirm{Target: row.Label, credential: row.ID}
	switch row.Kind {
	case accountRowFactor:
		if !m.mayPerform("RemoveTOTP") {
			m.err = "this session may not remove the second factor"
			return m, nil
		}
		c.Kind, c.Note = confirmRemoveTOTP, "and every recovery code beside it"
	case accountRowToken:
		if !m.mayPerform("RevokeToken") {
			m.err = "this session may not revoke tokens"
			return m, nil
		}
		c.Kind = confirmRevokeToken
	default:
		if !m.mayPerform("RevokeSSHKey") {
			m.err = "this session may not revoke keys"
			return m, nil
		}
		c.Kind, c.Note = confirmRevokeKey, "sessions it holds end at the listener's idle timeout"
	}
	return m.askConfirm(c, "there is nothing named to remove")
}

// submitAccountForm performs what one of the account screen's forms gathered.
func (m Model) submitAccountForm(form Form) (Model, tea.Cmd) {
	switch form.Kind {
	case formAccountAdd:
		switch form.Value("subject") {
		case addToken:
			return m.openForm(TokenForm(m.actor))
		case addKey:
			return m.openForm(SSHKeyForm())
		case addFactor:
			return m, m.accountCall("starting enrolment", func(svc core.Service, ctx context.Context) accountDoneMsg {
				e, err := svc.BeginTOTPEnrolment(ctx)
				return accountDoneMsg{err: err, enrolment: e}
			})
		case addRecovery:
			return m.askConfirm(Confirm{Kind: confirmRecoveryCodes, Target: "your recovery codes",
				Note: "every code issued before stops working, used or not"}, "there is nothing named to replace")
		}
	case formToken:
		name := strings.TrimSpace(form.Value("name"))
		if name == "" {
			m.err = "a token needs a name"
			return m.openForm(form)
		}
		expiry, _ := TokenExpiryFor(form.Value("expires"))
		picker := NewScopePicker(m.actor, name, expiry, PresetScopes(form.Value("preset")))
		m.acct.picker, m.err = &picker, ""
		return m, nil
	case formRotateToken:
		row, ok := m.selectedAccountRow()
		if !ok || row.Kind != accountRowToken {
			m.err = "select a token to rotate"
			return m, nil
		}
		grace, _ := onboard.RotateGraceFor(form.Value("grace"))
		note := "the old value stops at once"
		if grace.Grace > 0 {
			note = "the old value keeps working for " + strings.ToLower(strings.TrimPrefix(grace.Label, "Keep it working for "))
		}
		return m.askConfirm(Confirm{Kind: confirmRotateToken, Target: row.Label, Note: note,
			credential: row.ID, grace: grace.Grace}, "there is nothing named to rotate")
	case formSSHKey:
		in := core.EnrolSSHKeyInput{PublicKey: strings.TrimSpace(form.Value("key")), Label: strings.TrimSpace(form.Value("label"))}
		return m, m.accountCall("enrolling the key", func(svc core.Service, ctx context.Context) accountDoneMsg {
			k, err := svc.EnrolSSHKey(ctx, in)
			if err != nil {
				return accountDoneMsg{err: err}
			}
			return accountDoneMsg{status: "key " + keyName(*k) + " enrolled"}
		})
	case formTOTPCode:
		code := strings.TrimSpace(form.Value("code"))
		return m, m.accountCall("confirming the code", func(svc core.Service, ctx context.Context) accountDoneMsg {
			codes, err := svc.ConfirmTOTPEnrolment(ctx, code)
			if err != nil {
				return accountDoneMsg{err: err}
			}
			return accountDoneMsg{status: "second factor on", confirmed: true,
				reveal: &AccountReveal{Title: recoveryTitle + ":", Lines: codes.Codes}}
		})
	}
	return m, nil
}

// runAccountConfirm makes the call a confirmation on the account screen
// released.
func (m Model) runAccountConfirm(c Confirm) tea.Cmd {
	id := c.credential
	switch c.Kind {
	case confirmRevokeToken:
		return m.accountCall("revoking the token", func(svc core.Service, ctx context.Context) accountDoneMsg {
			return accountDoneMsg{err: svc.RevokeToken(ctx, id), status: c.Target + " revoked"}
		})
	case confirmRotateToken:
		in := core.RotateTokenInput{Grace: core.Duration(c.grace)}
		return m.accountCall("rotating the token", func(svc core.Service, ctx context.Context) accountDoneMsg {
			issued, err := svc.RotateToken(ctx, id, in)
			if err != nil {
				return accountDoneMsg{err: err}
			}
			return accountDoneMsg{status: "token " + issued.Name + " rotated",
				reveal: &AccountReveal{Title: "token " + issued.Name + ", rotated and shown once; copy it now:", Lines: []string{issued.Token}}}
		})
	case confirmRevokeKey:
		return m.accountCall("revoking the key", func(svc core.Service, ctx context.Context) accountDoneMsg {
			return accountDoneMsg{err: svc.RevokeSSHKey(ctx, id), status: c.Target + " revoked"}
		})
	case confirmRemoveTOTP:
		return m.accountCall("removing the second factor", func(svc core.Service, ctx context.Context) accountDoneMsg {
			return accountDoneMsg{err: svc.RemoveTOTP(ctx), status: "second factor removed"}
		})
	case confirmRecoveryCodes:
		return m.accountCall("replacing the recovery codes", func(svc core.Service, ctx context.Context) accountDoneMsg {
			codes, err := svc.RegenerateRecoveryCodes(ctx)
			if err != nil {
				return accountDoneMsg{err: err}
			}
			return accountDoneMsg{status: "recovery codes replaced",
				reveal: &AccountReveal{Title: recoveryTitle + "; every earlier code has stopped working:", Lines: codes.Codes}}
		})
	}
	return nil
}

// accountCall runs one of the screen's writes off the update loop, naming what
// it was doing so a refusal says which call refused.
func (m Model) accountCall(doing string, call func(core.Service, context.Context) accountDoneMsg) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		out := call(svc, ctx)
		out.failed = doing
		return out
	}
}

// onAccountDone applies a write's outcome and reads the screen again. A
// refusal leaves the picker or the enrolment where it was, so what the reader
// entered is not lost.
func (m Model) onAccountDone(msg accountDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.failed + ": " + msg.err.Error()
		return m, nil
	}
	m.err, m.status = "", msg.status
	if msg.issued {
		m.acct.picker = nil
	}
	if msg.confirmed {
		m.acct.enrolment = nil
	}
	if msg.enrolment != nil {
		m.acct.enrolment = msg.enrolment
		m.status = "enrolment begun; confirm it with the code the authenticator shows"
	}
	if msg.reveal != nil {
		m.acct.reveal = msg.reveal
		m.acct.sel = 0
	}
	return m, m.loadAccount()
}

// handleEnrolmentKey drives a begun enrolment: enter asks for the code, back
// abandons it.
func (m Model) handleEnrolmentKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.acct.enrolment = nil
		m.status = "enrolment abandoned; it grants nothing until confirmed"
		return m, m.loadAccount()
	case key.Matches(msg, m.keys.Enter):
		return m.openForm(TOTPCodeForm())
	}
	return m, nil
}

// handlePickerKey drives the scope picker: move, tick one, tick a group, issue
// or cancel.
func (m Model) handlePickerKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	p := *m.acct.picker
	switch {
	case key.Matches(msg, m.keys.Back):
		m.acct.picker, m.status = nil, "no token issued"
		return m, nil
	case key.Matches(msg, m.keys.Up):
		p = p.Move(-1)
	case key.Matches(msg, m.keys.Down):
		p = p.Move(1)
	case key.Matches(msg, m.keys.ToggleRead):
		next, err := p.Toggle()
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		p, m.err = next, ""
	case key.Matches(msg, m.keys.ReadAll):
		p = p.ToggleGroup()
	case key.Matches(msg, m.keys.Accept):
		return m, m.issueToken(p)
	}
	m.acct.picker = &p
	return m, nil
}

// issueToken asks the service for the token the picker describes. The expiry is
// resolved against now here, as the browser resolves it when the form arrives.
func (m Model) issueToken(p ScopePicker) tea.Cmd {
	in := core.CreateTokenInput{Name: p.Name, Scopes: p.Scopes()}
	if p.Expiry.Days > 0 {
		at := m.now().UTC().AddDate(0, 0, p.Expiry.Days)
		in.ExpiresAt = &at
	}
	return m.accountCall("issuing the token", func(svc core.Service, ctx context.Context) accountDoneMsg {
		issued, err := svc.CreateToken(ctx, in)
		if err != nil {
			return accountDoneMsg{err: err}
		}
		return accountDoneMsg{status: "token " + issued.Name + " issued", issued: true,
			reveal: &AccountReveal{Title: "token " + issued.Name + ", shown once; copy it now:", Lines: []string{issued.Token}}}
	})
}

// accountLines renders the screen: a begun enrolment or the picker when one is
// open, the listing otherwise, scrolled so the selection stays on screen.
func (m Model) accountLines(layout Layout) []string {
	if e := m.acct.enrolment; e != nil {
		code, note := EnrolmentQR(e.URI, m.theme.Color, m.width, layout.BodyHeight)
		return m.accountWindow(EnrolmentView(*e, code, note), layout, 0)
	}
	if p := m.acct.picker; p != nil {
		raw := p.Lines()
		return m.accountWindow(raw, layout, selectedLine(raw))
	}
	raw := AccountView(m.accountState())
	return m.accountWindow(raw, layout, selectedLine(raw))
}

// selectedLine is the index of the selected line, or zero when none is.
func selectedLine(lines []AccountLine) int {
	for i, l := range lines {
		if l.Selected {
			return i
		}
	}
	return 0
}

// accountWindow draws the lines that fit, keeping the selected one on screen.
func (m Model) accountWindow(raw []AccountLine, layout Layout, sel int) []string {
	rows := VisibleRows(layout.BodyHeight, len(raw))
	offset := ScrollWindow(0, sel, rows, len(raw))
	out := make([]string, 0, rows+1)
	for i := offset; i < len(raw) && i < offset+rows; i++ {
		out = append(out, m.accountStyle(raw[i]))
	}
	if hint := ScrollHint(offset, rows, len(raw)); hint != "" {
		out = append(out, m.theme.Dim.Render("  "+hint))
	}
	return out
}

// accountStyle draws one line of the account screen by what it is.
func (m Model) accountStyle(l AccountLine) string {
	switch {
	case l.Raw:
		return l.Text
	case l.Selected:
		return m.fit(m.selection().Render(l.Text))
	case l.Heading:
		return m.fit(m.theme.Header.Render(l.Text))
	case l.Dim:
		return m.fit(m.theme.Dim.Render(l.Text))
	default:
		return m.fit(l.Text)
	}
}
