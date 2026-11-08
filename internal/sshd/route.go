// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/tui"
)

// launch is what a session's command asked the listener to host.
type launch struct {
	// iface is the registered interface's name.
	iface string
	// project is the board to open, and start lands on the start menu
	// instead of the project list when none is named.
	project string
	start   bool
	watch   watchRequest
}

// launcher turns the words after an interface's name into a launch.
type launcher func(s *Server, args []string) (launch, error)

// launchers holds one launcher per interface tui.Interfaces registers, keyed
// by its name. A test fails when the two lists disagree.
var launchers = map[string]launcher{
	"board": (*Server).launchBoard,
	"watch": (*Server).launchWatch,
}

// route reads a session's command.
//
// The first word names a registered interface, compared without regard to
// case, and the rest are its arguments. A single word that names none is a
// project key, which is what the command always was: `ssh tix@host infra`
// opens that board as `tix tui -p infra` does locally. The registered names are
// therefore reserved, and `board KEY` opens a project spelled like one.
//
// With nothing named, a sandbox opens its seeded board, since it holds exactly
// one and opening it is the demo, and a hosted session lands on the start menu.
//
// A key is checked for shape here rather than left to the interface: an
// unknown-but-plausible key is the interface's to answer, against the projects
// this actor can actually reach, but a 4KB argument or one carrying control
// characters is not a board anybody has and would be drawn into an error line
// on the way to saying so.
func (s *Server) route(command []string) (launch, error) {
	if len(command) == 0 {
		if s.opts.Demo {
			return launch{iface: "board", project: seedProjectKey}, nil
		}
		return launch{iface: "board", start: true}, nil
	}
	if i, ok := tui.LookupInterface(command[0]); ok {
		if run, ok := launchers[i.Name]; ok {
			return run(s, command[1:])
		}
	}
	if len(command) == 1 {
		return boardFor(command[0])
	}
	return launch{}, errors.New("tix serves its interfaces and commands, not a shell: " + commandUsage())
}

// launchBoard opens the project list, or the board a key names.
func (s *Server) launchBoard(args []string) (launch, error) {
	switch len(args) {
	case 0:
		return launch{iface: "board"}, nil
	case 1:
		return boardFor(args[0])
	default:
		return launch{}, errors.New("board takes one project key: " + commandUsage())
	}
}

// launchWatch reads watch's filters.
func (s *Server) launchWatch(args []string) (launch, error) {
	req, err := parseWatch(args)
	if err != nil {
		return launch{}, err
	}
	return launch{iface: "watch", watch: req}, nil
}

// boardFor opens the board key names, once it is shaped like a key.
func boardFor(key string) (launch, error) {
	if err := core.ValidateProjectKey(key); err != nil {
		return launch{}, fmt.Errorf("%w; %s", err, commandUsage())
	}
	return launch{iface: "board", project: key}, nil
}

// commandUsage names every command a session may send, from the registry, so
// a refusal teaches what exists rather than only what does not.
func commandUsage() string {
	usages := make([]string, 0, len(tui.Interfaces()))
	for _, i := range tui.Interfaces() {
		usages = append(usages, "`"+i.Usage+"`")
	}
	return "run one of " + strings.Join(usages, ", ") + ", name a project key to open its board, " +
		"or run a tix command such as `task ls`"
}
