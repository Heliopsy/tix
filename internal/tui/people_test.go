// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// peopleFake is the fake service with the user and invite calls scripted and
// recorded, kept beside the people screen rather than in the shared fake.
type peopleFake struct {
	*fakeService

	users    []core.User
	invites  []core.Invite
	listed   int
	shown    []string
	created  []core.CreateUserInput
	invited  []core.CreateInviteInput
	updated  map[string]core.UpdateUserInput
	deleted  []string
	revoked  []string
	reissued map[string]core.ReissueInviteInput
	cleared  []string
	err      error
}

func newPeopleFake() *peopleFake {
	svc := adminService()
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &peopleFake{
		fakeService: svc,
		users: []core.User{
			{ID: "a-ada", Email: "ada@example.com", DisplayName: "Ada Lovelace", TOTPEnrolled: true, CreatedAt: created},
			{ID: "a-grace", Email: "grace@example.com", CreatedAt: created},
		},
		invites: []core.Invite{
			{ID: "inv-1", Email: "bob@example.com", Role: core.RoleMember, InvitedByHandle: "ada",
				ExpiresAt: time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)},
		},
		updated:  map[string]core.UpdateUserInput{},
		reissued: map[string]core.ReissueInviteInput{},
	}
}

func (f *peopleFake) ListUsers(context.Context, core.UserFilter) ([]core.User, string, error) {
	f.listed++
	return append([]core.User(nil), f.users...), "", nil
}

func (f *peopleFake) GetUser(_ context.Context, id string) (*core.User, error) {
	f.shown = append(f.shown, id)
	for _, u := range f.users {
		if u.ID == id {
			return &u, nil
		}
	}
	return nil, core.NotFound("user %q", id)
}

func (f *peopleFake) ListInvites(context.Context) ([]core.Invite, error) {
	return append([]core.Invite(nil), f.invites...), nil
}

func (f *peopleFake) CreateUser(_ context.Context, in core.CreateUserInput) (*core.User, error) {
	f.created = append(f.created, in)
	return &core.User{ID: "a-new", Email: in.Email}, f.err
}

func (f *peopleFake) CreateInvite(_ context.Context, in core.CreateInviteInput) (*core.IssuedInvite, error) {
	f.invited = append(f.invited, in)
	if f.err != nil {
		return nil, f.err
	}
	return &core.IssuedInvite{Invite: core.Invite{ID: "inv-2", Email: in.Email,
		ExpiresAt: time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)}, Token: "s3cret", Path: "/invite/s3cret"}, nil
}

func (f *peopleFake) UpdateUser(_ context.Context, id string, in core.UpdateUserInput) (*core.User, error) {
	f.updated[id] = in
	return &core.User{ID: id}, f.err
}

func (f *peopleFake) DeleteUser(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return f.err
}

func (f *peopleFake) RevokeInvite(_ context.Context, id string) error {
	f.revoked = append(f.revoked, id)
	return f.err
}

func (f *peopleFake) ReissueInvite(_ context.Context, id string, in core.ReissueInviteInput) (*core.IssuedInvite, error) {
	f.reissued[id] = in
	return &core.IssuedInvite{Invite: core.Invite{ID: id, Email: "bob@example.com"},
		Token: "fresh", URL: "https://tix.example/invite/fresh"}, f.err
}

func (f *peopleFake) ClearUserTOTP(_ context.Context, id string) error {
	f.cleared = append(f.cleared, id)
	return f.err
}

// peopleModel opens the people screen the way a reader does, with the key,
// and drains the read the key asked for.
func peopleModel(t *testing.T) (Model, *peopleFake) {
	t.Helper()
	m := boardModel(t)
	svc := newPeopleFake()
	m.svc = svc
	m, cmd := m.reduce(pressKey("@"))
	if m.view != viewPeople {
		t.Fatalf("@ did not open the people screen; view = %v", m.view)
	}
	return runCmd(t, m, cmd), svc
}

// drive presses keys in order, running every command that comes back the way
// the program would.
func pressAll(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var cmd tea.Cmd
		m, cmd = m.reduce(pressKey(k))
		m = drainPeople(m, cmd)
	}
	return m
}

// drain runs a command and everything it leads to, skipping the cursor
// blink, which waits on a timer.
func drainPeople(m Model, cmd tea.Cmd) Model {
	for cmd != nil {
		msg := cmd()
		switch msg := msg.(type) {
		case tea.BatchMsg:
			for _, c := range msg {
				m = drainPeople(m, c)
			}
			return m
		case peopleMsg, peopleShowMsg, peopleDoneMsg:
			m, cmd = m.reduce(msg)
		default:
			return m
		}
	}
	return m
}

// peopleSection returns the lines of one section of the people screen, from
// its heading to the blank line that ends it, so an address in the invites
// cannot pass for one in the users.
func peopleSection(t *testing.T, m Model, heading string) string {
	t.Helper()
	lines := strings.Split(m.Frame(), "\n")
	for i, l := range lines {
		if !strings.HasPrefix(strings.TrimSpace(l), heading) {
			continue
		}
		end := i + 1
		for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
			end++
		}
		return strings.Join(lines[i:end], "\n")
	}
	t.Fatalf("the people screen draws no %q section:\n%s", heading, m.Frame())
	return ""
}

func TestThePeopleScreenListsUsersAndInvites(t *testing.T) {
	m, _ := peopleModel(t)
	users := peopleSection(t, m, "users (2):")
	for _, want := range []string{"ada@example.com", "Ada Lovelace", "admin", "2fa", "grace@example.com", "viewer"} {
		if !strings.Contains(users, want) {
			t.Errorf("the users section does not show %q:\n%s", want, users)
		}
	}
	if strings.Contains(users, "bob@example.com") {
		t.Errorf("an invite is drawn among the users:\n%s", users)
	}
	invites := peopleSection(t, m, "pending invites (1):")
	if !strings.Contains(invites, "bob@example.com") || !strings.Contains(invites, "by @ada") {
		t.Errorf("the invites section does not show the invite:\n%s", invites)
	}
}

func TestThePeopleScreenIsOfferedOnlyToAUserAdministrator(t *testing.T) {
	member := &core.Actor{ID: "u-m", TenantID: "t", Kind: core.ActorUser, Role: core.RoleMember}
	m := New(Config{Access: capability.TUIAccess(member), Actor: member, Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 120, 40
	svc := newPeopleFake()
	m.svc = svc
	next, cmd := m.reduce(pressKey("@"))
	if next.view == viewPeople {
		t.Fatal("a member was let onto the people screen")
	}
	if cmd != nil {
		cmd()
	}
	if svc.listed != 0 {
		t.Fatal("the users were read for a reader who is not offered the screen")
	}
	for _, e := range next.keys.GlobalHelp(next.offersView()) {
		if e.Keys == "@" {
			t.Fatal("the help overlay offers @ to a member")
		}
	}
	admin := fullActor()
	if !capability.TUIAccess(admin)["people"] {
		t.Fatal("an administrator is not offered the people screen")
	}
}

func TestEnterReadsTheSelectedUserInFull(t *testing.T) {
	m, svc := peopleModel(t)
	m = pressAll(t, m, "enter")
	if len(svc.shown) != 1 || svc.shown[0] != "a-ada" {
		t.Fatalf("enter read %v, want a-ada", svc.shown)
	}
	shown := peopleSection(t, m, "user ada@example.com:")
	if !strings.Contains(shown, "second factor") || !strings.Contains(shown, "enrolled") {
		t.Errorf("the user is not stated in full:\n%s", shown)
	}
}

func TestCreatingAUserWithAPasswordRequiresAChange(t *testing.T) {
	m, svc := peopleModel(t)
	m = pressAll(t, m, "n")
	if m.form.Kind != formPeopleNew {
		t.Fatalf("n opened form %v", m.form.Kind)
	}
	m = pressAll(t, m, "tab")
	m = typeText(m, "eve@example.com")
	for range 4 {
		m = pressAll(t, m, "tab")
	}
	m = typeText(m, "correct horse battery")
	if strings.Contains(m.Frame(), "correct horse") {
		t.Fatal("the password is drawn in the clear while it is typed")
	}
	m = pressAll(t, m, "tab")
	if strings.Contains(m.Frame(), "correct horse") {
		t.Fatal("the password is drawn in the clear once the cursor has left it")
	}
	pressAll(t, m, "enter")
	if len(svc.created) != 1 {
		t.Fatalf("CreateUser was called %d times", len(svc.created))
	}
	got := svc.created[0]
	if got.Email != "eve@example.com" || got.Password != "correct horse battery" || !got.RequirePasswordChange {
		t.Fatalf("created %+v", got)
	}
	if len(svc.invited) != 0 {
		t.Fatal("a password user was also invited")
	}
}

func TestAnInviteIsIssuedAndItsLinkShownOnce(t *testing.T) {
	m, svc := peopleModel(t)
	m = pressAll(t, m, "n", "right", "tab")
	if m.form.Value("mode") != modeInvite {
		t.Fatalf("the mode is %q", m.form.Value("mode"))
	}
	m = typeText(m, "zed@example.com")
	m = pressAll(t, m, "enter")
	if len(svc.invited) != 1 || len(svc.created) != 0 {
		t.Fatalf("invited %d, created %d", len(svc.invited), len(svc.created))
	}
	if svc.invited[0].ExpiresIn.D() != 7*24*time.Hour {
		t.Errorf("the invite expires in %v, want the 7d default", svc.invited[0].ExpiresIn.D())
	}
	link := peopleSection(t, m, "invite link for zed@example.com")
	if !strings.Contains(link, "/invite/s3cret") || !strings.Contains(link, "in front of the path") {
		t.Fatalf("the link is not drawn:\n%s", link)
	}
	m = pressAll(t, m, "esc", "@")
	if strings.Contains(m.Frame(), "s3cret") {
		t.Fatal("the link was shown a second time after leaving the screen")
	}
}

func TestAnEditSendsOnlyTheFieldsThatChanged(t *testing.T) {
	m, svc := peopleModel(t)
	pressAll(t, m, "e", "tab", "right", "enter")
	in, ok := svc.updated["a-ada"]
	if !ok {
		t.Fatal("UpdateUser was not called")
	}
	if in.Role == nil || *in.Role == core.RoleAdmin {
		t.Fatalf("the role was not changed: %+v", in.Role)
	}
	if in.DisplayName != nil || in.Password != nil || in.Disabled != nil || in.RequirePasswordChange != nil {
		t.Fatalf("an untouched field was sent: %+v", in)
	}
}

func TestACrossTenantRefusalNamesThePerson(t *testing.T) {
	m, svc := peopleModel(t)
	svc.err = core.Forbidden("this person also belongs to another tenant").
		WithDetail("reason", reasonAccountInOtherTenants)
	m = pressAll(t, m, "e")
	m = pressAll(t, m, "tab", "tab", "tab", "right", "enter")
	if !strings.Contains(m.err, "ada@example.com also belongs to another tenant") ||
		!strings.Contains(m.err, "only they or a global admin") {
		t.Fatalf("the refusal reads %q", m.err)
	}
}

func TestClearingASecondFactorAsksFirst(t *testing.T) {
	m, svc := peopleModel(t)
	m = pressAll(t, m, "F")
	if !m.confirm.Open() || !strings.Contains(m.confirm.Question(), "ada@example.com") {
		t.Fatalf("no confirmation naming the user: %q", m.confirm.Question())
	}
	if len(svc.cleared) != 0 {
		t.Fatal("the second factor was cleared before the reader agreed")
	}
	m = pressAll(t, m, "y")
	if len(svc.cleared) != 1 || svc.cleared[0] != "a-ada" {
		t.Fatalf("cleared %v", svc.cleared)
	}
	m = pressAll(t, m, "down", "F")
	if m.confirm.Open() || !strings.Contains(m.err, "holds no second factor") {
		t.Fatalf("a user with no second factor was asked about one: %q", m.err)
	}
}

func TestDeletingAUserAndRevokingAnInvite(t *testing.T) {
	m, svc := peopleModel(t)
	m = pressAll(t, m, "X", "y")
	if len(svc.deleted) != 1 || svc.deleted[0] != "a-ada" {
		t.Fatalf("deleted %v", svc.deleted)
	}
	m = pressAll(t, m, "G", "X")
	if !strings.Contains(m.confirm.Question(), "revoke the invite for bob@example.com") {
		t.Fatalf("the confirmation reads %q", m.confirm.Question())
	}
	pressAll(t, m, "y")
	if len(svc.revoked) != 1 || svc.revoked[0] != "inv-1" {
		t.Fatalf("revoked %v", svc.revoked)
	}
	if len(svc.deleted) != 1 {
		t.Fatal("revoking an invite deleted a user")
	}
}

func TestReissuingAnInviteShowsTheNewLink(t *testing.T) {
	m, svc := peopleModel(t)
	m = pressAll(t, m, "G", "R", "right", "enter")
	in, ok := svc.reissued["inv-1"]
	if !ok {
		t.Fatal("ReissueInvite was not called")
	}
	if in.ExpiresIn.D() != 14*24*time.Hour {
		t.Errorf("re-issued for %v, want 14d", in.ExpiresIn.D())
	}
	link := peopleSection(t, m, "invite link for bob@example.com")
	if !strings.Contains(link, "https://tix.example/invite/fresh") {
		t.Fatalf("the new link is not drawn:\n%s", link)
	}
}

func TestPeopleKeysAreGatedOnTheOperationTheyCall(t *testing.T) {
	m, svc := peopleModel(t)
	m.allowed = ActionAccess{"ListUsers": true, "ListInvites": true, "GetUser": true}
	m = pressAll(t, m, "X", "F", "e", "n")
	if m.confirm.Open() || m.form.Open() {
		t.Fatal("a key the reader may not press opened something")
	}
	if len(svc.deleted)+len(svc.cleared)+len(svc.updated)+len(svc.created) != 0 {
		t.Fatal("a refused key reached the service")
	}
	for _, e := range m.keys.ShortHelp(viewPeople, m.actionContext()) {
		if e.Keys == "X" || e.Keys == "F" || e.Keys == "e" || e.Keys == "n" {
			t.Errorf("the footer offers %s to a reader who may not press it", e.Keys)
		}
	}
}
