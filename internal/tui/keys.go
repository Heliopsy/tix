// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap holds every binding the interface offers.
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	Top        key.Binding
	Bottom     key.Binding
	Enter      key.Binding
	Back       key.Binding
	Filter     key.Binding
	ClearFltr  key.Binding
	Claim      key.Binding
	Release    key.Binding
	Transition key.Binding
	New        key.Binding
	NewProject key.Binding
	EditTitle  key.Binding
	EditBody   key.Binding
	Priority   key.Binding
	Assign     key.Binding
	Comment    key.Binding
	Tag        key.Binding
	Untag      key.Binding
	Depend     key.Binding
	ClaimNext  key.Binding
	Renew      key.Binding
	Projects   key.Binding
	Settings   key.Binding
	Activity   key.Binding
	Tenant     key.Binding
	Refresh    key.Binding
	Help       key.Binding
	Quit       key.Binding
	Interrupt  key.Binding
	Accept     key.Binding
	Cancel     key.Binding
}

// DefaultKeyMap returns the shipped bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:       key.NewBinding(key.WithKeys("left", "h", "shift+tab"), key.WithHelp("←/h", "column left")),
		Right:      key.NewBinding(key.WithKeys("right", "l", "tab"), key.WithHelp("→/l", "column right")),
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
		EditTitle:  key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit title")),
		EditBody:   key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "edit body")),
		Priority:   key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "set priority")),
		Assign:     key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "set assignee")),
		Comment:    key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "comment")),
		Tag:        key.NewBinding(key.WithKeys("#"), key.WithHelp("#", "add tag")),
		Untag:      key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "remove tag")),
		Depend:     key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "add dependency")),
		ClaimNext:  key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "claim next")),
		Renew:      key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "renew lease")),
		Projects:   key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "projects")),
		Settings:   key.NewBinding(key.WithKeys(","), key.WithHelp(",", "settings")),
		Activity:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "activity")),
		Tenant:     key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "tenant")),
		Refresh:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:       key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Interrupt:  key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "interrupt")),
		Accept:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
		Cancel:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
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

// GlobalHelp lists the bindings that work in every view.
func (k KeyMap) GlobalHelp() []HelpEntry {
	return []HelpEntry{
		entry(k.Help), entry(k.Refresh), entry(k.Projects), entry(k.Settings), entry(k.Activity),
		entry(k.Tenant), entry(k.Quit), entry(k.Interrupt),
	}
}

// taskActions are the bindings that act on the selected task, offered wherever
// a task is selected so the board and the detail view stay in step.
func (k KeyMap) taskActions() []HelpEntry {
	return []HelpEntry{
		entry(k.Claim), entry(k.Release), entry(k.Transition), entry(k.EditTitle),
		entry(k.EditBody), entry(k.Priority), entry(k.Assign), entry(k.Comment),
		entry(k.Tag), entry(k.Untag), entry(k.Depend), entry(k.Renew),
	}
}

// ViewHelp lists the bindings of one view.
func (k KeyMap) ViewHelp(v viewKind) []HelpEntry {
	switch v {
	case viewProjects:
		return []HelpEntry{
			entry(k.Up), entry(k.Down), entry(k.Top), entry(k.Bottom),
			entry(k.Enter), entry(k.NewProject),
		}
	case viewBoard:
		return append([]HelpEntry{
			entry(k.Left), entry(k.Right), entry(k.Up), entry(k.Down), entry(k.Enter),
			entry(k.Filter), entry(k.ClearFltr), entry(k.New), entry(k.ClaimNext),
		}, append(k.taskActions(), entry(k.Back))...)
	case viewDetail:
		return append([]HelpEntry{entry(k.Up), entry(k.Down), entry(k.New)},
			append(k.taskActions(), entry(k.Back))...)
	case viewSettings:
		return []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Enter), entry(k.Back)}
	case viewActivity:
		return []HelpEntry{
			entry(k.Up), entry(k.Down), entry(k.Top), entry(k.Bottom),
			entry(k.Filter), entry(k.ClearFltr), entry(k.Back),
		}
	case viewTenant:
		return []HelpEntry{entry(k.Enter), entry(k.Back)}
	default:
		return []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Back)}
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
		short = []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Enter), entry(k.Back)}
	case viewBoard:
		short = []HelpEntry{entry(k.Enter)}
		if ctx.HasProject {
			short = append(short, entry(k.New))
		}
		short = append(short, k.claimHelp(ctx)...)
		short = append(short, k.editHelp(ctx)...)
		short = append(short, entry(k.Filter), entry(k.Back))
	case viewDetail:
		short = append(k.editHelp(ctx), k.claimHelp(ctx)...)
		short = append(short, entry(k.Back))
	case viewActivity:
		short = []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Filter), entry(k.Back)}
	case viewTenant:
		short = []HelpEntry{entry(k.Enter), entry(k.Back)}
	default:
		short = []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Back)}
	}
	return append(short, entry(k.Help), entry(k.Quit))
}

// claimHelp offers only the lease actions the selected task can accept: claim
// while it is free, release and renew while this session holds it, and neither
// while another worker does.
func (k KeyMap) claimHelp(ctx ActionContext) []HelpEntry {
	switch {
	case !ctx.HasTask:
		return nil
	case ctx.HeldHere:
		return []HelpEntry{entry(k.Release), entry(k.Renew)}
	case ctx.HeldElsewhere:
		return nil
	default:
		return []HelpEntry{entry(k.Claim)}
	}
}

// editHelp offers the editing actions a selected task can accept, and the
// transition only where the workflow permits one out of its current state.
func (k KeyMap) editHelp(ctx ActionContext) []HelpEntry {
	if !ctx.HasTask {
		return nil
	}
	out := []HelpEntry{entry(k.EditTitle), entry(k.Comment)}
	if ctx.CanTransition {
		out = append(out, entry(k.Transition))
	}
	return out
}
