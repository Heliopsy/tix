// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
)

// peopleMsg carries one read of the people screen. Each listing carries its
// own refusal, so a reader refused the roles still gets the users.
type peopleMsg struct {
	users      []core.User
	more       bool
	roles      map[string]core.Role
	rolesErr   string
	invites    []core.Invite
	usersErr   string
	invitesErr string
}

// peopleShowMsg carries one user read in full.
type peopleShowMsg struct {
	user *core.User
	err  error
}

// peopleDoneMsg reports one change the people screen made. issued is the link
// a create or a re-issue minted, and retry the form a refused create came
// from, reopened so the reader does not type it again.
type peopleDoneMsg struct {
	verb     string
	subject  string
	sentence string
	issued   *core.IssuedInvite
	retry    Form
	err      error
}

// peopleBindings are the actions the people screen offers, each against the
// operation it calls. One list, so the footer, the help overlay and the
// keystroke cannot disagree about who may press a key.
//
// The keys are the board's own, so every scheme drives the screen without
// rebinding anything. R renews, here a link; F forces, here the second factor
// off.
func (k KeyMap) peopleBindings() []viewAction {
	return []viewAction{
		{gatedAction{binding: k.Enter, methods: []string{"GetUser"}}, "show user"},
		{gatedAction{binding: k.New, methods: []string{"CreateUser", "CreateInvite"}}, "new user or invite"},
		{gatedAction{binding: k.Edit, methods: []string{"UpdateUser"}}, "edit user"},
		{gatedAction{binding: k.Renew, methods: []string{"ReissueInvite"}}, "re-issue invite"},
		{gatedAction{binding: k.Reclaim, methods: []string{"ClearUserTOTP"}}, "clear second factor"},
		{gatedAction{binding: k.Delete, methods: []string{"DeleteUser", "RevokeInvite"}},
			"delete user or revoke invite"},
	}
}

// peopleActions are the people screen's bindings, as far as this reader's
// authority reaches.
func (k KeyMap) peopleActions(may func(string) bool) []HelpEntry {
	out := make([]HelpEntry, 0, len(k.peopleBindings()))
	for _, a := range k.peopleBindings() {
		if a.permitted(may) {
			out = append(out, a.entry())
		}
	}
	return out
}

// peopleActionNames are the actions the people screen listens for, which is
// the set a rebinding collision is looked for within. Left and Right cycle a
// form's answers.
func peopleActionNames() []string {
	return []string{
		"Up", "Down", "Left", "Right", "Top", "Bottom", "Enter",
		"New", "Edit", "Renew", "Reclaim", "Delete", "Back",
	}
}

// openPeople shows the people screen and reads what it lists. A link issued
// on an earlier visit is not shown again: it was shown once.
func (m Model) openPeople() (Model, tea.Cmd) {
	next := m.enterView(viewPeople)
	if next.view != viewPeople {
		return next, nil
	}
	next.people.sel, next.people.off = 0, 0
	next.people.issued, next.people.shown = nil, nil
	return next, next.loadPeople()
}

// loadPeople reads the users, their roles and the pending invites in one
// command. The roles are read only when this reader may read memberships,
// since the screen is offered on user:admin and the roles need tenant:admin.
func (m Model) loadPeople() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, mayRoles := m.svc, m.ctx, m.mayPerform("ListMembers")
	return func() tea.Msg {
		out := peopleMsg{}
		if users, next, err := svc.ListUsers(ctx, core.UserFilter{Page: core.Page{Limit: peopleLimit}}); err != nil {
			out.usersErr = err.Error()
		} else {
			out.users, out.more = users, next != ""
		}
		if invites, err := svc.ListInvites(ctx); err != nil {
			out.invitesErr = err.Error()
		} else {
			out.invites = invites
		}
		if !mayRoles {
			out.rolesErr = "reading roles needs tenant:admin"
			return out
		}
		members, err := svc.ListMembers(ctx)
		if err != nil {
			out.rolesErr = err.Error()
			return out
		}
		out.roles = make(map[string]core.Role, len(members))
		for _, mb := range members {
			out.roles[mb.ActorID] = mb.Role
		}
		return out
	}
}

// onPeople adopts a read, bounding the cursor to the rows that came back.
func (m Model) onPeople(msg peopleMsg) (Model, tea.Cmd) {
	p := m.people
	p.users, p.more, p.invites = msg.users, msg.more, msg.invites
	p.roles, p.rolesErr = msg.roles, msg.rolesErr
	p.usersErr, p.invitesErr, p.loaded = msg.usersErr, msg.invitesErr, true
	p.sel = clamp(p.sel, 0, len(PeopleRows(p))-1)
	if p.shown != nil && !hasUser(p.users, p.shown.ID) {
		p.shown = nil
	}
	m.people = p
	return m, nil
}

// hasUser reports whether a listing still holds a user.
func hasUser(users []core.User, id string) bool {
	for _, u := range users {
		if u.ID == id {
			return true
		}
	}
	return false
}

// selectedPerson is the row the screen's keys act on.
func (m Model) selectedPerson() (PeopleRow, bool) {
	rows := PeopleRows(m.people)
	if m.people.sel < 0 || m.people.sel >= len(rows) {
		return PeopleRow{}, false
	}
	return rows[m.people.sel], true
}

// selectedUser is the user under the cursor, if the cursor is on one.
func (m Model) selectedUser() (core.User, bool) {
	row, ok := m.selectedPerson()
	if !ok || row.Kind != peopleRowUser {
		return core.User{}, false
	}
	for _, u := range m.people.users {
		if u.ID == row.ID {
			return u, true
		}
	}
	return core.User{}, false
}

// selectedInvite is the invite under the cursor, if the cursor is on one.
func (m Model) selectedInvite() (core.Invite, bool) {
	row, ok := m.selectedPerson()
	if !ok || row.Kind != peopleRowInvite {
		return core.Invite{}, false
	}
	for _, inv := range m.people.invites {
		if inv.ID == row.ID {
			return inv, true
		}
	}
	return core.Invite{}, false
}

// handlePeopleKey moves the cursor and runs the screen's actions, refusing a
// key this reader's authority does not reach rather than letting the service
// refuse it.
func (m Model) handlePeopleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	rows := len(PeopleRows(m.people))
	switch {
	case key.Matches(msg, m.keys.Back):
		m.people.issued, m.people.shown = nil, nil
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		m.people.sel = clamp(m.people.sel-1, 0, rows-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.people.sel = clamp(m.people.sel+1, 0, rows-1)
		return m, nil
	case key.Matches(msg, m.keys.Top):
		m.people.sel, m.people.off = 0, 0
		return m, nil
	case key.Matches(msg, m.keys.Bottom):
		m.people.sel = clamp(rows-1, 0, rows-1)
		return m, nil
	}
	if !m.mayPeoplePress(msg) {
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Enter):
		return m.showPerson()
	case key.Matches(msg, m.keys.New):
		return m.openPeopleCreate()
	case key.Matches(msg, m.keys.Edit):
		return m.openPeopleEdit()
	case key.Matches(msg, m.keys.Renew):
		return m.openReissue()
	case key.Matches(msg, m.keys.Reclaim):
		return m.confirmClearTOTP()
	case key.Matches(msg, m.keys.Delete):
		return m.confirmPeopleRemoval()
	}
	return m, nil
}

// mayPeoplePress reports whether this reader holds the authority the pressed
// key acts through, asked of the list the footer and the help overlay filter
// with.
func (m Model) mayPeoplePress(msg tea.KeyPressMsg) bool {
	for _, a := range m.keys.peopleBindings() {
		if key.Matches(msg, a.binding) {
			return a.permitted(m.permits())
		}
	}
	return true
}

// showPerson reads the selected user in full.
func (m Model) showPerson() (Model, tea.Cmd) {
	u, ok := m.selectedUser()
	if !ok {
		m.err = "select a user to read; an invite row already says everything it holds"
		return m, nil
	}
	if m.svc == nil {
		return m, nil
	}
	svc, ctx, id := m.svc, m.ctx, u.ID
	return m, func() tea.Msg {
		user, err := svc.GetUser(ctx, id)
		return peopleShowMsg{user: user, err: err}
	}
}

// onPeopleShow draws the user read in full under the listings.
func (m Model) onPeopleShow(msg peopleShowMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "reading the user: " + msg.err.Error()
		return m, nil
	}
	m.people.shown, m.err = msg.user, ""
	return m, nil
}

// openPeopleCreate opens the form for a new user or an invite.
func (m Model) openPeopleCreate() (Model, tea.Cmd) {
	form := PeopleCreateForm(m.mayPerform("CreateUser"), m.mayPerform("CreateInvite"))
	if !form.Open() {
		m.err = "this session may neither create a user nor invite one"
		return m, nil
	}
	m.people.issued = nil
	return m.openForm(form)
}

// openPeopleEdit opens the edit form on the selected user.
func (m Model) openPeopleEdit() (Model, tea.Cmd) {
	u, ok := m.selectedUser()
	if !ok {
		m.err = "select a user to edit"
		return m, nil
	}
	m.people.issued = nil
	return m.openForm(PeopleEditForm(u, m.people.roles[u.ID]))
}

// openReissue asks how long the selected invite's new link works.
func (m Model) openReissue() (Model, tea.Cmd) {
	inv, ok := m.selectedInvite()
	if !ok {
		m.err = "select an invite to re-issue"
		return m, nil
	}
	m.people.issued = nil
	m.form, m.err = ReissueForm(inv), ""
	return m, nil
}

// confirmClearTOTP names the user whose second factor goes and waits for
// agreement. A user holding none is told so rather than asked about nothing.
func (m Model) confirmClearTOTP() (Model, tea.Cmd) {
	u, ok := m.selectedUser()
	if !ok {
		m.err = "select a user to clear the second factor of"
		return m, nil
	}
	if !u.TOTPEnrolled {
		m.err = u.Email + " holds no second factor to clear"
		return m, nil
	}
	m.people.issued = nil
	return m.askConfirm(Confirm{Kind: confirmClearTOTP, Target: u.Email,
		Note: "dropping the enrolment and its recovery codes", userID: u.ID}, "there is nobody named to clear")
}

// confirmPeopleRemoval names the selected user or invite and waits for
// agreement.
func (m Model) confirmPeopleRemoval() (Model, tea.Cmd) {
	row, ok := m.selectedPerson()
	if !ok {
		m.err = "this tenant lists no user and no invite to remove"
		return m, nil
	}
	m.people.issued = nil
	if row.Kind == peopleRowInvite {
		return m.askConfirm(Confirm{Kind: confirmRevokeInvite, Target: row.Label,
			Note: "keeping the account without a password", inviteID: row.ID}, "there is no invite named to revoke")
	}
	return m.askConfirm(Confirm{Kind: confirmDeleteUser, Target: row.Label,
		Note: "which cannot be undone; disabling can", userID: row.ID}, "there is nobody named to delete")
}

// applyPeopleCreate creates the user or issues the invite the form gathered.
func (m Model) applyPeopleCreate(form Form) (Model, tea.Cmd) {
	if m.svc == nil {
		return m, nil
	}
	svc, ctx := m.svc, m.ctx
	if form.Value("mode") == modeInvite {
		in, err := CreateInviteFrom(form)
		if err != nil {
			m.err = "cannot invite: " + err.Error()
			return m, nil
		}
		return m, func() tea.Msg {
			issued, err := svc.CreateInvite(ctx, in)
			return peopleDoneMsg{verb: "invite " + in.Email, subject: in.Email,
				sentence: "invited " + in.Email, issued: issued, retry: form, err: err}
		}
	}
	in := CreateUserFrom(form)
	return m, func() tea.Msg {
		_, err := svc.CreateUser(ctx, in)
		return peopleDoneMsg{verb: "create " + in.Email, subject: in.Email,
			sentence: "created " + in.Email, retry: form, err: err}
	}
}

// applyPeopleEdit sends the fields the edit form changed.
func (m Model) applyPeopleEdit(form Form) (Model, tea.Cmd) {
	u, ok := m.selectedUser()
	if !ok || m.svc == nil {
		return m, nil
	}
	in, changed := UpdateUserFrom(u, m.people.roles[u.ID], form)
	if !changed {
		m.status = "nothing changed for " + u.Email
		return m, nil
	}
	svc, ctx := m.svc, m.ctx
	return m, func() tea.Msg {
		_, err := svc.UpdateUser(ctx, u.ID, in)
		return peopleDoneMsg{verb: "edit " + u.Email, subject: u.Email, sentence: "updated " + u.Email, err: err}
	}
}

// applyReissue replaces the selected invite's link.
func (m Model) applyReissue(form Form) (Model, tea.Cmd) {
	inv, ok := m.selectedInvite()
	if !ok || m.svc == nil {
		return m, nil
	}
	ttl, err := core.ParseDuration(form.Value("expires"))
	if err != nil {
		m.err = "cannot re-issue: " + err.Error()
		return m, nil
	}
	svc, ctx := m.svc, m.ctx
	return m, func() tea.Msg {
		issued, err := svc.ReissueInvite(ctx, inv.ID, core.ReissueInviteInput{ExpiresIn: ttl})
		return peopleDoneMsg{verb: "re-issue the invite for " + inv.Email, subject: inv.Email,
			sentence: "re-issued the invite for " + inv.Email, issued: issued, err: err}
	}
}

// deleteUser removes the user the reader agreed to remove.
func (m Model) deleteUser(c Confirm) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		return peopleDoneMsg{verb: "delete " + c.Target, subject: c.Target,
			sentence: "deleted " + c.Target, err: svc.DeleteUser(ctx, c.userID)}
	}
}

// revokeInvite stops the link the reader agreed to stop.
func (m Model) revokeInvite(c Confirm) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		return peopleDoneMsg{verb: "revoke the invite for " + c.Target, subject: c.Target,
			sentence: "revoked the invite for " + c.Target, err: svc.RevokeInvite(ctx, c.inviteID)}
	}
}

// clearTOTP drops the second factor the reader agreed to drop.
func (m Model) clearTOTP(c Confirm) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx := m.svc, m.ctx
	return func() tea.Msg {
		return peopleDoneMsg{verb: "clear the second factor of " + c.Target, subject: c.Target,
			sentence: "cleared the second factor of " + c.Target, err: svc.ClearUserTOTP(ctx, c.userID)}
	}
}

// onPeopleDone reports a change and reads the screen again. A refused create
// reopens its form with the refusal beside it, so nothing typed is lost.
func (m Model) onPeopleDone(msg peopleDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		failure := PeopleFailure(msg.verb, msg.subject, msg.err)
		if msg.retry.Open() && !m.form.Open() && !m.confirm.Open() {
			retry := msg.retry
			retry.Note = failure
			next, cmd := m.openForm(retry)
			next.err = failure
			return next, cmd
		}
		m.err = failure
		return m, nil
	}
	m.err, m.status = "", msg.sentence
	m.people.issued = msg.issued
	if msg.issued != nil {
		m.status += "; the link is on screen once"
	}
	return m, m.loadPeople()
}

// peopleLines renders the people screen, scrolled so the selected row stays
// on screen.
func (m Model) peopleLines(layout Layout) []string {
	raw := PeopleView(m.people, m.timeStyle, m.now())
	head := []string{m.theme.Header.Render("  people"), ""}
	rows := VisibleRows(layout.BodyHeight-len(head), len(raw))
	at := 0
	for i, l := range raw {
		if l.Selected {
			at = i
			break
		}
	}
	offset := ScrollWindow(m.people.off, at, rows, len(raw))
	out := make([]string, 0, rows+len(head)+1)
	out = append(out, head...)
	for i := offset; i < len(raw) && i < offset+rows; i++ {
		out = append(out, m.fit(m.tenantStyle(raw[i])))
	}
	if hint := ScrollHint(offset, rows, len(raw)); hint != "" {
		out = append(out, m.theme.Dim.Render("  "+hint))
	}
	return out
}
