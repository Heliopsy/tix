package integration

import (
	"io"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/tui"
)

// driveTUI runs a tea.Model's commands synchronously, with no terminal and no
// bubbletea runtime loop, which is the cheapest honest way to exercise a
// bubbletea program from a test: its Update function is pure, and Init and
// every handler return plain tea.Cmd values (func() tea.Msg) rather than
// touching a terminal directly.
//
// It deliberately does not chase every command to its end: a subscription's
// "wait for the next event" command and its reconnect timer return a command
// that blocks on a live channel or a timer by design, and a synchronous pump
// calling them would hang forever. Those two message kinds are applied to
// Update, so the model still reaches its normal "connected" state, but
// whatever command they return is left unrun. Every other command is chased,
// bounded by a step budget so a genuine cycle fails the test instead of
// hanging it.
func driveTUI(t *testing.T, m tea.Model, initial tea.Cmd) tea.Model {
	t.Helper()
	queue := []tea.Cmd{initial}
	budget := 200
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		budget--
		if budget < 0 {
			t.Fatal("the tui command pump exceeded its step budget; a command likely never settles")
		}
		msg := c()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		var next tea.Cmd
		m, next = m.Update(msg)
		switch reflect.TypeOf(msg).Name() {
		case "streamMsg", "reconnectMsg", "eventMsg":
			// These watch a live subscription or a timer; chasing what they
			// return is exactly the hang this pump exists to avoid.
		default:
			queue = append(queue, next)
		}
	}
	return m
}

// pressKey feeds one key press through Update and drains the commands it
// produces the same way driveTUI does.
func pressKey(t *testing.T, m tea.Model, key tea.KeyType) tea.Model {
	t.Helper()
	m, cmd := m.Update(tea.KeyMsg{Type: key})
	return driveTUI(t, m, cmd)
}

// tuiScenario is one row of the terminal-interface transport-equivalence
// table: seed data through tg.svc directly, then drive the real Model's
// Init and Update against the SAME target and assert what it renders.
type tuiScenario struct {
	name   string
	exempt string
	// seed writes whatever the scenario needs the interface to display, and
	// returns the text the rendered view must contain once driven.
	seed func(t *testing.T, tg target) (project core.Project, want string)
	// openBoard, when true, presses Down once (past the harness's one
	// starter project, "default", at position 0) then Enter, to open the
	// project seed just created at position 1. When false, the assertion is
	// made against the project picker itself.
	openBoard bool
}

var tuiScenarios = []tuiScenario{
	{
		name: "the project picker lists a project written through this target",
		seed: func(t *testing.T, tg target) (core.Project, string) {
			p, err := tg.svc.CreateProject(tg.ctx, core.CreateProjectInput{
				Key: "m" + randSuffix(), Name: "Widgets " + randSuffix(),
			})
			if err != nil {
				t.Fatalf("[%s] creating a project: %v", tg.name, err)
			}
			return *p, p.Name
		},
	},
	{
		name:      "opening a project's board shows a task written through this target",
		openBoard: true,
		seed: func(t *testing.T, tg target) (core.Project, string) {
			p, err := tg.svc.CreateProject(tg.ctx, core.CreateProjectInput{Key: "m" + randSuffix(), Name: "Board"})
			if err != nil {
				t.Fatalf("[%s] creating a project: %v", tg.name, err)
			}
			task, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: p.Key, Title: "seen on the board"})
			if err != nil {
				t.Fatalf("[%s] creating a task: %v", tg.name, err)
			}
			// The board column is narrow enough to truncate a title before
			// its distinguishing suffix would show; the task's ref is always
			// rendered in full and is exactly as much proof that this
			// specific write reached the board.
			return *p, task.Ref
		},
	},
}

// runTUIScenario boots the real Model against tg, seeds through the given
// func, drives Init and, for a board scenario, the key presses that open the
// seeded project, and returns the rendered view plus the text it must
// contain.
func runTUIScenario(t *testing.T, tg target, sc tuiScenario) (view, want string) {
	t.Helper()
	_, want = sc.seed(t, tg)

	model := tui.New(tui.Config{
		Service: tg.svc, Context: tg.ctx,
		Actor: &core.Actor{ID: "tui-driver", TenantID: "n/a", Kind: core.ActorUser, Handle: "tui", Role: core.RoleAdmin},
		Out:   io.Discard,
	})
	var m tea.Model = model
	m = driveTUI(t, m, m.Init())
	if sc.openBoard {
		// The harness seeds exactly one starter project ("default") at
		// position 0; every scenario's own project is created afterward and
		// sorts after it, landing at position 1.
		m = pressKey(t, m, tea.KeyDown)
		m = pressKey(t, m, tea.KeyEnter)
	}
	return m.View(), want
}

func TestTUITransportEquivalence(t *testing.T) {
	for _, sc := range tuiScenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			if sc.exempt != "" {
				t.Skipf("exempt: %s", sc.exempt)
			}

			localH := newMatrixHarness(t)
			localView, localWant := runTUIScenario(t, localH.localTarget(), sc)
			if !containsText(localView, localWant) {
				t.Fatalf("direct service.Local: rendered view does not contain %q:\n%s", localWant, localView)
			}

			remoteH := newMatrixHarness(t)
			remoteView, remoteWant := runTUIScenario(t, remoteH.remoteTarget(), sc)
			if !containsText(remoteView, remoteWant) {
				t.Fatalf("client.Client over HTTP: rendered view does not contain %q:\n%s", remoteWant, remoteView)
			}

			// The two rendered views are never byte-identical: each target
			// seeded its own randomly-suffixed project and task, precisely
			// so a run of this table can never see another run's leftovers.
			// What must be identical is that BOTH transports, independently,
			// render what was written through them; that is asserted above.
		})
	}
}

func containsText(haystack, needle string) bool {
	return needle != "" && strings.Contains(haystack, needle)
}
