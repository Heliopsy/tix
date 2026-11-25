// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
)

// accountFake is the fake service with the account screen's reads scripted and
// its writes recorded. It wraps fakeService rather than growing it, so the
// account screen's tests stay beside the screen.
type accountFake struct {
	*fakeService

	tokens   []core.APIToken
	keys     []core.SSHKey
	totp     *core.TOTPStatus
	log      []core.AccountLogEntry
	logErr   error
	totpErr  error
	sessions []core.SessionInfo

	createdTokens []core.CreateTokenInput
	revokedTokens []string
	rotatedTokens map[string]core.RotateTokenInput
	enrolledKeys  []core.EnrolSSHKeyInput
	revokedKeys   []string
	revokedSess   []string
	codes         []string
	began         int
	removed       int
	regenerated   int
	confirmErr    error
}

func (f *accountFake) ListTokens(context.Context, string) ([]core.APIToken, error) {
	return f.tokens, nil
}
func (f *accountFake) ListSSHKeys(context.Context, string) ([]core.SSHKey, error) { return f.keys, nil }

func (f *accountFake) TOTPStatus(context.Context) (*core.TOTPStatus, error) {
	return f.totp, f.totpErr
}

func (f *accountFake) GetAccountLog(context.Context, core.Page) ([]core.AccountLogEntry, string, error) {
	return f.log, "", f.logErr
}

func (f *accountFake) ListSessions(context.Context, core.Page) ([]core.SessionInfo, string, error) {
	return f.sessions, "", nil
}

func (f *accountFake) RevokeSession(_ context.Context, id string) error {
	f.revokedSess = append(f.revokedSess, id)
	return nil
}

func (f *accountFake) CreateToken(_ context.Context, in core.CreateTokenInput) (*core.IssuedToken, error) {
	f.createdTokens = append(f.createdTokens, in)
	return &core.IssuedToken{APIToken: core.APIToken{ID: "tok-new", Name: in.Name, Scopes: in.Scopes},
		Token: "tix_secretvalue"}, nil
}

func (f *accountFake) RevokeToken(_ context.Context, id string) error {
	f.revokedTokens = append(f.revokedTokens, id)
	return nil
}

func (f *accountFake) RotateToken(_ context.Context, id string, in core.RotateTokenInput) (*core.IssuedToken, error) {
	if f.rotatedTokens == nil {
		f.rotatedTokens = map[string]core.RotateTokenInput{}
	}
	f.rotatedTokens[id] = in
	return &core.IssuedToken{APIToken: core.APIToken{ID: "tok-rotated", Name: "rotated"}, Token: "tix_rotatedvalue"}, nil
}

func (f *accountFake) EnrolSSHKey(_ context.Context, in core.EnrolSSHKeyInput) (*core.SSHKey, error) {
	f.enrolledKeys = append(f.enrolledKeys, in)
	return &core.SSHKey{ID: "key-new", Label: in.Label, Fingerprint: "SHA256:new"}, nil
}

func (f *accountFake) RevokeSSHKey(_ context.Context, id string) error {
	f.revokedKeys = append(f.revokedKeys, id)
	return nil
}

func (f *accountFake) BeginTOTPEnrolment(context.Context) (*core.TOTPEnrolment, error) {
	f.began++
	return &core.TOTPEnrolment{Secret: "JBSWY3DPEHPK3PXP",
		URI:    "otpauth://totp/Acme:ada@example.test?secret=JBSWY3DPEHPK3PXP&issuer=Acme",
		Issuer: "Acme", Account: "ada@example.test"}, nil
}

func (f *accountFake) ConfirmTOTPEnrolment(_ context.Context, code string) (*core.TOTPRecoveryCodes, error) {
	f.codes = append(f.codes, code)
	if f.confirmErr != nil {
		return nil, f.confirmErr
	}
	return &core.TOTPRecoveryCodes{Codes: []string{"aaaa-bbbb", "cccc-dddd"}}, nil
}

func (f *accountFake) RemoveTOTP(context.Context) error {
	f.removed++
	return nil
}

func (f *accountFake) RegenerateRecoveryCodes(context.Context) (*core.TOTPRecoveryCodes, error) {
	f.regenerated++
	return &core.TOTPRecoveryCodes{Codes: []string{"eeee-ffff"}}, nil
}

// accountService is a service holding one live token, one revoked token, one
// key, no second factor and two log entries.
func accountService() *accountFake {
	at := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	later := at.AddDate(1, 0, 0)
	gone := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	return &accountFake{
		fakeService: newFakeService(),
		tokens: []core.APIToken{
			{ID: "tok-ci", Name: "ci-bot", Scopes: []core.Scope{core.ScopeTaskRead}, CreatedAt: at, ExpiresAt: &later},
			{ID: "tok-old", Name: "old-bot", Scopes: []core.Scope{core.ScopeTaskWrite}, CreatedAt: gone, RevokedAt: &gone},
		},
		keys: []core.SSHKey{{ID: "key-laptop", Label: "laptop", Fingerprint: "SHA256:abc", CreatedAt: at}},
		sessions: []core.SessionInfo{{ID: "sess-firefox", CreatedAt: at, ExpiresAt: later, Method: "password",
			SecondFactor: true, UserAgent: "Firefox/140"}},
		totp: &core.TOTPStatus{RequiredSSH: true},
		log: []core.AccountLogEntry{
			{Seq: 2, Action: core.AccountMemberAdded, TenantKey: "acme", ByEmail: "root@example.test",
				ByGlobalAdmin: true, Via: core.ViaWeb, Detail: map[string]any{"role": "admin"}, OccurredAt: at},
			{Seq: 1, Action: core.AccountFactorEnrolled, Via: core.ViaCLILocal, OccurredAt: at},
		},
	}
}

// accountModel opens the account screen the way a reader does: with the key,
// draining the read it asked for.
func accountModel(t *testing.T) (Model, *accountFake) {
	t.Helper()
	m := boardModel(t)
	svc := accountService()
	m.svc, m.tenantKey = svc, "default"
	m.actor = &core.Actor{ID: "u-full", TenantID: "t", Kind: core.ActorUser, Role: core.RoleAdmin, Handle: "ada"}
	m.allowed = resolveActions(m.actor)
	m, cmd := m.reduce(pressKey("$"))
	m = runCmd(t, m, cmd)
	if m.view != viewAccount {
		t.Fatalf("$ did not open the account screen; view = %v", m.view)
	}
	return m, svc
}

// drainAccount runs a command and folds its message in, failing on no command.
func drainAccount(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	return runCmd(t, m, cmd)
}

// fill answers the open form's fields and accepts it.
func fill(t *testing.T, m Model, answers map[string]string) (Model, tea.Cmd) {
	t.Helper()
	if !m.form.Open() {
		t.Fatal("no form is open")
	}
	for k, v := range answers {
		m.form = m.form.SetValue(k, v)
	}
	if f, ok := m.form.Selected(); ok && f.Typed() {
		m.input.SetValue(m.form.value(f.Key))
	}
	return m.reduce(pressKey("enter"))
}

// selectRow moves the cursor to the first row of a kind.
func selectRow(t *testing.T, m Model, kind AccountRowKind, id string) Model {
	t.Helper()
	for i, r := range m.accountRows() {
		if r.Kind == kind && r.ID == id {
			m.acct.sel = i
			return m
		}
	}
	t.Fatalf("no %v row %q on the account screen", kind, id)
	return m
}

func TestTheAccountScreenListsWhatItRead(t *testing.T) {
	m, _ := accountModel(t)
	frame := m.Frame()

	for _, tc := range []struct{ heading, want string }{
		{"signed in as:", "@ada"},
		{"second factor:", "off"},
		{"second factor:", "by this tenant to sign in on ssh"},
		{"tokens (2):", "ci-bot"},
		{"tokens (2):", "task:read"},
		{"tokens (2):", "revoked"},
		{"ssh keys (1):", "SHA256:abc"},
		{"sessions (1):", "password + second factor"},
		{"sessions (1):", "Firefox/140"},
		{"account log, newest first:", "account.membership_added  in acme  by root@example.test as global admin  via web  role=admin"},
		{"account log, newest first:", "by the system  via cli-local"},
	} {
		if got := adminSection(t, frame, tc.heading); !strings.Contains(got, tc.want) {
			t.Errorf("the %q section does not say %q:\n%s", tc.heading, tc.want, got)
		}
	}
	if strings.Contains(adminSection(t, frame, "tokens (2):"), "laptop") {
		t.Error("the tokens section drew a key")
	}
}

func TestARefusedAccountSectionCostsNothingElse(t *testing.T) {
	m := boardModel(t)
	svc := accountService()
	svc.logErr = core.Invalid("an account log belongs to a person, and this caller is not one")
	svc.totp, svc.totpErr = nil, core.Invalid("not a person")
	m.svc = svc
	m, cmd := m.reduce(pressKey("$"))
	m = drainAccount(t, m, cmd)
	frame := m.Frame()

	if got := adminSection(t, frame, "account log, newest first:"); !strings.Contains(got, "not shown: ") || !strings.Contains(got, "an account log belongs to a person") {
		t.Errorf("the refused log does not say why:\n%s", got)
	}
	if got := adminSection(t, frame, "second factor:"); !strings.Contains(got, "not shown: invalid: not a person") {
		t.Errorf("the refused factor does not say why:\n%s", got)
	}
	if got := adminSection(t, frame, "tokens (2):"); !strings.Contains(got, "ci-bot") {
		t.Errorf("a refused log cost the token listing:\n%s", got)
	}
	if rows := m.accountRows(); len(rows) == 0 || rows[0].Kind != accountRowToken {
		t.Errorf("the cursor runs over a refused factor: %+v", rows)
	}
}

func TestATokenIsIssuedThroughTheScopePickerAndShownOnce(t *testing.T) {
	m, svc := accountModel(t)
	m, _ = m.reduce(pressKey("n"))
	if m.form.Kind != formAccountAdd || m.form.Value("subject") != addToken {
		t.Fatalf("n opened %v on %q, want the add form on a token", m.form.Kind, m.form.Value("subject"))
	}
	m, _ = m.reduce(pressKey("enter"))
	if m.form.Kind != formToken {
		t.Fatalf("choosing a token opened %v, want the token form", m.form.Kind)
	}
	if got := m.form.Value("expires"); got != "90 days" {
		t.Errorf("the token form proposes %q, want the browser's 90 days", got)
	}
	m, _ = fill(t, m, map[string]string{"name": "agent-1", "preset": "Agent"})
	if m.acct.picker == nil {
		t.Fatal("the token form did not open the scope picker")
	}
	if got := m.acct.picker.Scopes(); !slices.Equal(got, PresetScopes("Agent")) {
		t.Fatalf("the Agent preset ticked %v, want %v", got, PresetScopes("Agent"))
	}
	body := strings.Join(m.accountLines(LayoutFor(m.width, 200, 0)), "\n")
	for _, want := range []string{"Tasks:", "Administration:", "[x] task:read", "Read tasks, their comments and history.",
		"[-] global:admin", "(you do not hold it)"} {
		if !strings.Contains(body, want) {
			t.Errorf("the picker does not draw %q:\n%s", want, body)
		}
	}

	for m.acct.picker.Rows[m.acct.picker.Sel].Scope != core.ScopeEventSubscribe {
		m, _ = m.reduce(pressKey("down"))
	}
	m, _ = m.reduce(pressKey(" "))
	m, cmd := m.reduce(pressKey("enter"))
	m = drainAccount(t, m, cmd)

	if len(svc.createdTokens) != 1 {
		t.Fatalf("CreateToken was called %d times", len(svc.createdTokens))
	}
	in := svc.createdTokens[0]
	if in.Name != "agent-1" || !slices.Contains(in.Scopes, core.ScopeEventSubscribe) ||
		!slices.Contains(in.Scopes, core.ScopeTaskClaim) {
		t.Errorf("CreateToken got %+v", in)
	}
	if want := m.now().UTC().AddDate(0, 0, 90); in.ExpiresAt == nil || !in.ExpiresAt.Equal(want) {
		t.Errorf("the token expires at %v, want %v", in.ExpiresAt, want)
	}
	if m.acct.picker != nil {
		t.Error("the picker stayed open after the token was issued")
	}
	frame := m.Frame()
	if !strings.Contains(adminSection(t, frame, "token agent-1, shown once; copy it now:"), "tix_secretvalue") {
		t.Errorf("the issued value is not drawn:\n%s", frame)
	}
	if strings.Contains(m.statusBar(), "tix_secretvalue") {
		t.Error("the token value went to the status bar")
	}
	m, _ = m.reduce(pressKey("esc"))
	if m.view != viewAccount || strings.Contains(m.Frame(), "tix_secretvalue") {
		t.Errorf("esc did not dismiss the value and stay; view = %v", m.view)
	}
}

func TestTheScopePickerRefusesAScopeTheReaderLacks(t *testing.T) {
	m, _ := accountModel(t)
	m.form = TokenForm(m.actor)
	m, _ = fill(t, m, map[string]string{"name": "x"})
	for m.acct.picker.Rows[m.acct.picker.Sel].Scope != core.ScopeGlobalAdmin {
		m, _ = m.reduce(pressKey("down"))
	}
	m, _ = m.reduce(pressKey(" "))
	if m.acct.picker.Ticked[core.ScopeGlobalAdmin] {
		t.Fatal("a scope the reader does not hold was ticked")
	}
	if !strings.Contains(m.err, "you do not hold global:admin") {
		t.Errorf("the refusal says %q", m.err)
	}
}

func TestAGroupIsTickedAndClearedWhole(t *testing.T) {
	expiry, _ := TokenExpiryFor("90 days")
	p := NewScopePicker(fullActor(), "x", expiry, nil)
	for p.Rows[p.Sel].Group != "Projects" {
		p = p.Move(1)
	}
	p = p.ToggleGroup()
	if got := p.Scopes(); !slices.Equal(got, []core.Scope{core.ScopeProjectRead, core.ScopeProjectWrite}) {
		t.Fatalf("the group ticked %v", got)
	}
	if got := p.ToggleGroup().Scopes(); len(got) != 0 {
		t.Fatalf("a fully ticked group was not cleared: %v", got)
	}
}

func TestAKeyIsEnrolledFromItsForm(t *testing.T) {
	m, svc := accountModel(t)
	m.form = AccountAddForm(m.mayPerform, svc.totp)
	m, _ = fill(t, m, map[string]string{"subject": addKey})
	if m.form.Kind != formSSHKey {
		t.Fatalf("choosing a key opened %v", m.form.Kind)
	}
	m, cmd := fill(t, m, map[string]string{"key": "ssh-ed25519 AAAAC3 ada@laptop", "label": "desk"})
	m = drainAccount(t, m, cmd)
	if len(svc.enrolledKeys) != 1 || svc.enrolledKeys[0].PublicKey != "ssh-ed25519 AAAAC3 ada@laptop" ||
		svc.enrolledKeys[0].Label != "desk" || svc.enrolledKeys[0].ActorID != "" {
		t.Fatalf("EnrolSSHKey got %+v", svc.enrolledKeys)
	}
	if m.status != "key desk enrolled" {
		t.Errorf("status = %q", m.status)
	}
}

func TestRevokingNamesTheTokenOrKeyAndWaitsForAgreement(t *testing.T) {
	for _, tc := range []struct {
		kind  AccountRowKind
		id    string
		label string
		got   func(*accountFake) []string
	}{
		{accountRowToken, "tok-ci", "revoke token ci-bot?", func(f *accountFake) []string { return f.revokedTokens }},
		{accountRowKey, "key-laptop", "revoke key laptop,", func(f *accountFake) []string { return f.revokedKeys }},
		{accountRowSession, "sess-firefox", "revoke session sess-firefox,", func(f *accountFake) []string { return f.revokedSess }},
	} {
		m, svc := accountModel(t)
		m = selectRow(t, m, tc.kind, tc.id)
		m, _ = m.reduce(pressKey("X"))
		if !strings.Contains(m.confirm.Question(), tc.label) {
			t.Fatalf("X asked %q, want it to name %q", m.confirm.Question(), tc.label)
		}
		if len(tc.got(svc)) != 0 {
			t.Fatal("the revocation ran before agreement")
		}
		m, cmd := m.reduce(pressKey("y"))
		_ = drainAccount(t, m, cmd)
		if got := tc.got(svc); len(got) != 1 || got[0] != tc.id {
			t.Errorf("revoked %v, want %s", got, tc.id)
		}
	}
}

func TestARevokedTokenIsNotRevokedAgain(t *testing.T) {
	m, svc := accountModel(t)
	m = selectRow(t, m, accountRowToken, "tok-old")
	if m.actionContext().HasRows {
		t.Error("the footer offers a removal on a revoked token")
	}
	m, _ = m.reduce(pressKey("X"))
	if m.confirm.Open() || len(svc.revokedTokens) != 0 {
		t.Fatal("a revoked token was offered for revocation")
	}
	if !strings.Contains(m.err, "no longer works") {
		t.Errorf("the refusal says %q", m.err)
	}
}

func TestEnrolmentFallsBackToTheKeyWithoutColour(t *testing.T) {
	m, svc := accountModel(t)
	m.form = AccountAddForm(m.mayPerform, svc.totp)
	m, cmd := fill(t, m, map[string]string{"subject": addFactor})
	m = drainAccount(t, m, cmd)
	if svc.began != 1 || m.acct.enrolment == nil {
		t.Fatalf("enrolment did not begin: began = %d", svc.began)
	}
	frame := m.Frame()
	section := adminSection(t, frame, "second factor enrolment:")
	for _, want := range []string{"JBSW Y3DP EHPK 3PXP", "otpauth://totp/Acme:ada@example.test", "no colour"} {
		if !strings.Contains(section, want) {
			t.Errorf("the enrolment does not draw %q:\n%s", want, section)
		}
	}
	if strings.Contains(frame, "▀") {
		t.Error("a code was drawn on a terminal with no colour")
	}

	m, _ = m.reduce(pressKey("enter"))
	if m.form.Kind != formTOTPCode {
		t.Fatalf("enter opened %v, want the code form", m.form.Kind)
	}
	m, cmd = fill(t, m, map[string]string{"code": "123456"})
	m = drainAccount(t, m, cmd)
	if len(svc.codes) != 1 || svc.codes[0] != "123456" {
		t.Fatalf("ConfirmTOTPEnrolment got %v", svc.codes)
	}
	if m.acct.enrolment != nil {
		t.Error("the enrolment stayed open after it was confirmed")
	}
	if !strings.Contains(m.Frame(), "aaaa-bbbb") {
		t.Error("the recovery codes are not drawn")
	}
}

func TestARefusedCodeKeepsTheEnrolment(t *testing.T) {
	m, svc := accountModel(t)
	svc.confirmErr = core.Invalid("that code is not right")
	m.acct.enrolment, _ = svc.BeginTOTPEnrolment(context.TODO())
	m.form = TOTPCodeForm()
	m, cmd := fill(t, m, map[string]string{"code": "000000"})
	m, _ = m.reduce(cmd())
	if m.acct.enrolment == nil {
		t.Fatal("a refused code threw the enrolment away")
	}
	if !strings.Contains(m.err, "that code is not right") {
		t.Errorf("err = %q", m.err)
	}
}

func TestEnrolmentQRIsDrawnOnlyWhenItFitsWithColour(t *testing.T) {
	t.Parallel()
	uri := "otpauth://totp/Acme:ada@example.test?secret=JBSWY3DPEHPK3PXP&issuer=Acme"
	code, note := EnrolmentQR(uri, true, 120, 60)
	if len(code) == 0 || note != "" || !strings.Contains(code[0], "▀") {
		t.Fatalf("a roomy colour terminal got no code: note %q", note)
	}
	if code, note := EnrolmentQR(uri, true, 30, 60); code != nil || !strings.Contains(note, "needs a terminal at least") {
		t.Errorf("a narrow terminal got a code, note %q", note)
	}
	if code, note := EnrolmentQR(uri, true, 120, 10); code != nil || !strings.Contains(note, "needs a terminal at least") {
		t.Errorf("a short terminal got a code, note %q", note)
	}
	if code, note := EnrolmentQR(uri, false, 120, 60); code != nil || !strings.Contains(note, "no colour") {
		t.Errorf("a colourless terminal got a code, note %q", note)
	}
	if code, note := EnrolmentQR(strings.Repeat("x", 400), true, 400, 400); code != nil || !strings.Contains(note, "will not fit") {
		t.Errorf("an oversized uri got a code, note %q", note)
	}
}

func TestTheSecondFactorIsRemovedAndItsCodesReplacedBehindAConfirmation(t *testing.T) {
	m, svc := accountModel(t)
	svc.totp = &core.TOTPStatus{Enrolled: true, RecoveryCodesLeft: 3}
	m = drainAccount(t, m, m.loadAccount())
	if got := adminSection(t, m.Frame(), "second factor:"); !strings.Contains(got, "on, 3 recovery codes left") {
		t.Errorf("the factor row says:\n%s", got)
	}

	m.form = AccountAddForm(m.mayPerform, svc.totp)
	if m.form.Fields[0].Options[len(m.form.Fields[0].Options)-1] != addRecovery ||
		slices.Contains(m.form.Fields[0].Options, addFactor) {
		t.Fatalf("an enrolled reader is offered %v", m.form.Fields[0].Options)
	}
	m, _ = fill(t, m, map[string]string{"subject": addRecovery})
	if !strings.Contains(m.confirm.Question(), "replace your recovery codes") {
		t.Fatalf("replacing the codes asked %q", m.confirm.Question())
	}
	m, cmd := m.reduce(pressKey("y"))
	m = drainAccount(t, m, cmd)
	if svc.regenerated != 1 || !strings.Contains(m.Frame(), "eeee-ffff") {
		t.Fatalf("the codes were not replaced and shown: %d", svc.regenerated)
	}

	m = selectRow(t, m, accountRowFactor, "")
	m, _ = m.reduce(pressKey("X"))
	if !strings.Contains(m.confirm.Question(), "remove your second factor") {
		t.Fatalf("X on the factor asked %q", m.confirm.Question())
	}
	m, cmd = m.reduce(pressKey("y"))
	_ = drainAccount(t, m, cmd)
	if svc.removed != 1 {
		t.Errorf("RemoveTOTP was called %d times", svc.removed)
	}
}

func TestAReaderWithoutTokenAdminIsOfferedNoTokenOrKey(t *testing.T) {
	t.Parallel()
	viewer := &core.Actor{ID: "v", Kind: core.ActorUser, Role: core.RoleViewer}
	may := func(method string) bool { return resolveActions(viewer)[method] }
	form := AccountAddForm(may, &core.TOTPStatus{})
	if !form.Open() {
		t.Fatal("a reader who may enrol a factor was offered nothing")
	}
	if got := form.Fields[0].Options; slices.Contains(got, addToken) || slices.Contains(got, addKey) {
		t.Errorf("a viewer is offered %v", got)
	}
	if AccountAddForm(func(string) bool { return false }, nil).Open() {
		t.Error("a reader who may do nothing was offered a form")
	}
}

func TestTheAccountKeysAreDocumentedInTheHelpOverlay(t *testing.T) {
	t.Parallel()
	help := DefaultKeyMap().ViewHelp(viewAccount, permitAll)
	var descs []string
	for _, e := range help {
		descs = append(descs, e.Desc)
	}
	for _, want := range []string{"issue a token, enrol a key or a second factor", "revoke or remove the selected row", "tick a scope"} {
		if !slices.Contains(descs, want) {
			t.Errorf("the account help does not list %q: %v", want, descs)
		}
	}
	if got := DefaultKeyMap().ViewHelp(viewAccount, permitOnly()); slices.ContainsFunc(got, func(e HelpEntry) bool {
		return e.Desc == "revoke or remove the selected row"
	}) {
		t.Error("a reader who may revoke nothing is offered the removal")
	}
}

func TestAbandoningAnEnrolmentGrantsNothingAndRereads(t *testing.T) {
	m, svc := accountModel(t)
	m.acct.enrolment, _ = svc.BeginTOTPEnrolment(context.TODO())
	m, _ = m.reduce(pressKey("j"))
	if m.acct.enrolment == nil || m.acct.sel != 0 {
		t.Fatal("a listing key reached the listing while an enrolment held the keyboard")
	}
	m, cmd := m.reduce(pressKey("esc"))
	if m.acct.enrolment != nil {
		t.Fatal("esc did not abandon the enrolment")
	}
	if m.view != viewAccount {
		t.Fatalf("abandoning the enrolment left the screen; view = %s", viewName(m.view))
	}
	if !strings.Contains(m.status, "grants nothing") {
		t.Errorf("the status %q does not say the abandoned enrolment grants nothing", m.status)
	}
	if cmd == nil {
		t.Error("abandoning the enrolment did not read the screen again")
	}
	if len(svc.codes) != 0 {
		t.Errorf("abandoning confirmed a code: %v", svc.codes)
	}
}

func TestTheAccountCursorStaysOnItsRowsAndEscClearsAShownSecretFirst(t *testing.T) {
	m, _ := accountModel(t)
	rows := len(m.accountRows())
	for _, step := range []struct {
		key  string
		want int
	}{
		{"G", rows - 1}, {"j", rows - 1}, {"k", rows - 2}, {"g", 0}, {"k", 0},
	} {
		m, _ = m.reduce(pressKey(step.key))
		if m.acct.sel != step.want {
			t.Fatalf("after %s the cursor is on row %d, want %d", step.key, m.acct.sel, step.want)
		}
	}
	m.acct.reveal = &AccountReveal{}
	m, _ = m.reduce(pressKey("esc"))
	if m.acct.reveal != nil || m.view != viewAccount {
		t.Fatalf("esc with a value shown did not just clear it; view %s", viewName(m.view))
	}
	m, _ = m.reduce(pressKey("esc"))
	if m.view == viewAccount {
		t.Error("a second esc did not leave the account screen")
	}
}

func TestRemovingRefusesARowWithNothingToRemove(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind AccountRowKind
		want string
	}{
		{"a log entry", accountRowLog, "select a token, a key, a session or the second factor"},
		{"a factor that is off", accountRowFactor, "no second factor is on"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := accountModel(t)
			at := slices.IndexFunc(m.accountRows(), func(r AccountRow) bool { return r.Kind == tc.kind })
			if at < 0 {
				t.Fatalf("no %v row on the screen", tc.kind)
			}
			m.acct.sel = at
			m, _ = m.reduce(pressKey("X"))
			if m.confirm.Open() || svc.removed != 0 {
				t.Fatal("a row with nothing to remove was offered for removal")
			}
			if !strings.Contains(m.err, tc.want) {
				t.Errorf("the refusal says %q, want %q", m.err, tc.want)
			}
		})
	}
}

func TestRotatingATokenAsksForAGraceConfirmsAndShowsTheValueOnce(t *testing.T) {
	m, svc := accountModel(t)
	m = selectRow(t, m, accountRowToken, "tok-ci")
	m, _ = m.reduce(pressKey("R"))
	if m.form.Kind != formRotateToken {
		t.Fatalf("R opened %v, want the rotate form", m.form.Kind)
	}
	if got := m.form.Value("grace"); got != "Stop the old token now" {
		t.Errorf("the rotate form proposes %q, want the old token stopped at once", got)
	}
	m, _ = fill(t, m, map[string]string{"grace": "Keep it working for 10 minutes"})
	if q := m.confirm.Question(); !strings.Contains(q, "rotate token ci-bot") || !strings.Contains(q, "10 minutes") {
		t.Fatalf("the confirmation asks %q, want it to name the token and the grace", q)
	}
	if len(svc.rotatedTokens) != 0 {
		t.Fatal("the rotation ran before agreement")
	}
	m, cmd := m.reduce(pressKey("y"))
	m = drainAccount(t, m, cmd)
	in, ok := svc.rotatedTokens["tok-ci"]
	if !ok || len(svc.rotatedTokens) != 1 || in.Grace.D() != 10*time.Minute {
		t.Fatalf("RotateToken got %v, want tok-ci with a 10 minute grace", svc.rotatedTokens)
	}
	frame := m.Frame()
	if !strings.Contains(adminSection(t, frame, "token rotated, rotated and shown once; copy it now:"), "tix_rotatedvalue") {
		t.Errorf("the rotated value is not drawn:\n%s", frame)
	}
	if strings.Contains(m.statusBar(), "tix_rotatedvalue") {
		t.Error("the rotated value went to the status bar")
	}
}

func TestATokenThatCannotBeRotatedIsNotOffered(t *testing.T) {
	future := time.Now().Add(time.Hour)
	for _, tc := range []struct {
		name, id, says string
		token          *core.APIToken
	}{
		{"revoked", "tok-old", "no longer works", nil},
		{"retiring", "tok-retiring", "already rotated",
			&core.APIToken{ID: "tok-retiring", Name: "swap", Scopes: []core.Scope{core.ScopeTaskRead}, RevokedAt: &future}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := accountModel(t)
			if tc.token != nil {
				svc.tokens = append(svc.tokens, *tc.token)
				m = drainAccount(t, m, m.loadAccount())
			}
			m = selectRow(t, m, accountRowToken, tc.id)
			m, _ = m.reduce(pressKey("R"))
			if m.form.Open() || m.confirm.Open() || len(svc.rotatedTokens) != 0 {
				t.Fatal("a token with nothing to replace was offered for rotation")
			}
			if !strings.Contains(m.err, tc.says) {
				t.Errorf("the refusal says %q, want %q", m.err, tc.says)
			}
		})
	}
}

func TestTheRotateKeyIsDocumentedAndGated(t *testing.T) {
	t.Parallel()
	has := func(help []HelpEntry) bool {
		return slices.ContainsFunc(help, func(e HelpEntry) bool { return e.Desc == "rotate the selected token" })
	}
	if !has(DefaultKeyMap().ViewHelp(viewAccount, permitAll)) {
		t.Error("the account help does not list the rotate key")
	}
	if has(DefaultKeyMap().ViewHelp(viewAccount, permitOnly())) {
		t.Error("a reader who may rotate nothing is offered the rotate key")
	}
}
