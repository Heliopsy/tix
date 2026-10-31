// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// undoRoute offers the way back only where the workflow has one, so the toast
// never offers an Undo the service would refuse.
func TestUndoRouteFollowsTheWorkflow(t *testing.T) {
	t.Parallel()
	oneWay := core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo"}, {Key: "doing"}, {Key: "review"}, {Key: "done", Terminal: true},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "doing"}, {From: "doing", To: "review"}, {From: "review", To: "todo"},
			{From: "review", To: "done"},
		},
	}
	cases := []struct {
		name, from, to, want string
	}{
		{"a direct way back", "review", "todo", "todo"},
		{"a way back through another state", "review", "doing", "todo>doing"},
		{"no way back at all", "done", "review", ""},
		{"nothing moved", "todo", "todo", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := undoRoute(oneWay, tc.from, tc.to); got != tc.want {
				t.Errorf("undoRoute(%s -> %s) = %q, want %q", tc.from, tc.to, got, tc.want)
			}
		})
	}
}
