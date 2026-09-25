// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// seedAgent creates an agent actor, which holds no user. An agent is what a
// picker built from the user list would leave out, and agents are what most
// of the work in this product is assigned to.
func seedAgent(t *testing.T, f *fixture, tenantID, handle string) core.Actor {
	t.Helper()
	ctx := context.Background()
	actor := core.Actor{Kind: core.ActorAgent, Handle: handle}
	if err := f.store.Update(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		return tx.CreateActor(ctx, &actor)
	}); err != nil {
		t.Fatalf("creating agent %q: %v", handle, err)
	}
	actor.TenantID = tenantID
	return actor
}

// The assignee field suggests the handles this tenant holds without becoming
// a closed list: a datalist decorates a free text field, so an actor of
// another tenant stays assignable by identifier.
func TestTaskScreenSuggestsActorHandles(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedAgent(t, f, f.tenantA.ID, "mint")
	seedAgent(t, f, f.tenantB.ID, "intruder")

	b := f.as("alice")
	page := b.page("/tasks/" + b.createTask("infra", "needs an owner"))

	cases := []struct {
		name string
		want string
	}{
		{"the field is a free text input", `name="assignee"`},
		{"the field names the list", `list="actor-handles"`},
		{"the list exists", `<datalist id="actor-handles">`},
		{"a person is offered", `<option value="alice">`},
		{"an agent is offered", `<option value="mint">`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(page, tc.want) {
				t.Errorf("the task screen is missing %s:\n%s", tc.want, page)
			}
		})
	}
	if strings.Contains(page, `<option value="intruder">`) {
		t.Error("the picker offered an actor of another tenant")
	}
	if strings.Contains(page, `<select id="assignee"`) {
		t.Error("the assignee became a closed list, so an actor of another tenant is no longer assignable")
	}
}

// The directory screen is the same listing, shown whole.
func TestDirectoryScreenListsPeopleAndAgents(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedAgent(t, f, f.tenantA.ID, "mint")
	seedAgent(t, f, f.tenantB.ID, "intruder")

	page := f.as("alice").page("/actors")
	for _, want := range []string{"alice", "mint", "agent", "user"} {
		if !strings.Contains(page, want) {
			t.Errorf("the directory does not show %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "intruder") {
		t.Error("the directory disclosed an actor of another tenant")
	}
}
