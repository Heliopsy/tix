// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// adminSection returns only the lines of one section of the tenant screen, from
// its heading to the blank line that ends it.
//
// Every assertion about the screen goes through here rather than through the
// frame. The sections use the same words as each other, as the title bar and as
// the switch note below them: "default" is a tenant key, a value in the tenants
// listing and the word the session is pinned to in the title bar, and "tenant"
// is a heading, a view name and half the footer. An assertion that read the
// frame would pass on whichever of them happened to match, which is exactly how
// a guard passes while the thing it names is gone.
func adminSection(t *testing.T, frame, heading string) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == heading {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("the tenant screen draws no %q section:\n%s", heading, frame)
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			return strings.Join(lines[start:i], "\n")
		}
	}
	return strings.Join(lines[start:], "\n")
}

// adminRow returns the one row of a section that starts with a named label, so
// an assertion about the tenant's key cannot be satisfied by a domain's.
func adminRow(t *testing.T, frame, heading, label string) string {
	t.Helper()
	section := adminSection(t, frame, heading)
	var found []string
	for _, l := range strings.Split(section, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), label+" ") {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the %q section draws %d rows for %q, want exactly one:\n%s",
			heading, len(found), label, section)
	}
	return found[0]
}

// adminTenant is the tenant the screen tests render.
func adminTenant() core.Tenant {
	return core.Tenant{ID: "t1", Key: "default", Name: "Acme Works", Theme: "forest"}
}

// adminService is a service the tenant screen can be driven against.
func adminService() *fakeService {
	svc := newFakeService()
	tenant := adminTenant()
	verified := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	svc.tenant = &tenant
	svc.tenants = []core.Tenant{tenant}
	svc.domains = []core.Domain{
		{Hostname: "acme.example", CertMode: core.CertNone, VerifiedAt: &verified},
		{Hostname: "www.acme.example", CertMode: core.CertFile},
	}
	svc.members = []core.Membership{
		{ActorID: "a-ada", Role: core.RoleAdmin},
		{ActorID: "a-grace", Role: core.RoleViewer},
	}
	svc.actors = map[string]*core.Actor{
		"a-ada":   {ID: "a-ada", Handle: "ada"},
		"a-grace": {ID: "a-grace", Handle: "grace"},
	}
	svc.directory = []core.Actor{
		{ID: "a-ada", Handle: "ada"},
		{ID: "a-lovelace", Handle: "lovelace"},
	}
	return svc
}

// adminModel opens the tenant screen the way a reader does: with the key, and
// draining the read the key asked for.
func adminModel(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := adminService()
	m.svc, m.tenantKey = svc, "default"
	m.actor = &core.Actor{ID: "u-full", TenantID: "t1", Kind: core.ActorUser,
		Role: core.RoleAdmin, Handle: "ada"}
	m, cmd := m.reduce(pressKey("T"))
	m = runCmd(t, m, cmd)
	if m.view != viewTenant {
		t.Fatalf("T did not open the tenant screen; view = %v", m.view)
	}
	return m, svc
}

func TestTheTenantScreenStatesTheTenantItFetched(t *testing.T) {
	m, svc := adminModel(t)
	frame := m.Frame()

	for _, tc := range []struct{ label, want string }{
		{"key", "default"},
		{"name", "Acme Works"},
		{"theme", "forest"},
		{"actor", "@"},
	} {
		if got := adminRow(t, frame, "tenant:", tc.label); !strings.Contains(got, tc.want) {
			t.Errorf("the %s row says %q, want it to carry %q", tc.label, got, tc.want)
		}
	}
	if len(svc.tenantAsked) != 1 || svc.tenantAsked[0] != "default" {
		t.Errorf("the screen asked for %v, want the tenant this session is pinned to", svc.tenantAsked)
	}
}

func TestTheTenantScreenListsTheDomainsAndTheMembers(t *testing.T) {
	m, _ := adminModel(t)
	frame := m.Frame()

	domains := adminSection(t, frame, "domains (2):")
	for _, want := range []string{"acme.example", "verified", "www.acme.example", "cert file"} {
		if !strings.Contains(domains, want) {
			t.Errorf("the domains section does not say %q:\n%s", want, domains)
		}
	}
	if strings.Contains(domains, "@ada") {
		t.Errorf("the domains section drew a member:\n%s", domains)
	}

	members := adminSection(t, frame, "members (2):")
	for _, want := range []string{"@ada", "admin", "@grace", "viewer"} {
		if !strings.Contains(members, want) {
			t.Errorf("the members section does not say %q:\n%s", want, members)
		}
	}
	if strings.Contains(members, "acme.example") {
		t.Errorf("the members section drew a domain:\n%s", members)
	}

	tenants := adminSection(t, frame, "tenants (1):")
	if !strings.Contains(tenants, "Acme Works") {
		t.Errorf("the tenants section does not name the tenant:\n%s", tenants)
	}
}

// A member the directory cannot resolve is named by the identifier the service
// stores, because a blank row reads as a row that failed to draw.
func TestAMemberWithNoHandleIsNamedByItsIdentifier(t *testing.T) {
	t.Parallel()
	state := TenantAdminState{
		Loaded:  true,
		Members: []core.Membership{{ActorID: "a-unknown", Role: core.RoleMember}},
	}
	section := renderSection(t, TenantAdminView(state), "members (1):")
	if !strings.Contains(section, "a-unknown") {
		t.Errorf("an unresolved member rendered as %q", section)
	}
}

// renderSection is adminSection over the pure view, for the tests that render
// a state directly rather than driving a model.
func renderSection(t *testing.T, lines []TenantLine, heading string) string {
	t.Helper()
	var texts []string
	for _, l := range lines {
		texts = append(texts, l.Text)
	}
	return adminSection(t, strings.Join(texts, "\n"), heading)
}

// A refusal belongs to the read it refused. A reader who may not list domains
// keeps the tenant's own attributes.
func TestARefusedListingCostsOnlyItsOwnSection(t *testing.T) {
	m := boardModel(t)
	svc := adminService()
	svc.domainErr = core.Forbidden("domain:read is required")
	m.svc, m.tenantKey = svc, "default"
	m, cmd := m.reduce(pressKey("T"))
	m = runCmd(t, m, cmd)
	frame := m.Frame()

	domains := adminSection(t, frame, "domains (0):")
	if !strings.Contains(domains, "domain:read") {
		t.Errorf("the domains section does not say why it is empty:\n%s", domains)
	}
	if got := adminRow(t, frame, "tenant:", "name"); !strings.Contains(got, "Acme Works") {
		t.Errorf("a refused listing cost the tenant's own attributes; the name row says %q", got)
	}
	if section := adminSection(t, frame, "members (2):"); !strings.Contains(section, "@ada") {
		t.Errorf("a refused listing cost the members section:\n%s", section)
	}
}

// The cursor runs over the domains and the members as one list, so moving past
// the last domain lands on the first member rather than stopping.
func TestTheCursorRunsOverBothListings(t *testing.T) {
	m, _ := adminModel(t)
	if got := m.adminSel; got != 0 {
		t.Fatalf("the screen opened on row %d", got)
	}
	for range 2 {
		m, _ = m.reduce(pressKey("down"))
	}
	row, ok := m.selectedAdminRow()
	if !ok {
		t.Fatal("two rows down from the first domain selected nothing")
	}
	if row.Kind != tenantRowMember || row.Key != "a-ada" {
		t.Fatalf("selection = %+v, want the first member", row)
	}
	m, _ = m.reduce(pressKey("G"))
	if row, _ := m.selectedAdminRow(); row.Key != "a-grace" {
		t.Fatalf("the last row is %+v, want the last member", row)
	}
	m, _ = m.reduce(pressKey("g"))
	if row, _ := m.selectedAdminRow(); row.Key != "acme.example" {
		t.Fatalf("the first row is %+v, want the first domain", row)
	}
}

func TestRemovingADomainOrAMemberNamesItInTheQuestion(t *testing.T) {
	for _, tc := range []struct {
		name  string
		downs int
		want  string
		call  func(*fakeService) []string
	}{
		{"a domain", 0, "remove domain acme.example?",
			func(f *fakeService) []string { return f.domainsGone }},
		{"a member", 2, "remove member @ada?",
			func(f *fakeService) []string { return f.membersGone }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := adminModel(t)
			for range tc.downs {
				m, _ = m.reduce(pressKey("down"))
			}
			m, _ = m.reduce(pressKey("X"))
			line := confirmLine(t, m.Frame(), m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)
			if !strings.Contains(line, tc.want) {
				t.Errorf("the question reads %q, want it to ask %q", line, tc.want)
			}
			if got := tc.call(svc); len(got) != 0 {
				t.Fatalf("the removal ran before the reader agreed: %v", got)
			}
			m, cmd := m.reduce(pressKey("y"))
			runCmd(t, m, cmd)
			if got := tc.call(svc); len(got) != 1 {
				t.Fatalf("removals sent = %v, want exactly the one agreed to", got)
			}
		})
	}
}

// Declining is not the same as agreeing by a different key. Every key that is
// not the agreement leaves the question standing and sends nothing.
func TestDecliningRemovesNothing(t *testing.T) {
	m, svc := adminModel(t)
	m, _ = m.reduce(pressKey("X"))
	m, cmd := m.reduce(pressKey("esc"))
	if cmd != nil {
		t.Fatal("cancelling the confirmation returned a command")
	}
	if m.confirm.Open() {
		t.Fatal("esc left the confirmation standing")
	}
	if len(svc.domainsGone) != 0 {
		t.Fatalf("domains removed = %v, want none", svc.domainsGone)
	}
}

// A screen with nothing on it to remove opens no question, because a question
// over nothing is answered by reflex and removes whatever was last selected.
func TestARemovalWithNoRowsOpensNoQuestion(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	tenant := adminTenant()
	svc.tenant = &tenant
	m.svc, m.tenantKey = svc, "default"
	m, cmd := m.reduce(pressKey("T"))
	m = runCmd(t, m, cmd)

	m, _ = m.reduce(pressKey("X"))
	if m.confirm.Open() {
		t.Fatal("a screen with no domain and no member opened a confirmation")
	}
	if !strings.Contains(m.err, "no domain") {
		t.Errorf("err = %q, want it to say there is nothing to remove", m.err)
	}
}

func TestTheTenantEditGathersEachAttributeWhereItBelongs(t *testing.T) {
	m, svc := adminModel(t)
	m, _ = m.reduce(pressKey("e"))
	frame := m.Frame()
	if got := formRow(t, frame, "edit tenant default", "attribute"); !strings.Contains(got, attrName) {
		t.Errorf("the attribute row opens on %q, want the name", got)
	}

	// The theme is answered here, from the palettes the build carries.
	m, _ = m.reduce(pressKey("right"))
	themed := m.Frame()
	if got := formRow(t, themed, "edit tenant default", "theme"); !strings.Contains(got, "forest") {
		t.Errorf("the theme row says %q, want the tenant's current theme", got)
	}
	m, cmd := m.reduce(pressKey("enter"))
	runCmd(t, m, cmd)
	if len(svc.tenantsEdited) != 1 || svc.tenantsEdited[0].Theme == nil {
		t.Fatalf("edits sent = %+v, want one carrying a theme", svc.tenantsEdited)
	}
	if *svc.tenantsEdited[0].Theme != "forest" {
		t.Errorf("theme sent = %q", *svc.tenantsEdited[0].Theme)
	}
}

// The name is free text, so the form hands over to the single-line input rather
// than offering a list the service would not take.
func TestTheNameEditHandsOverToThePromptSeeded(t *testing.T) {
	m, _ := adminModel(t)
	m, _ = m.reduce(pressKey("e"))
	m, _ = m.reduce(pressKey("enter"))
	if m.prompt != promptTenantName {
		t.Fatalf("prompt = %v, want the tenant name input", m.prompt)
	}
	if got := m.input.Value(); got != "Acme Works" {
		t.Errorf("the input opened carrying %q, want the name it would replace", got)
	}
}

func TestAddingADomainTakesItsHostnameAtThePrompt(t *testing.T) {
	m, svc := adminModel(t)
	m, _ = m.reduce(pressKey("n"))
	if got := formRow(t, m.Frame(), "add to tenant", "subject"); !strings.Contains(got, addDomain) {
		t.Errorf("the add form opens on %q, want the domain", got)
	}
	m, _ = m.reduce(pressKey("enter"))
	if m.prompt != promptDomain {
		t.Fatalf("prompt = %v, want the hostname input", m.prompt)
	}
	m = typeText(m, "beta.example")
	m, cmd := m.reduce(pressKey("enter"))
	runCmd(t, m, cmd)

	if len(svc.domainsAdded) != 1 {
		t.Fatalf("domains added = %+v, want exactly one", svc.domainsAdded)
	}
	if got := svc.domainsAdded[0]; got.Hostname != "beta.example" || got.CertMode != core.CertNone {
		t.Errorf("added %+v, want the typed hostname with no certificate of its own", got)
	}
}

func TestAddingAMemberPicksFromTheDirectoryReadAtTheKeystroke(t *testing.T) {
	m, svc := adminModel(t)
	m, _ = m.reduce(pressKey("n"))
	m, _ = m.reduce(pressKey("right"))
	if got := formRow(t, m.Frame(), "add to tenant", "subject"); !strings.Contains(got, addMember) {
		t.Errorf("stepping the add form landed on %q, want the member", got)
	}
	m, cmd := m.reduce(pressKey("enter"))
	m = runCmd(t, m, cmd)

	frame := m.Frame()
	if got := formRow(t, frame, "add member", "actor"); !strings.Contains(got, "ada") {
		t.Errorf("the actor row offers %q, want somebody from the directory", got)
	}
	if got := formRow(t, frame, "add member", "role"); !strings.Contains(got, string(core.RoleMember)) {
		t.Errorf("the role row offers %q", got)
	}
	m, cmd = m.reduce(pressKey("enter"))
	runCmd(t, m, cmd)
	if len(svc.membersAdded) != 1 {
		t.Fatalf("members added = %v, want exactly one", svc.membersAdded)
	}
	if got := svc.membersAdded[0]; got[0] != "a-ada" || got[1] != string(core.RoleMember) {
		t.Errorf("added %v, want the picked actor's identifier and role", got)
	}
}

// A picker with nothing in it is an entry leading nowhere, so an empty
// directory says so instead of opening one.
func TestAnEmptyDirectoryOpensNoMemberForm(t *testing.T) {
	m, svc := adminModel(t)
	svc.directory = nil
	m, _ = m.reduce(pressKey("n"))
	m, _ = m.reduce(pressKey("right"))
	m, cmd := m.reduce(pressKey("enter"))
	m = runCmd(t, m, cmd)

	if m.form.Open() {
		t.Fatalf("a form opened over an empty directory: %+v", m.form)
	}
	if !strings.Contains(m.err, "nobody") {
		t.Errorf("err = %q, want it to say the tenant has nobody to add", m.err)
	}
}

// An action against the tenant reads the screen again, because a screen left
// stating what it used to be is a screen that lies about the action it just ran.
func TestAnActionRereadsTheTenant(t *testing.T) {
	m, svc := adminModel(t)
	before := len(svc.tenantAsked)
	m, cmd := m.reduce(actionMsg{kind: actionAddDomain, label: "beta.example"})
	if cmd == nil {
		t.Fatal("an action against the tenant asked for no re-read")
	}
	runCmd(t, m, cmd)
	if len(svc.tenantAsked) != before+1 {
		t.Errorf("the tenant was read %d times, want one more than %d", len(svc.tenantAsked), before)
	}
}

// The three actions are gated at the keystroke as well as in the footer.
// Gating only the footer refuses the affordance and lets the call go out.
func TestAReaderWithoutTenantAdminIsOfferedNoneOfTheActions(t *testing.T) {
	m, svc := adminModel(t)
	m.allowed = ActionAccess{}

	for _, press := range []string{"e", "n", "X"} {
		next, cmd := m.reduce(pressKey(press))
		if next.form.Open() || next.confirm.Open() || next.prompt != promptNone {
			t.Errorf("%q opened a control for a reader who may not administer the tenant", press)
		}
		if cmd != nil {
			t.Errorf("%q returned a command for a reader who may not administer the tenant", press)
		}
	}
	if len(svc.tenantsEdited)+len(svc.domainsAdded)+len(svc.membersGone) != 0 {
		t.Error("a refused keystroke still reached the service")
	}

	footer := m.keys.ShortHelp(viewTenant, ActionContext{May: m.permits(), HasRows: true})
	for _, e := range footer {
		for _, refused := range []string{"edit tenant", "add domain or member", "remove the selected row"} {
			if e.Desc == refused {
				t.Errorf("the footer offers %q to a reader who may not perform it", refused)
			}
		}
	}
}

// The footer is a promise. A screen with nothing to remove does not advertise
// the key that removes.
func TestTheFooterDropsTheRemovalWhenThereIsNothingToRemove(t *testing.T) {
	t.Parallel()
	keys := DefaultKeyMap()
	full := keys.ShortHelp(viewTenant, ActionContext{May: permitAll, HasRows: true})
	empty := keys.ShortHelp(viewTenant, ActionContext{May: permitAll, HasRows: false})
	if !hasDesc(full, "remove the selected row") {
		t.Fatal("the footer never offers the removal, so this test proves nothing")
	}
	if hasDesc(empty, "remove the selected row") {
		t.Error("the footer offers the removal on a screen with no rows")
	}
}

// hasDesc reports whether a footer carries an entry with this description.
func hasDesc(entries []HelpEntry, desc string) bool {
	for _, e := range entries {
		if e.Desc == desc {
			return true
		}
	}
	return false
}

func TestThemeOptionsCarryTheWordForHavingNone(t *testing.T) {
	t.Parallel()
	options := ThemeOptions()
	if len(options) == 0 || options[0] != noTheme {
		t.Fatalf("ThemeOptions = %v, want the word for none first", options)
	}
	if ThemeValue(noTheme) != "" {
		t.Errorf("ThemeValue(%q) = %q, want the empty string the service takes", noTheme, ThemeValue(noTheme))
	}
	if got := ThemeValue("forest"); got != "forest" {
		t.Errorf("ThemeValue(forest) = %q", got)
	}
	for _, name := range core.ThemeNames() {
		if !containsString(options, name) {
			t.Errorf("ThemeOptions leaves out %q, which the build carries", name)
		}
	}
}

// containsString reports whether a slice holds a value.
func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func TestTheAddFormOffersOnlyWhatTheReaderMayAdd(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		domain, member, open bool
		want                 string
	}{
		{"both", true, true, true, addDomain},
		{"members only", false, true, true, addMember},
		{"neither", false, false, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			form := TenantAddForm(tc.domain, tc.member)
			if form.Open() != tc.open {
				t.Fatalf("TenantAddForm(%v, %v).Open() = %v", tc.domain, tc.member, form.Open())
			}
			if !tc.open {
				return
			}
			if got := form.Value("subject"); got != tc.want {
				t.Errorf("the form opens on %q, want %q", got, tc.want)
			}
		})
	}
}
