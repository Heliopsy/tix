// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// No screen that draws a task can be reached without task:read, so the star
// button's absence is checked where it is decided: the partial, rendered for
// a reader who holds everything except the scope starring needs.
func TestTheStarButtonIsDrawnOnlyForAReaderWhoMayStar(t *testing.T) {
	t.Parallel()
	tmpl := Handler(nil).(*handler).templates["tasks.html"]
	task := core.Task{Ref: "infra-1", ProjectID: "p", Status: "todo", Priority: core.PriorityNormal}

	cases := []struct {
		name  string
		actor *core.Actor
		want  bool
	}{
		{"a reader", &core.Actor{Scopes: []core.Scope{core.ScopeTaskRead}}, true},
		{"everything but task:read", &core.Actor{Scopes: []core.Scope{core.ScopeTaskWrite, core.ScopeUserAdmin}}, false},
		{"nobody", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			v := view{Actor: tc.actor, CSRF: "tok", Data: tasksView{Tasks: []core.Task{task}}}
			if err := tmpl.ExecuteTemplate(&out, "tasks-listing", v); err != nil {
				t.Fatalf("rendering: %v", err)
			}
			row := out.String()
			if !strings.Contains(row, `id="t-infra-1"`) {
				t.Fatalf("the row was not drawn at all:\n%s", row)
			}
			if got := strings.Contains(row, `action="/tasks/infra-1/star">`); got != tc.want {
				t.Errorf("star button drawn = %v, want %v", got, tc.want)
			}
		})
	}
}

// A star set in one tab has to reach the reader's other open screens, which
// it does only if every screen's subscription admits the two star events.
func TestEveryLiveTaskScreenSubscribesToStars(t *testing.T) {
	t.Parallel()
	for _, typ := range []core.EventType{core.EventTaskStarred, core.EventTaskUnstarred} {
		e := core.Event{Type: typ, ProjectID: "p", SubjectID: "t"}
		for name, types := range map[string][]core.EventType{"list and board": liveTaskTypes, "task screen": liveDetailTypes} {
			if !(core.EventFilter{ProjectIDs: []string{"p"}, Types: types}).Matches(e) {
				t.Errorf("the %s subscription drops %s", name, typ)
			}
		}
	}
}
