package tui

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thereisnotime/tix/internal/core"
)

func TestExitStatus(t *testing.T) {
	tests := []struct {
		name      string
		model     tea.Model
		err       error
		want      int
		wantPrint bool
	}{
		{"clean quit", Model{}, nil, core.ExitOK, false},
		{"interrupt key", Model{interrupted: true}, nil, ExitInterrupt, false},
		{"killed program", Model{}, tea.ErrProgramKilled, ExitInterrupt, false},
		{"cancelled context", Model{}, context.Canceled, ExitInterrupt, false},
		{"fatal not found", Model{fatal: core.NotFound("no tenant")}, nil, core.ExitNotFound, true},
		{"fatal internal", Model{fatal: core.Internal("store is gone")}, nil, core.ExitError, true},
		{"fatal permission", Model{fatal: core.Forbidden("denied")}, nil, core.ExitPermission, true},
		{"program failure", Model{}, errors.New("no tty"), core.ExitError, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			got := exitStatus(tc.model, tc.err, &buf)
			if got != tc.want {
				t.Fatalf("exitStatus = %d, want %d", got, tc.want)
			}
			if printed := strings.Contains(buf.String(), "error:"); printed != tc.wantPrint {
				t.Fatalf("printed = %v, want %v (%q)", printed, tc.wantPrint, buf.String())
			}
		})
	}
}

func TestExitStatusOfAForeignModel(t *testing.T) {
	var buf bytes.Buffer
	if got := exitStatus(nil, nil, &buf); got != core.ExitOK {
		t.Fatalf("exitStatus = %d", got)
	}
}

func TestViewNeverPanics(t *testing.T) {
	sizes := []struct{ w, h int }{{120, 40}, {60, 24}, {30, 12}, {10, 4}, {0, 0}}
	views := []viewKind{viewProjects, viewBoard, viewDetail, viewHelp}
	for _, size := range sizes {
		for _, v := range views {
			m := boardModel(t)
			m.width, m.height, m.view = size.w, size.h, v
			m, _ = m.reduce(detailMsg{task: task("a", "todo", 1, core.PriorityNormal)})
			m.view = v
			m.err, m.status, m.filterErr = "boom", "ok", "bad filter"
			m.editing, m.choosing, m.choices = true, true, NextStates(testWorkflow(), "todo")
			if m.View() == "" && size.w > 0 {
				t.Fatalf("view %v at %dx%d rendered nothing", v, size.w, size.h)
			}
		}
	}
}

func TestViewWithoutColorEmitsNoEscapeSequences(t *testing.T) {
	m := New(Config{Environ: []string{"NO_COLOR=1"}, Now: func() time.Time { return time.Time{} }})
	m.width, m.height = 120, 40
	m, _ = m.reduce(boardMsg{
		project:  core.Project{ID: "p1", Key: "infra", Name: "Infrastructure"},
		workflow: testWorkflow(),
		tasks:    []core.Task{task("a", "todo", 1, core.PriorityNormal)},
	})
	m.err = "something failed"
	if strings.Contains(m.View(), "\x1b") {
		t.Fatal("NO_COLOR was set and the frame still carried escape sequences")
	}
}

func TestRunStartsAndQuitsCleanly(t *testing.T) {
	svc := newFakeService()
	svc.projects = []core.Project{{ID: "p1", Key: "infra", Name: "Infrastructure"}}
	svc.workflows = []core.Workflow{{ID: "w1", Key: "default", Definition: *testWorkflow()}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var out, errw bytes.Buffer
	code := Run(Options{
		Service: svc, Context: ctx, Environ: []string{"NO_COLOR=1", "TERM=dumb"},
		In: strings.NewReader("q"), Out: &out, Err: &errw,
	})
	if code != core.ExitOK {
		t.Fatalf("exit code = %d, stderr = %q", code, errw.String())
	}
	if errw.Len() != 0 {
		t.Fatalf("stderr = %q", errw.String())
	}
}

func TestCustomInputLeavesTheRealTerminalToBubbletea(t *testing.T) {
	if customInput(nil) {
		t.Fatal("a nil reader was treated as custom input")
	}
	if customInput(os.Stdin) {
		t.Fatal("os.Stdin was treated as custom input, which would skip raw mode")
	}
	if !customInput(strings.NewReader("q")) {
		t.Fatal("a scripted reader was not treated as custom input")
	}
}
