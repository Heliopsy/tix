// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// serverFake answers the server view with one installation, two servers and
// one connection.
func serverFake() *adminFake {
	svc := newAdminFake()
	seen := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	held := 3
	svc.status = &core.StatusReport{
		Installation: core.Installation{Version: "v9.9.9", SchemaVersion: 42, Engine: "sqlite", ObservedAt: seen},
		Servers: []core.ServerStatus{
			{ID: "s1", Address: "10.0.0.5:8080", Version: "v9.9.9", Attached: true,
				Surfaces: []core.ServerSurface{core.ServerSurfaceAPI, core.ServerSurfaceSSH},
				Uptime:   core.Duration(time.Hour), Connections: &held},
			{ID: "s2", Address: "10.0.0.6:8080", Version: "v9.9.8", LastSeenAt: seen},
		},
		AnsweringServer: "s1",
		Work:            core.Work{Tenants: 1, Projects: 2, Tasks: 17},
	}
	svc.conns = &core.ConnectionList{ServerID: "s1",
		Connections: []core.Connection{{ID: "c-1", Surface: core.ConnectionSSH, ActorHandle: "ada",
			Remote: "192.0.2.7", Since: seen}},
		Counts: core.ConnectionCounts{SSH: 1, Tenant: 1, Process: 3}}
	return svc
}

// serverModel opens the server view the way a reader does.
func serverModel(t *testing.T, svc *adminFake) Model {
	t.Helper()
	m := boardModel(t)
	m.svc = svc
	m, cmd := m.reduce(pressKey("%"))
	m = runCmd(t, m, cmd)
	if m.view != viewServer {
		t.Fatalf("%% did not open the server view; view = %v", m.view)
	}
	return m
}

func TestTheServerViewStatesTheStatusAndTheConnections(t *testing.T) {
	m := serverModel(t, serverFake())
	frame := m.Frame()

	if got := adminRow(t, frame, "installation:", "version"); !strings.Contains(got, "v9.9.9") {
		t.Errorf("the version row says %q", got)
	}
	servers := adminSection(t, frame, "servers (2, 1 attached):")
	for _, want := range []string{"10.0.0.5:8080", "attached, up 1h0m0s", "3 connections", "(answering)",
		"10.0.0.6:8080", "stale since"} {
		if !strings.Contains(servers, want) {
			t.Errorf("the servers section does not say %q:\n%s", want, servers)
		}
	}
	conns := adminSection(t, frame, "connections (1):")
	for _, want := range []string{"ssh", "@ada", "192.0.2.7", "3 across all tenants"} {
		if !strings.Contains(conns, want) {
			t.Errorf("the connections section does not say %q:\n%s", want, conns)
		}
	}
}

func TestARefusedStatusStillShowsTheConnections(t *testing.T) {
	svc := serverFake()
	svc.status, svc.statusErr = nil, errors.New("forbidden: no")
	frame := serverModel(t, svc).Frame()

	if got := adminSection(t, frame, "installation:"); !strings.Contains(got, "not shown: forbidden: no") {
		t.Errorf("the refused status does not say why:\n%s", got)
	}
	if got := adminSection(t, frame, "connections (1):"); !strings.Contains(got, "@ada") {
		t.Errorf("a refused status cost the connections:\n%s", got)
	}
}

func TestEndingAConnectionNamesItAndWaitsForAgreement(t *testing.T) {
	svc := serverFake()
	m := serverModel(t, svc)

	m, _ = m.reduce(pressKey("X"))
	if !m.confirm.Open() {
		t.Fatal("X opened no confirmation")
	}
	q := m.confirm.Question()
	for _, want := range []string{"end connection ssh @ada from 192.0.2.7", "may reconnect"} {
		if !strings.Contains(q, want) {
			t.Errorf("the question %q does not say %q", q, want)
		}
	}
	if len(svc.ended) != 0 {
		t.Fatal("the connection was ended before agreement")
	}
	m, cmd := m.reduce(pressKey("y"))
	m, cmd = m.reduce(cmd())
	if len(svc.ended) != 1 || svc.ended[0] != "c-1" {
		t.Fatalf("ended %v, want the selected connection", svc.ended)
	}
	if cmd == nil {
		t.Fatal("ending a connection did not read the view again")
	}
	if !strings.Contains(m.status, "ended ssh @ada") {
		t.Errorf("status = %q", m.status)
	}
}

func TestAServerWithNoConnectionRefusesTheEndKey(t *testing.T) {
	svc := serverFake()
	svc.conns.Connections = nil
	m := serverModel(t, svc)

	m, _ = m.reduce(pressKey("X"))
	if m.confirm.Open() {
		t.Fatal("a confirmation opened over no connection")
	}
	if !strings.Contains(m.err, "no connection") {
		t.Errorf("err = %q", m.err)
	}
	for _, e := range m.keys.ShortHelp(viewServer, m.actionContext()) {
		if e.Desc == "end the selected connection" {
			t.Error("the footer offers to end a connection that is not there")
		}
	}
}

func TestAReaderWhoMayNotReadStatusIsNotOfferedTheServerView(t *testing.T) {
	actor := &core.Actor{ID: "r", TenantID: "t", Role: core.RoleViewer}
	svc := serverFake()
	m := New(Config{Actor: actor, Access: capability.TUIAccess(actor), Service: svc})
	if m.canReach(viewServer) {
		t.Fatal("a viewer is offered the server view")
	}
	m, cmd := m.reduce(pressKey("%"))
	if m.view == viewServer || cmd != nil {
		t.Fatalf("%% entered view %v or asked for a read", m.view)
	}
	if svc.statusN != 0 {
		t.Fatal("the status was read for a reader not offered it")
	}
}
