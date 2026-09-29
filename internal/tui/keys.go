// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import "charm.land/bubbles/v2/key"

// KeyMap holds every binding the interface offers.
type KeyMap struct {
	Up            key.Binding
	Down          key.Binding
	Left          key.Binding
	Right         key.Binding
	Top           key.Binding
	Bottom        key.Binding
	Enter         key.Binding
	Back          key.Binding
	Filter        key.Binding
	ClearFltr     key.Binding
	Claim         key.Binding
	Release       key.Binding
	Transition    key.Binding
	New           key.Binding
	NewProject    key.Binding
	Edit          key.Binding
	Priority      key.Binding
	CyclePriority key.Binding
	Assign        key.Binding
	Comment       key.Binding
	Tag           key.Binding
	Untag         key.Binding
	Tags          key.Binding
	Depend        key.Binding
	Undepend      key.Binding
	CommentEdit   key.Binding
	Delete        key.Binding
	Restore       key.Binding
	Artifact      key.Binding
	ClaimNext     key.Binding
	Renew         key.Binding
	Projects      key.Binding
	Project       key.Binding
	Fields        key.Binding
	Settings      key.Binding
	Activity      key.Binding
	History       key.Binding
	Stats         key.Binding
	Tenant        key.Binding
	Refresh       key.Binding
	Help          key.Binding
	Quit          key.Binding
	Interrupt     key.Binding
	Accept        key.Binding
	Commit        key.Binding
	Cancel        key.Binding
	Agree         key.Binding
	NextField     key.Binding
	PrevField     key.Binding
}

// DefaultKeyMap returns the shipped bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:       key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "column left")),
		Right:      key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "column right")),
		Top:        key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "first")),
		Bottom:     key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "last")),
		Enter:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:       key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		ClearFltr:  key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "clear filter")),
		Claim:      key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "claim")),
		Release:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "release")),
		Transition: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "transition")),
		New:        key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new task")),
		NewProject: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new project")),
		// One edit key, not one per field. The form it opens already holds the
		// title, so a second binding for the title alone was a slower way to
		// do part of what this does.
		Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit task")),
		Priority: key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "set priority")),
		// p steps one place down the scale and wraps at the bottom, so every
		// priority is reachable without leaving the board. P still picks one
		// outright, which is the shorter trip when the reader knows where they
		// are going.
		CyclePriority: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "cycle priority")),
		Assign:        key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "set assignee")),
		Comment:       key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "comment")),
		Tag:           key.NewBinding(key.WithKeys("#"), key.WithHelp("#", "add tag")),
		Untag:         key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "remove tag")),
		// # and U take a name the reader already knows. L shows the names the
		// tenant has, which is what a reader who does not know them needs.
		Tags:        key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "pick a tag")),
		Depend:      key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "add dependency")),
		Undepend:    key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "remove dependency")),
		CommentEdit: key.NewBinding(key.WithKeys("M"), key.WithHelp("M", "edit comment")),
		Delete:      key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "delete")),
		// u, for undoing the deletion. It is the one key on a deleted card, and
		// vim's own undo, which is the reflex a reader arrives with.
		Restore: key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "restore")),
		// O, for the output a worker records. Lowercase o opens a new task under
		// vim and helix, so the capital is what the other taken letters use.
		Artifact:  key.NewBinding(key.WithKeys("O"), key.WithHelp("O", "record artifact")),
		ClaimNext: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "claim next")),
		Renew:     key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "renew lease")),
		// W, not p: p cycles the selected task's priority, and P is the picker
		// that sets one outright, so the project list takes the capital of the
		// project screen's own key rather than a third letter unrelated to
		// either.
		Projects: key.NewBinding(key.WithKeys("W"), key.WithHelp("W", "projects")),
		// w, for the workflow, which is the largest thing the screen shows. The
		// obvious letters are spoken for: p lists projects and , opens the
		// session's own preferences, which are a different kind of setting.
		Project:  key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "project setup")),
		Fields:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "custom fields")),
		Settings: key.NewBinding(key.WithKeys(","), key.WithHelp(",", "settings")),
		Activity: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "activity")),
		// H, for the stored history, beside v for the live tail. Lowercase h
		// moves a column, so the capital is what the taken letters already use.
		History: key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "history")),
		// S, not s: lowercase s is taken on the board, and a capital is what
		// the other cross-view keys already use when their letter is spoken
		// for.
		Stats:     key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "statistics")),
		Tenant:    key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "tenant")),
		Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Interrupt: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "interrupt")),
		Accept:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
		// A second way to apply, for the fields enter cannot end. Inside a
		// multi-line field enter is a newline, which is the whole point of the
		// field, so the form needs a key that never means anything else.
		Commit: key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "apply")),
		Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		// Not enter. A confirmation answered by the key every other input is
		// accepted with is answered by reflex, which is the habit a destructive
		// confirmation exists to interrupt.
		Agree: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm")),
		// tab and shift+tab, taken off column movement. A form holding a text
		// field cannot spend the arrows on moving between fields, because the
		// field the reader is typing in needs them.
		NextField: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
		PrevField: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous field")),
	}
}

// HelpEntry is one line of the help view.
type HelpEntry struct {
	Keys string
	Desc string
}

// entry renders a binding as a help line.
func entry(b key.Binding) HelpEntry {
	return HelpEntry{Keys: b.Help().Key, Desc: b.Help().Desc}
}

// globalKey is one cross-view binding and the view it opens. A binding that
// opens no view, such as refresh, carries none and is always offered.
type globalKey struct {
	binding key.Binding
	opens   viewKind
	always  bool
}

// globalKeys pairs each cross-view binding with the view it opens, so the help
// overlay and the key that opens the view cannot disagree about which views a
// reader has.
func (k KeyMap) globalKeys() []globalKey {
	return []globalKey{
		{binding: k.Help, always: true},
		{binding: k.Refresh, always: true},
		{binding: k.Projects, opens: viewProjects},
		{binding: k.Project, opens: viewProject},
		{binding: k.Settings, opens: viewSettings},
		{binding: k.Activity, opens: viewActivity},
		{binding: k.History, opens: viewHistory},
		{binding: k.Stats, opens: viewStats},
		{binding: k.Tenant, opens: viewTenant},
		{binding: k.Quit, always: true},
		{binding: k.Interrupt, always: true},
	}
}

// GlobalHelp lists the bindings that work in every view, leaving out the ones
// that open a view this reader is not offered. A key documented in the overlay
// is a promise that pressing it does something, and the overlay used to render
// every binding regardless of who was reading it.
//
// A nil predicate offers none of the views, the same closed default the
// interface itself takes.
func (k KeyMap) GlobalHelp(offered func(viewKind) bool) []HelpEntry {
	out := make([]HelpEntry, 0, len(k.globalKeys()))
	for _, g := range k.globalKeys() {
		if !g.always && (offered == nil || !offered(g.opens)) {
			continue
		}
		out = append(out, entry(g.binding))
	}
	return out
}

// gatedAction pairs a binding with the registry operations whose authority it
// needs. A binding listed with several is offered when any one of them is
// permitted, which is what the delete key wants: it removes a task or a
// comment, and the two are separate authorities.
type gatedAction struct {
	binding key.Binding
	methods []string
	// needs are the operations the action cannot do without, on top of the one
	// it performs. The tag picker lists before it changes, so a reader who may
	// change tags and not read them is offered nothing rather than a picker
	// with nothing in it.
	needs []string
}

// permitted reports whether a reader may press this binding. A nil predicate
// permits nothing, the same closed default the rest of the interface takes, and
// a binding naming no operation is always offered.
func (g gatedAction) permitted(may func(string) bool) bool {
	if len(g.methods) == 0 && len(g.needs) == 0 {
		return true
	}
	if may == nil {
		return false
	}
	for _, method := range g.needs {
		if !may(method) {
			return false
		}
	}
	if len(g.methods) == 0 {
		return true
	}
	for _, method := range g.methods {
		if may(method) {
			return true
		}
	}
	return false
}

// taskBindings are the actions that act on the selected task, each against the
// operation it calls. One list, so the footer, the help overlay and the
// keystroke cannot disagree about who may press a key.
func (k KeyMap) taskBindings() []gatedAction {
	return []gatedAction{
		{binding: k.New, methods: []string{"CreateTask"}},
		{binding: k.ClaimNext, methods: []string{"ClaimNext"}},
		{binding: k.Claim, methods: []string{"ClaimTask"}},
		{binding: k.Release, methods: []string{"ReleaseLease"}},
		{binding: k.Transition, methods: []string{"TransitionTask"}},
		{binding: k.Edit, methods: []string{"UpdateTask"}},
		{binding: k.Priority, methods: []string{"UpdateTask"}},
		{binding: k.CyclePriority, methods: []string{"UpdateTask"}},
		{k.Assign, []string{"UpdateTask"}, []string{"ListActors"}},
		{binding: k.Comment, methods: []string{"AddComment"}},
		{binding: k.CommentEdit, methods: []string{"EditComment"}},
		{binding: k.Tag, methods: []string{"AddTag"}},
		{binding: k.Untag, methods: []string{"RemoveTag"}},
		{k.Tags, []string{"AddTag", "RemoveTag"}, []string{"ListTags"}},
		{binding: k.Depend, methods: []string{"AddDependency"}},
		{binding: k.Undepend, methods: []string{"RemoveDependency"}},
		{binding: k.Delete, methods: []string{"DeleteTask", "DeleteComment"}},
		{binding: k.Restore, methods: []string{"RestoreTask"}},
		{binding: k.Artifact, methods: []string{"PutArtifact"}},
		{binding: k.Renew, methods: []string{"RenewLease"}},
	}
}

// taskActions are the bindings that act on the selected task, offered wherever
// a task is selected so the board and the detail view stay in step, and only as
// far as this reader's authority reaches: a key documented for an operation the
// service will refuse tells the reader the refusal was their mistake.
func (k KeyMap) taskActions(may func(string) bool) []HelpEntry {
	out := make([]HelpEntry, 0, len(k.taskBindings()))
	for _, a := range k.taskBindings() {
		if a.permitted(may) {
			out = append(out, entry(a.binding))
		}
	}
	return out
}

// ViewHelp lists the bindings of one view, as far as this reader's authority
// reaches. A nil predicate offers none of the gated actions.
func (k KeyMap) ViewHelp(v viewKind, may func(string) bool) []HelpEntry {
	switch v {
	case viewProjects:
		return []HelpEntry{
			entry(k.Up), entry(k.Down), entry(k.Top), entry(k.Bottom),
			entry(k.Enter), entry(k.NewProject),
		}
	case viewBoard:
		return append([]HelpEntry{
			entry(k.Left), entry(k.Right), entry(k.Up), entry(k.Down), entry(k.Enter),
			entry(k.Filter), entry(k.ClearFltr),
		}, append(k.taskActions(may), entry(k.Back))...)
	case viewDetail:
		return append(append([]HelpEntry{entry(k.Up), entry(k.Down)}, k.commentHelp()...),
			append(k.taskActions(may), entry(k.Back))...)
	case viewSettings:
		return append([]HelpEntry{entry(k.Up), entry(k.Down)},
			append(k.settingHelp(), entry(k.Back))...)
	case viewActivity:
		return []HelpEntry{
			entry(k.Up), entry(k.Down), entry(k.Top), entry(k.Bottom),
			entry(k.Filter), entry(k.ClearFltr), entry(k.Back),
		}
	case viewTenant:
		return append([]HelpEntry{
			entry(k.Up), entry(k.Down), entry(k.Top), entry(k.Bottom), entry(k.Enter),
		}, append(k.tenantActions(may), entry(k.Back))...)
	case viewHistory:
		return []HelpEntry{
			entry(k.Up), entry(k.Down), entry(k.Top), entry(k.Bottom), entry(k.Back),
		}
	case viewProject:
		return append([]HelpEntry{entry(k.Up), entry(k.Down)},
			append(k.projectActions(may), entry(k.Back))...)
	default:
		return []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Back)}
	}
}

// viewAction is one of a configuration screen's bindings: the key it borrows
// from the board, the authority it needs, and what it does here. The description
// is its own because "edit title" on a screen holding no task describes the
// board.
type viewAction struct {
	gatedAction
	desc string
}

// projectBindings are the actions the project screen offers, each against the
// operation it calls. One list, so the footer, the help overlay and the
// keystroke cannot disagree about who may press a key.
//
// The keys are the board's own, which is what keeps every scheme working here
// without rebinding anything: a reader who moved Edit onto i edits a
// project with i too.
func (k KeyMap) projectBindings() []viewAction {
	return []viewAction{
		{gatedAction{binding: k.Edit, methods: []string{"UpdateProject"}}, "edit project"},
		{gatedAction{binding: k.Delete,
			methods: []string{"ArchiveProject", "DeleteProject"}}, "archive or delete"},
		{gatedAction{binding: k.New, methods: []string{"PutFieldDef"}}, "new field"},
		{gatedAction{binding: k.Fields, methods: []string{"PutFieldDef", "DeleteFieldDef"},
			needs: []string{"ListFieldDefs"}}, "custom fields"},
	}
}

// entry renders a project action as a help line, under the description the
// project screen gives it.
func (a viewAction) entry() HelpEntry {
	return HelpEntry{Keys: a.binding.Help().Key, Desc: a.desc}
}

// projectActions are the project screen's bindings, as far as this reader's
// authority reaches.
func (k KeyMap) projectActions(may func(string) bool) []HelpEntry {
	out := make([]HelpEntry, 0, len(k.projectBindings()))
	for _, a := range k.projectBindings() {
		if a.permitted(may) {
			out = append(out, a.entry())
		}
	}
	return out
}

// tenantBindings are the actions the tenant screen offers, each against the
// operation it calls. One list, so the footer, the help overlay and the
// keystroke cannot disagree about who may press a key.
//
// The keys are the board's own, which is what keeps every scheme working here
// without rebinding anything: a reader who moved New onto o adds a domain with
// o too.
func (k KeyMap) tenantBindings() []viewAction {
	return []viewAction{
		{gatedAction{binding: k.Edit, methods: []string{"UpdateTenant"}}, "edit tenant"},
		{gatedAction{binding: k.New, methods: []string{"AddDomain", "AddMember"}}, "add domain or member"},
		{gatedAction{binding: k.Delete,
			methods: []string{"RemoveDomain", "RemoveMember"}}, "remove the selected row"},
	}
}

// tenantActions are the tenant screen's bindings, as far as this reader's
// authority reaches.
func (k KeyMap) tenantActions(may func(string) bool) []HelpEntry {
	out := make([]HelpEntry, 0, len(k.tenantBindings()))
	for _, a := range k.tenantBindings() {
		if a.permitted(may) {
			out = append(out, a.entry())
		}
	}
	return out
}

// tenantShort offers the tenant screen's keys, dropping the removal when the
// screen holds no domain and no member: a key that can only report having
// nothing to act on is a promise the footer should not make.
func (k KeyMap) tenantShort(ctx ActionContext) []HelpEntry {
	out := make([]HelpEntry, 0, len(k.tenantBindings()))
	for _, a := range k.tenantBindings() {
		if !a.permitted(ctx.May) {
			continue
		}
		if a.desc == "remove the selected row" && !ctx.HasRows {
			continue
		}
		out = append(out, a.entry())
	}
	return out
}

// commentHelp relabels the column keys for the detail view, where they step
// through the comment thread rather than between columns. The detail view has no
// columns, so the keys were idle there, and a thread whose entries cannot be
// selected is a thread whose entries cannot be edited or removed.
func (k KeyMap) commentHelp() []HelpEntry {
	return []HelpEntry{
		{Keys: k.Left.Help().Key, Desc: "previous comment"},
		{Keys: k.Right.Help().Key, Desc: "next comment"},
	}
}

// settingHelp relabels the column keys for the settings view, where they step
// through a setting's values rather than moving between columns. A footer that
// promised "column left" on a screen with no columns was describing the board.
func (k KeyMap) settingHelp() []HelpEntry {
	return []HelpEntry{
		{Keys: k.Left.Help().Key, Desc: "previous value"},
		{Keys: k.Right.Help().Key, Desc: "next value"},
	}
}

// ActionContext is what the footer needs to know about the selected task in
// order to decide which keys it may offer.
type ActionContext struct {
	HasProject    bool
	HasTask       bool
	HeldHere      bool
	HeldElsewhere bool
	CanTransition bool
	// HasComment reports whether the open task has a thread to step through,
	// so the detail view does not advertise a cursor over nothing.
	HasComment bool
	// HasFields reports whether the open project defines custom fields, so the
	// project screen does not advertise a picker with nothing to pick.
	HasFields bool
	// HasRows reports whether the tenant screen holds a domain or a member, so
	// it does not advertise a removal with nothing under the cursor.
	HasRows bool
	// IsDeleted reports whether the selected card is one of the deleted tasks a
	// filter revealed. A deleted task accepts one action, so the footer offers
	// that one and none of the editing keys the service would refuse.
	IsDeleted bool
	// May reports whether this reader holds the authority one operation needs,
	// asked of the capability registry. Nil offers none of the gated actions.
	May func(string) bool
}

// ShortHelp lists the bindings a view advertises on its footer. A key in the
// footer is a promise that pressing it will work, so an action the selected
// task cannot accept is left out rather than offered and then refused. The
// help view still documents every binding the view has.
func (k KeyMap) ShortHelp(v viewKind, ctx ActionContext) []HelpEntry {
	var short []HelpEntry
	switch v {
	case viewProjects:
		short = []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Enter), entry(k.NewProject)}
	case viewSettings:
		short = append([]HelpEntry{entry(k.Up), entry(k.Down)},
			append(k.settingHelp(), entry(k.Back))...)
	case viewBoard:
		short = []HelpEntry{entry(k.Enter)}
		if ctx.HasProject {
			short = append(short, k.offer(ctx, gatedAction{binding: k.New, methods: []string{"CreateTask"}})...)
		}
		short = append(short, k.claimHelp(ctx)...)
		short = append(short, k.editHelp(ctx)...)
		short = append(short, entry(k.Filter), entry(k.Back))
	case viewDetail:
		short = append(k.editHelp(ctx), k.claimHelp(ctx)...)
		if ctx.HasComment {
			short = append(short, k.commentHelp()...)
		}
		short = append(short, entry(k.Back))
	case viewActivity:
		short = []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Filter), entry(k.Back)}
	case viewTenant:
		short = append([]HelpEntry{entry(k.Up), entry(k.Down), entry(k.Enter)},
			append(k.tenantShort(ctx), entry(k.Back))...)
	case viewHistory:
		short = []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Back)}
	case viewProject:
		short = append(k.projectShort(ctx), entry(k.Back))
	default:
		short = []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Back)}
	}
	return append(short, entry(k.Help), entry(k.Quit))
}

// projectShort offers the project screen's keys, dropping the field picker when
// the project defines no fields: a picker with nothing in it is an entry leading
// nowhere.
func (k KeyMap) projectShort(ctx ActionContext) []HelpEntry {
	out := make([]HelpEntry, 0, len(k.projectBindings()))
	for _, a := range k.projectBindings() {
		if !a.permitted(ctx.May) {
			continue
		}
		if a.desc == "custom fields" && !ctx.HasFields {
			continue
		}
		out = append(out, a.entry())
	}
	return out
}

// claimHelp offers only the lease actions the selected task can accept: claim
// while it is free, release and renew while this session holds it, and neither
// while another worker does.
func (k KeyMap) claimHelp(ctx ActionContext) []HelpEntry {
	switch {
	case !ctx.HasTask, ctx.IsDeleted:
		return nil
	case ctx.HeldHere:
		return k.offer(ctx, gatedAction{binding: k.Release, methods: []string{"ReleaseLease"}},
			gatedAction{binding: k.Renew, methods: []string{"RenewLease"}})
	case ctx.HeldElsewhere:
		return nil
	default:
		return k.offer(ctx, gatedAction{binding: k.Claim, methods: []string{"ClaimTask"}})
	}
}

// editHelp offers the editing actions a selected task can accept, and the
// transition only where the workflow permits one out of its current state.
func (k KeyMap) editHelp(ctx ActionContext) []HelpEntry {
	if !ctx.HasTask {
		return nil
	}
	if ctx.IsDeleted {
		return k.offer(ctx, gatedAction{binding: k.Restore, methods: []string{"RestoreTask"}})
	}
	out := k.offer(ctx, gatedAction{binding: k.Edit, methods: []string{"UpdateTask"}},
		gatedAction{binding: k.CyclePriority, methods: []string{"UpdateTask"}},
		gatedAction{binding: k.Comment, methods: []string{"AddComment"}})
	if ctx.CanTransition {
		out = append(out, k.offer(ctx, gatedAction{binding: k.Transition, methods: []string{"TransitionTask"}})...)
	}
	return append(out, k.offer(ctx, gatedAction{binding: k.Delete, methods: []string{"DeleteTask", "DeleteComment"}})...)
}

// offer keeps the bindings this reader's authority reaches.
func (k KeyMap) offer(ctx ActionContext, actions ...gatedAction) []HelpEntry {
	out := make([]HelpEntry, 0, len(actions))
	for _, a := range actions {
		if a.permitted(ctx.May) {
			out = append(out, entry(a.binding))
		}
	}
	return out
}
