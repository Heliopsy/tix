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
	Projects   key.Binding
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
		Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		ClearFltr:  key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "clear filter")),
		Claim:      key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "claim")),
		Release:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "release")),
		Transition: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "transition")),
		Projects:   key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "projects")),
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
	return []HelpEntry{entry(k.Help), entry(k.Refresh), entry(k.Quit), entry(k.Interrupt)}
}

// ViewHelp lists the bindings of one view.
func (k KeyMap) ViewHelp(v viewKind) []HelpEntry {
	switch v {
	case viewProjects:
		return []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Top), entry(k.Bottom), entry(k.Enter)}
	case viewBoard:
		return []HelpEntry{
			entry(k.Left), entry(k.Right), entry(k.Up), entry(k.Down), entry(k.Enter),
			entry(k.Filter), entry(k.ClearFltr), entry(k.Claim), entry(k.Release),
			entry(k.Transition), entry(k.Projects), entry(k.Back),
		}
	case viewDetail:
		return []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Claim), entry(k.Release), entry(k.Transition), entry(k.Back)}
	default:
		return []HelpEntry{entry(k.Back)}
	}
}

// ShortHelp lists the bindings a view advertises on its footer. It names the
// actions rather than the first few of ViewHelp: taking the first four left
// the board advertising its four arrow keys and hiding the one binding that
// opens a task, so the screen looked like it could only be scrolled.
func (k KeyMap) ShortHelp(v viewKind) []HelpEntry {
	var short []HelpEntry
	switch v {
	case viewProjects:
		short = []HelpEntry{entry(k.Up), entry(k.Down), entry(k.Enter)}
	case viewBoard:
		short = []HelpEntry{entry(k.Enter), entry(k.Claim), entry(k.Transition), entry(k.Filter)}
	case viewDetail:
		short = []HelpEntry{entry(k.Claim), entry(k.Release), entry(k.Transition), entry(k.Back)}
	default:
		short = []HelpEntry{entry(k.Back)}
	}
	return append(short, entry(k.Help), entry(k.Quit))
}
