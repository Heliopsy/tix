// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// globalActor is a tenant admin of t1 the service reports as a global admin.
func globalActor() *core.Actor {
	return &core.Actor{ID: "u-root", TenantID: "t1", Kind: core.ActorUser, Role: core.RoleAdmin,
		Handle: "root", GlobalAdmin: true}
}

// globalFake holds two tenants, one global admin, users in acme and a log.
func globalFake() *adminFake {
	svc := newAdminFake()
	svc.allTenants = []core.Tenant{{ID: "t1", Key: "default", Name: "Default"},
		{ID: "t2", Key: "acme", Name: "Acme Works"}}
	svc.admins = []core.User{{ID: "u-root", Email: "root@example.com", DisplayName: "Root"}}
	svc.users["acme"] = []core.TenantUser{
		{User: core.User{ID: "u-ada", Email: "ada@acme.example", DisplayName: "Ada"}, Role: core.RoleMember},
	}
	svc.logs["root@example.com"] = []core.AccountLogEntry{{Action: "password_changed", TenantKey: "default",
		ByEmail: "root@example.com", Via: core.ViaKind("web"),
		OccurredAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}
	return svc
}

// globalModel opens the global admin view as a global admin does.
func globalModel(t *testing.T, svc *adminFake) Model {
	t.Helper()
	m := boardModel(t)
	actor := globalActor()
	m.svc, m.actor, m.access, m.allowed = svc, actor, capability.TUIAccess(actor), resolveActions(actor)
	m, cmd := m.reduce(pressKey("!"))
	m = runCmd(t, m, cmd)
	if m.view != viewGlobal {
		t.Fatalf("! did not open the global admin view; view = %v", m.view)
	}
	return m
}

// drive folds one key and every message its command produces back in.
func drive(t *testing.T, m Model, k tea.KeyPressMsg) Model {
	t.Helper()
	m, cmd := m.reduce(k)
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		if _, ok := msg.(tea.BatchMsg); ok {
			break
		}
		m, cmd = m.reduce(msg)
	}
	return m
}

func TestOnlyAReportedGlobalAdminIsOfferedTheView(t *testing.T) {
	admin := &core.Actor{ID: "a", TenantID: "t1", Role: core.RoleAdmin}
	svc := globalFake()
	m := New(Config{Actor: admin, Access: capability.TUIAccess(admin), Service: svc})
	if m.canReach(viewGlobal) {
		t.Fatal("a tenant admin who is not a global admin is offered the global admin view")
	}
	m, cmd := m.reduce(pressKey("!"))
	if m.view == viewGlobal || cmd != nil || svc.adminsN != 0 {
		t.Fatal("! entered the view or read the global admins for a reader not offered it")
	}
	for _, e := range m.keys.GlobalHelp(m.offersView()) {
		if e.Desc == "global admin" {
			t.Error("the help overlay names the global admin view to a reader not offered it")
		}
	}

	reported := globalActor()
	if !New(Config{Actor: reported, Access: capability.TUIAccess(reported)}).canReach(viewGlobal) {
		t.Fatal("a reported global admin is not offered the view")
	}
}

func TestWithStandingCarriesTheServicesReport(t *testing.T) {
	svc := newFakeService()
	svc.whoAmI = &core.Actor{ID: "a", GlobalAdmin: true}
	actor := &core.Actor{ID: "a", TenantID: "t1"}
	got := WithStanding(t.Context(), svc, actor)
	if !got.GlobalAdmin || actor.GlobalAdmin {
		t.Fatalf("standing = %v, caller's actor changed = %v", got.GlobalAdmin, actor.GlobalAdmin)
	}
	svc.whoAmI, svc.whoAmIErr = nil, core.Unauthenticated("no")
	if WithStanding(t.Context(), svc, actor).GlobalAdmin {
		t.Fatal("a failed question offered global admin")
	}
}

func TestTheGlobalAdminViewListsTenantsAndAdmins(t *testing.T) {
	frame := globalModel(t, globalFake()).Frame()
	tenants := adminSection(t, frame, "tenants (2):")
	for _, want := range []string{"default", "(this session)", "acme", "Acme Works"} {
		if !strings.Contains(tenants, want) {
			t.Errorf("the tenants section does not say %q:\n%s", want, tenants)
		}
	}
	if strings.Contains(adminRow(t, frame, "tenants (2):", "acme"), "(this session)") {
		t.Error("another tenant is marked as this session's")
	}
	if got := adminSection(t, frame, "global admins (1):"); !strings.Contains(got, "root@example.com") {
		t.Errorf("the admins section does not name the admin:\n%s", got)
	}
}

func TestTheSessionsOwnTenantIsNotDeleted(t *testing.T) {
	svc := globalFake()
	m := globalModel(t, svc)
	m, _ = m.reduce(pressKey("X"))
	if m.confirm.Open() {
		t.Fatal("a deletion of the session's own tenant was offered")
	}
	if !strings.Contains(m.err, "this session is on default") {
		t.Errorf("err = %q", m.err)
	}

	m = drive(t, m, pressKey("down"))
	m, _ = m.reduce(pressKey("X"))
	if q := m.confirm.Question(); !strings.Contains(q, "delete tenant acme") {
		t.Fatalf("question = %q", q)
	}
	m = drive(t, m, pressKey("y"))
	if len(svc.deletedTenants) != 1 || svc.deletedTenants[0] != "acme" {
		t.Fatalf("deleted %v, want acme", svc.deletedTenants)
	}
	if !strings.Contains(m.status, "deleted tenant acme") {
		t.Errorf("status = %q", m.status)
	}
}

func TestCreatingATenantShowsTheGeneratedPasswordsOnce(t *testing.T) {
	svc := globalFake()
	svc.credentials = []core.InitialCredential{{Email: "boss@new.example", Handle: "boss", Password: "s3cret-once-only"}}
	m := globalModel(t, svc)

	m, _ = m.reduce(pressKey("n"))
	if m.form.Kind != formGlobalAdd {
		t.Fatalf("n opened form %v", m.form.Kind)
	}
	m.form = m.form.SetValue("key", "newco").SetValue("name", "New Co").
		SetValue("admins", "boss@new.example, ops@new.example")
	m = drive(t, m, pressKey("enter"))

	if len(svc.createdTenants) != 1 {
		t.Fatalf("created %v", svc.createdTenants)
	}
	in := svc.createdTenants[0]
	if in.Key != "newco" || in.Name != "New Co" || len(in.Admins) != 2 || in.Admins[1].Email != "ops@new.example" {
		t.Errorf("CreateTenant got %+v", in)
	}
	frame := m.Frame()
	if got := adminSection(t, frame, "generated passwords (shown once):"); !strings.Contains(got, "s3cret-once-only") {
		t.Errorf("the generated password is not shown:\n%s", got)
	}
	m = drive(t, m, pressKey("r"))
	if strings.Contains(m.Frame(), "s3cret-once-only") {
		t.Error("the generated password survived reading the view again")
	}
}

func TestGrantingGlobalAdminTakesAnEmail(t *testing.T) {
	svc := globalFake()
	m := globalModel(t, svc)
	m, _ = m.reduce(pressKey("n"))
	m.form = m.form.SetValue("subject", addGlobalAdmin).SetValue("email", "new@example.com")
	drive(t, m, pressKey("enter"))
	if len(svc.granted) != 1 || svc.granted[0] != "new@example.com" || len(svc.createdTenants) != 0 {
		t.Fatalf("granted %v, created %v", svc.granted, svc.createdTenants)
	}
}

func TestRevokingGlobalAdminNamesTheAddress(t *testing.T) {
	svc := globalFake()
	m := globalModel(t, svc)
	m = drive(t, m, pressKey("G"))
	m, _ = m.reduce(pressKey("X"))
	if q := m.confirm.Question(); q != "revoke global admin from root@example.com?" {
		t.Fatalf("question = %q", q)
	}
	drive(t, m, pressKey("y"))
	if len(svc.revoked) != 1 || svc.revoked[0] != "root@example.com" {
		t.Fatalf("revoked %v", svc.revoked)
	}
}

// usersOf opens acme's users.
func usersOf(t *testing.T, svc *adminFake) Model {
	t.Helper()
	m := globalModel(t, svc)
	m = drive(t, m, pressKey("down"))
	m = drive(t, m, pressKey("enter"))
	if m.global.mode != globalUsers || m.global.tenant != "acme" {
		t.Fatalf("enter on acme opened mode %v of %q", m.global.mode, m.global.tenant)
	}
	return m
}

func TestATenantsUsersOpenAndBackWalksOut(t *testing.T) {
	m := usersOf(t, globalFake())
	row := adminRow(t, m.Frame(), "users of acme (1):", "▸ ada@acme.example")
	if !strings.Contains(row, "member") {
		t.Errorf("the user row says %q", row)
	}
	m = drive(t, m, pressKey("esc"))
	if m.view != viewGlobal || m.global.mode != globalHome {
		t.Fatalf("esc from users went to view %v mode %v, want the tenant listing", m.view, m.global.mode)
	}
	m = drive(t, m, pressKey("esc"))
	if m.view == viewGlobal {
		t.Fatal("esc on the tenant listing did not leave the view")
	}
}

func TestAnExistingAccountIsAddedToTheOpenTenant(t *testing.T) {
	svc := globalFake()
	m := usersOf(t, svc)
	m, _ = m.reduce(pressKey("n"))
	m.form = m.form.SetValue("subject", addAccount).SetValue("email", "grace@example.com").
		SetValue("handle", "grace").SetValue("role", string(core.RoleAdmin))
	drive(t, m, pressKey("enter"))
	if len(svc.addedMembers) != 1 {
		t.Fatalf("added %v", svc.addedMembers)
	}
	got := svc.addedMembers[0]
	if got.tenant != "acme" || got.in.Email != "grace@example.com" || got.in.Handle != "grace" ||
		got.in.Role != core.RoleAdmin || len(svc.createdUsers) != 0 {
		t.Errorf("AddTenantMember got %+v", got)
	}
}

func TestANewUsersPasswordIsNeverDrawn(t *testing.T) {
	svc := globalFake()
	m := usersOf(t, svc)
	m, _ = m.reduce(pressKey("n"))
	tab := tea.KeyPressMsg{Code: tea.KeyTab}
	for range 4 {
		m = drive(t, m, tab)
	}
	if f, _ := m.form.Selected(); f.Key != "password" {
		t.Fatalf("four tabs landed on %q", f.Key)
	}
	const secret = "correct-horse-battery"
	for _, r := range secret {
		m, _ = m.reduce(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if strings.Contains(m.Frame(), secret) {
		t.Fatal("the password is drawn while it is typed")
	}
	m = drive(t, m, tab)
	if strings.Contains(m.Frame(), secret) {
		t.Fatal("the password is drawn once the cursor leaves it")
	}
	m.form = m.form.SetValue("email", "new@acme.example")
	drive(t, m, pressKey("enter"))
	if len(svc.createdUsers) != 1 || svc.createdUsers[0].in.Password != secret {
		t.Fatalf("created %+v", svc.createdUsers)
	}
}

func TestEditingAUserSendsOnlyWhatChanged(t *testing.T) {
	svc := globalFake()
	m := usersOf(t, svc)
	m, _ = m.reduce(pressKey("e"))
	if m.form.Kind != formGlobalUserEdit {
		t.Fatalf("e opened form %v", m.form.Kind)
	}
	m.form = m.form.SetValue("role", string(core.RoleAdmin))
	drive(t, m, pressKey("enter"))
	if len(svc.updatedUsers) != 1 {
		t.Fatalf("updated %v", svc.updatedUsers)
	}
	got := svc.updatedUsers[0]
	if got.tenant != "acme" || got.id != "u-ada" || got.in.Role == nil || *got.in.Role != core.RoleAdmin ||
		got.in.DisplayName != nil || got.in.Disabled != nil {
		t.Errorf("UpdateTenantUser got %+v", got)
	}
}

func TestDeletingAUserIsConfirmed(t *testing.T) {
	svc := globalFake()
	m := usersOf(t, svc)
	m, _ = m.reduce(pressKey("X"))
	if q := m.confirm.Question(); q != "delete user ada@acme.example from acme?" {
		t.Fatalf("question = %q", q)
	}
	drive(t, m, pressKey("y"))
	if len(svc.deletedUsers) != 1 || svc.deletedUsers[0] != [2]string{"acme", "u-ada"} {
		t.Fatalf("deleted %v", svc.deletedUsers)
	}
}

func TestAnAccountLogIsReadByEmail(t *testing.T) {
	svc := globalFake()
	m := globalModel(t, svc)
	m, _ = m.reduce(pressKey("/"))
	if m.form.Kind != formGlobalLog {
		t.Fatalf("/ opened form %v", m.form.Kind)
	}
	m.form = m.form.SetValue("email", "root@example.com")
	m.form.Sel = -1
	m = drive(t, m, pressKey("enter"))
	if m.global.mode != globalLog {
		t.Fatalf("mode = %v, want the log", m.global.mode)
	}
	got := adminSection(t, m.Frame(), "account log of root@example.com (1):")
	for _, want := range []string{"password_changed", "in default", "by root@example.com", "via web"} {
		if !strings.Contains(got, want) {
			t.Errorf("the log does not say %q:\n%s", want, got)
		}
	}
	m = drive(t, m, pressKey("esc"))
	if m.view != viewGlobal || m.global.mode != globalHome {
		t.Fatalf("esc from the log went to view %v mode %v", m.view, m.global.mode)
	}

	m, _ = m.reduce(pressKey("/"))
	m.form = m.form.SetValue("email", "nobody@example.com")
	m.form.Sel = -1
	m = drive(t, m, pressKey("enter"))
	if m.global.mode != globalHome || !strings.Contains(m.err, "nobody@example.com") {
		t.Fatalf("an unknown address left mode %v and err %q", m.global.mode, m.err)
	}
}

func TestTheGlobalAdminFooterFollowsTheOpenPart(t *testing.T) {
	descs := func(m Model) []string {
		var out []string
		for _, e := range m.keys.ShortHelp(viewGlobal, m.actionContext()) {
			out = append(out, e.Desc)
		}
		return out
	}
	has := func(list []string, s string) bool {
		for _, x := range list {
			if x == s {
				return true
			}
		}
		return false
	}
	m := globalModel(t, globalFake())
	home := descs(m)
	if !has(home, "create tenant or grant global admin") || has(home, "edit the selected user") {
		t.Errorf("the tenant listing's footer is %v", home)
	}
	users := descs(usersOf(t, globalFake()))
	if !has(users, "edit the selected user") || has(users, "create tenant or grant global admin") {
		t.Errorf("the users footer is %v", users)
	}
}
