// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// tenantDirectory is the people and agents an assignee picker offers.
func tenantDirectory() []core.Actor {
	return []core.Actor{
		{ID: "a-ada", Handle: "ada", Kind: core.ActorUser},
		{ID: "a-bot", Handle: "runner", Kind: core.ActorAgent},
		{ID: "01JQZZZZZZZZZZZZZZZZZZZZZZ"},
	}
}

func TestTheDirectoryIsOfferedByHandleAndResolvedByIdentifier(t *testing.T) {
	options := ActorOptions(tenantDirectory())
	if options[0] != unassignedOption {
		t.Fatalf("the list opens on %q rather than on the way to clear an assignment", options[0])
	}
	for _, want := range []string{"ada", "runner"} {
		if !slices.Contains(options, want) {
			t.Errorf("the picker does not offer %q: %v", want, options)
		}
	}
	if slices.Contains(options, "a-ada") {
		t.Errorf("the picker offers an identifier nobody recognises: %v", options)
	}

	if got := ActorIDFor(tenantDirectory(), "runner"); got != "a-bot" {
		t.Errorf("picking runner resolved to %q", got)
	}
	if got := ActorIDFor(tenantDirectory(), unassignedOption); got != "" {
		t.Errorf("clearing an assignment resolved to %q", got)
	}
	// The word for nobody wins over an actor called that. Without the word
	// taking precedence, a tenant holding a handle spelled like the option
	// would have "clear the assignment" quietly assign somebody.
	awkward := append(tenantDirectory(), core.Actor{ID: "a-odd", Handle: unassignedOption})
	if got := ActorIDFor(awkward, unassignedOption); got != "" {
		t.Errorf("an actor handled %q took the meaning of clearing an assignment: %q",
			unassignedOption, got)
	}
	// An actor with no handle is still reachable, under the short identifier
	// the rest of the interface falls back to.
	short := ActorLabel(tenantDirectory()[2])
	if short == "" || ActorIDFor(tenantDirectory(), short) != "01JQZZZZZZZZZZZZZZZZZZZZZZ" {
		t.Errorf("an actor with no handle is unreachable; label = %q", short)
	}
}

// assigneeForm opens the picker the way a reader does: the key, then the read
// the key asked for.
func assigneeForm(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	svc.directory = tenantDirectory()
	m.svc = svc
	m, cmd := m.reduce(pressKey("A"))
	if cmd == nil {
		t.Fatalf("A asked for no directory read; err = %q", m.err)
	}
	m, _ = m.reduce(run(t, cmd))
	if !m.form.Open() {
		t.Fatalf("the directory opened no form; err = %q", m.err)
	}
	return m, svc
}

func TestTheAssigneePickerOffersThePeopleTheTenantHas(t *testing.T) {
	m, _ := assigneeForm(t)
	row := formRow(t, m.View(), "assign", "assignee")
	if !strings.Contains(row, unassignedOption) {
		t.Errorf("the picker opens on %q rather than on the task's current assignee", row)
	}
	if !strings.Contains(row, "ada") && !strings.Contains(row, "of 4") {
		t.Errorf("the picker names none of its alternatives: %q", row)
	}
}

func TestThePickerOpensOnWhoeverHoldsTheTaskNow(t *testing.T) {
	form := AssigneeForm(tenantDirectory(), "a-bot")
	if got := form.Value("assignee"); got != "runner" {
		t.Fatalf("the picker opened on %q for a task assigned to a-bot", got)
	}
	// An assignee the directory no longer holds falls back to nobody rather
	// than to the first name on the list, which would reassign by accident.
	stale := AssigneeForm(tenantDirectory(), "a-gone")
	if got := stale.Value("assignee"); got != unassignedOption {
		t.Fatalf("a stale assignee opened the picker on %q", got)
	}
}

func TestAssigningSendsTheIdentifierBehindTheHandle(t *testing.T) {
	m, svc := assigneeForm(t)
	// One step off "unassigned" onto the first of the tenant's actors.
	m, _ = m.reduce(pressKey("right"))
	picked := m.form.Value("assignee")
	next, cmd := m.reduce(pressKey("enter"))
	if cmd == nil {
		t.Fatalf("applying the picker asked for nothing; err = %q", next.err)
	}
	next.reduce(run(t, cmd))

	if len(svc.updated) != 1 || svc.updated[0].AssigneeActorID == nil {
		t.Fatalf("updated = %+v", svc.updated)
	}
	want := ActorIDFor(tenantDirectory(), picked)
	if got := *svc.updated[0].AssigneeActorID; got != want {
		t.Fatalf("picking %q assigned %q, want %q", picked, got, want)
	}
	if want == "" {
		t.Fatal("the step landed back on nobody, so this proves no resolution")
	}
}

func TestClearingAnAssignmentIsAChoiceOnTheSameList(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	svc.directory = tenantDirectory()
	m.svc = svc
	m, cmd := m.reduce(pressKey("A"))
	m, _ = m.reduce(run(t, cmd))
	if m.form.Value("assignee") != unassignedOption {
		t.Fatalf("the picker did not open on %q", unassignedOption)
	}
	next, cmd := m.reduce(pressKey("enter"))
	next.reduce(run(t, cmd))
	if len(svc.updated) != 1 || svc.updated[0].AssigneeActorID == nil ||
		*svc.updated[0].AssigneeActorID != "" {
		t.Fatalf("choosing nobody sent %+v", svc.updated)
	}
}

func TestAnEmptyDirectorySaysSoRatherThanOpeningAnEmptyPicker(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	m, cmd := m.reduce(pressKey("A"))
	m, _ = m.reduce(run(t, cmd))
	if m.form.Open() {
		t.Fatal("an empty directory opened a picker with nothing to pick")
	}
	if !strings.Contains(m.err, "nobody to assign") {
		t.Errorf("err = %q", m.err)
	}
}

// TestTheAssigneeKeyNeedsTheDirectoryItListsFrom is the "needs" half of the
// gate: a reader who may change a task but may not read the directory is
// offered nothing rather than a picker that cannot be filled.
func TestTheAssigneeKeyNeedsTheDirectoryItListsFrom(t *testing.T) {
	keys := DefaultKeyMap()
	var assign gatedAction
	for _, a := range keys.taskBindings() {
		if a.binding.Help().Key == keys.Assign.Help().Key {
			assign = a
		}
	}
	if !slices.Contains(assign.needs, "ListActors") {
		t.Fatalf("the assignee key does not declare the listing it opens on: %+v", assign)
	}
	if assign.permitted(permitOnly("UpdateTask")) {
		t.Fatal("a reader who may not list actors is offered the picker")
	}
	if !assign.permitted(permitOnly("UpdateTask", "ListActors")) {
		t.Fatal("a reader holding both is refused the picker")
	}
}

func TestEverySchemeAssignsThroughThePicker(t *testing.T) {
	for _, scheme := range Schemes() {
		t.Run(string(scheme), func(t *testing.T) {
			m := boardModel(t)
			svc := newFakeService()
			svc.directory = tenantDirectory()
			m.svc = svc
			m = m.installScheme(string(scheme))
			if m.err != "" {
				t.Fatalf("installing %s: %s", scheme, m.err)
			}
			m, cmd := m.reduce(keyMsgFor(m.keys.Assign.Keys()[0]))
			if cmd == nil {
				t.Fatalf("%s: the assignee key asked for no read", scheme)
			}
			m, _ = m.reduce(run(t, cmd))
			if !m.form.Open() {
				t.Fatalf("%s: no picker opened; err = %q", scheme, m.err)
			}
			m, _ = m.reduce(keyMsgFor(m.keys.Right.Keys()[0]))
			m, cmd = m.reduce(keyMsgFor(m.keys.Accept.Keys()[0]))
			if cmd == nil {
				t.Fatalf("%s: applying the picker asked for nothing", scheme)
			}
			m.reduce(run(t, cmd))
			if len(svc.updated) != 1 {
				t.Fatalf("%s: updated = %+v", scheme, svc.updated)
			}
		})
	}
}

// TestTheRegistryBindsTheDirectoryWhereItIsRead keeps the binding where it
// changes no gate: the listing needs no scope, and binding a scopeless read to
// the board would offer a board to a reader who may read no task on it.
func TestTheRegistryBindsTheDirectoryWhereItIsRead(t *testing.T) {
	op, ok := capability.ByMethod("ListActors")
	if !ok {
		t.Fatal("the registry declares no actor listing")
	}
	if op.TUI != viewName(viewDetail) {
		t.Fatalf("the actor listing is bound to view %q", op.TUI)
	}
	nothing := &core.Actor{ID: "u", TenantID: "t", Kind: core.ActorUser}
	access := capability.TUIAccess(nothing)
	if access[viewName(viewBoard)] {
		t.Fatal("a reader holding no scope is offered the board, so a scopeless read reached it")
	}
	if !access[viewName(viewDetail)] {
		t.Fatal("the detail view stopped being reachable without a scope")
	}
}
