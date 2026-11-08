// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// helpFrame opens the help overlay over m on a terminal tall enough to hold it.
func helpFrame(t *testing.T, m Model) string {
	t.Helper()
	m.width, m.height = 140, 80
	under := m.view
	m, _ = m.reduce(pressKey("?"))
	if m.view != viewHelp {
		t.Fatalf("? over %s did not open help", viewName(under))
	}
	return m.Frame()
}

func TestTheHelpOverlayNamesEachAdministrationViewsActions(t *testing.T) {
	integ, _ := integrationsModel(t)
	for _, tc := range []struct {
		name string
		m    Model
		want []string
	}{
		{"server", serverModel(t, serverFake()), []string{"end the selected connection"}},
		{"global admin", globalModel(t, globalFake()), []string{
			"open users or account log", "create tenant or grant global admin",
			"delete tenant or revoke global admin", "edit the selected user", "account log by email",
		}},
		{"integrations", integ, []string{"previous, next tab", "webhooks: ", "sync: ", "retention: "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := helpFrame(t, tc.m)
			for _, want := range tc.want {
				if !strings.Contains(frame, want) {
					t.Errorf("help over %s does not name %q:\n%s", tc.name, want, frame)
				}
			}
		})
	}
}

func TestTheServerViewCursorStaysOnItsRowsAndEscLeaves(t *testing.T) {
	svc := serverFake()
	svc.conns.Connections = append(svc.conns.Connections,
		core.Connection{ID: "c-2", Surface: core.ConnectionSSH, ActorHandle: "bob", Remote: "192.0.2.8"},
		core.Connection{ID: "c-3", Surface: core.ConnectionSSH, ActorHandle: "cy", Remote: "192.0.2.9"})
	m := serverModel(t, svc)
	rows := len(m.serverRows())
	if rows < 2 {
		t.Fatalf("the fake needs two connections for this test, has %d", rows)
	}
	for _, step := range []struct {
		key  string
		want int
	}{
		{"G", rows - 1}, {"j", rows - 1}, {"k", rows - 2}, {"g", 0}, {"k", 0}, {"j", 1},
	} {
		m, _ = m.reduce(pressKey(step.key))
		if m.server.sel != step.want {
			t.Fatalf("after %s the cursor is on row %d, want %d", step.key, m.server.sel, step.want)
		}
	}
	m, _ = m.reduce(pressKey("esc"))
	if m.view == viewServer {
		t.Error("esc did not leave the server view")
	}
}
