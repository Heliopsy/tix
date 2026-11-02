// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import "strings"

// Interface is one thing a session can be opened on: a row on the start menu
// and, over SSH, the command word that opens it.
type Interface struct {
	// Name is the command word and the menu row's identity, lower case.
	Name string
	// Title is what the start menu calls it.
	Title string
	// Summary says in one line what it is.
	Summary string
	// Usage is the command line that opens it, as a refusal prints it.
	Usage string
	opens viewKind
}

// Interfaces is the one list of interfaces. The start menu draws a row for
// each, the SSH listener keeps a launcher for each and prints each one's usage
// when it refuses a command, and tests fail when either reader misses one.
func Interfaces() []Interface {
	return []Interface{
		{
			Name: "board", Title: "Boards", opens: viewProjects,
			Summary: "the project list; open a board and work its tasks",
			Usage:   "board [KEY]",
		},
		{
			Name: "watch", Title: "Watch", opens: viewActivity,
			Summary: "live events as they are committed, optionally for one project",
			Usage:   "watch [KEY...] [--project REF] [--type TYPE] [--actor ID] [--since N] [--limit N] [-o ndjson]",
		},
	}
}

// LookupInterface finds a registered interface by its command word, compared
// without regard to case because the project keys the word competes with are.
func LookupInterface(name string) (Interface, bool) {
	for _, i := range Interfaces() {
		if strings.EqualFold(i.Name, name) {
			return i, true
		}
	}
	return Interface{}, false
}
