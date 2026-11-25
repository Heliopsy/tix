// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// clipboardText is the text a command asks the terminal's clipboard to hold
// with OSC 52, or fails when it asks for anything else. bubbletea keeps the
// message type unexported, so it is recognised by name and read as the string
// it is.
func clipboardText(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		t.Fatal("the share sent no command")
	}
	msg := cmd()
	if got := fmt.Sprintf("%T", msg); got != "tea.setClipboardMsg" {
		t.Fatalf("the share sent %s, not an OSC 52 clipboard write", got)
	}
	return reflect.ValueOf(msg).String()
}

// shareBoard is the three task board with every task carrying a reference
// of its project's shape and, when linked, the web_url a server with
// server.public_url sets.
func shareBoard(t *testing.T, linked bool) Model {
	t.Helper()
	m := boardModel(t)
	tasks := slices.Clone(m.tasks)
	for i := range tasks {
		tasks[i].Ref = fmt.Sprintf("infra-%d", tasks[i].Seq)
		tasks[i].Title = "Fix [the] build"
		if linked {
			tasks[i].WebURL = "https://tix.example.com/tasks/" + tasks[i].Ref
		}
	}
	m, _ = m.installTasks(tasks)
	return m
}

// The share key on a card offers what the web Share menu copies, and the
// pick goes to the terminal's clipboard and onto the screen alike.
func TestTheShareKeyCopiesEachFormToTheClipboard(t *testing.T) {
	cases := []struct{ digit, label, want string }{
		{"1", "link", "https://tix.example.com/tasks/infra-1"},
		{"2", "Markdown", `[INFRA-1 Fix \[the\] build](https://tix.example.com/tasks/infra-1)`},
		{"3", "key and title", "INFRA-1 Fix [the] build"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			m := shareBoard(t, true)
			if got, _ := TaskAt(m.columns, m.sel); got.ID != "a" {
				t.Fatalf("selected %q, want a", got.ID)
			}
			m, cmd := m.reduce(pressKey("^"))
			if m.choice != choiceShare || cmd != nil {
				t.Fatalf("the share key opened %v and sent %v, want the share picker and nothing sent", m.choice, cmd)
			}
			if panel := strings.Join(m.inputPanel(), "\n"); !strings.Contains(panel, "1) link") ||
				!strings.Contains(panel, "2) Markdown") || !strings.Contains(panel, "3) key and title") {
				t.Fatalf("the share picker lacks a form:\n%s", panel)
			}
			m, cmd = m.reduce(pressKey(tc.digit))
			if got := clipboardText(t, cmd); got != tc.want {
				t.Fatalf("copied %q, want %q", got, tc.want)
			}
			shown := stripANSI(strings.Join(m.inputPanel(), "\n"))
			if !strings.Contains(shown, "copied "+tc.label) || !strings.Contains(shown, tc.want) {
				t.Fatalf("the copied text is not on screen:\n%s", shown)
			}
			if !strings.Contains(stripANSI(m.Frame()), tc.want) {
				t.Fatalf("the frame does not show the copied text")
			}
			m, cmd = m.reduce(pressKey("j"))
			if m.shared != nil || cmd != nil || m.sel.Row != 0 {
				t.Fatalf("a key after the copy did more than close the reveal: shared %v, row %d", m.shared, m.sel.Row)
			}
		})
	}
}

// On the task screen the share key shares the task it shows.
func TestTheShareKeyWorksOnTheTaskScreen(t *testing.T) {
	m := detailModel(t, 120, 40)
	m.detail.task.WebURL = "https://tix.example.com/tasks/" + m.detail.task.Ref
	m, _ = m.reduce(pressKey("^"))
	want := m.detail.task.WebURL
	_, cmd := m.reduce(pressKey("1"))
	if got := clipboardText(t, cmd); got != want {
		t.Fatalf("copied %q, want the task's web_url", got)
	}
}

// A remote context links a task under its server when the server set no
// web_url; with neither there is no link to copy, and the picker says why.
func TestTheShareLinkFallsBackAndSaysWhyItHasNone(t *testing.T) {
	m := shareBoard(t, false)
	m.webBase = "https://remote.example"
	m, _ = m.reduce(pressKey("^"))
	_, cmd := m.reduce(pressKey("1"))
	if got := clipboardText(t, cmd); got != "https://remote.example/tasks/infra-1" {
		t.Fatalf("copied %q, want the task under the context's server", got)
	}

	m = shareBoard(t, false)
	m, _ = m.reduce(pressKey("^"))
	panel := stripANSI(strings.Join(m.inputPanel(), "\n"))
	if !strings.Contains(panel, "server.public_url") || strings.Contains(panel, ") link") {
		t.Fatalf("without an address the picker offers a link or does not say why:\n%s", panel)
	}
	if len(m.choices) != 2 {
		t.Fatalf("offered %d forms without an address, want key and title and the Markdown label", len(m.choices))
	}
	m, cmd = m.reduce(pressKey("1"))
	if got := clipboardText(t, cmd); got != "INFRA-1 Fix [the] build" {
		t.Fatalf("copied %q, want the key and title", got)
	}
	m, _ = m.reduce(pressKey("esc"))
	m, _ = m.reduce(pressKey("^"))
	_, cmd = m.reduce(pressKey("2"))
	if got := clipboardText(t, cmd); got != `INFRA-1 Fix \[the\] build` {
		t.Fatalf("copied %q, want the Markdown label", got)
	}
}

// The share key is in the help overlay and the palette, like every action.
func TestTheShareKeyIsInTheHelpAndThePalette(t *testing.T) {
	m := shareBoard(t, true)
	help := m.keys.ViewHelp(viewBoard, func(string) bool { return true })
	if !slices.Contains(help, HelpEntry{Keys: "^", Desc: "share task"}) {
		t.Errorf("the board's help has no share entry: %v", help)
	}
	detail := m.keys.ViewHelp(viewDetail, func(string) bool { return true })
	if !slices.Contains(detail, HelpEntry{Keys: "^", Desc: "share task"}) {
		t.Errorf("the task screen's help has no share entry: %v", detail)
	}
	found := false
	for _, a := range m.keys.Actions() {
		found = found || (a.id == doShare && !a.hidden && a.Keys() == "^")
	}
	if !found {
		t.Error("the palette's action list has no share entry")
	}
}

func TestWrapCellsKeepsEveryCharacter(t *testing.T) {
	long := "https://tix.example.com/tasks/infra-123456789"
	lines := wrapCells(long, 10)
	if strings.Join(lines, "") != long || len(lines) != 5 {
		t.Fatalf("wrapped into %q", lines)
	}
	if got := wrapCells("short", 10); len(got) != 1 || got[0] != "short" {
		t.Fatalf("a short line was wrapped: %q", got)
	}
}
