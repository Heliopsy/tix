// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func headsOf(keys ...string) []groupHead {
	out := make([]groupHead, len(keys))
	for i, k := range keys {
		out[i] = groupHead{Key: k, Name: strings.ToUpper(k)}
	}
	return out
}

func headPtr(h *groupHead) *groupHead { return h }

func TestArrangeGroupsOrdersHidesAndDecidesMoves(t *testing.T) {
	cases := []struct {
		name   string
		prefs  []core.GroupPref
		drawn  string
		hidden string
		left   string
		right  string
	}{
		{"untouched", nil, "a,b,c,d", "", "b,c,d", "a,b,c"},
		{"a star leads", []core.GroupPref{{Group: "c", Starred: true}}, "c,a,b,d", "", "b,d", "a,b"},
		{"positions before the rest", []core.GroupPref{{Group: "d", Position: 1}, {Group: "b", Position: 2}}, "d,b,a,c", "", "b,a,c", "d,b,a"},
		{"stars keep their positions", []core.GroupPref{{Group: "d", Starred: true, Position: 2}, {Group: "c", Starred: true, Position: 1}}, "c,d,a,b", "", "d,b", "c,a"},
		{"hidden is not drawn", []core.GroupPref{{Group: "B", Hidden: true}}, "a,c,d", "b", "c,d", "a,c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			drawn, hidden := arrangeGroups(headsOf("a", "b", "c", "d"), headPtr, groupPrefIndex(tc.prefs))
			var keys, left, right, gone []string
			for _, h := range drawn {
				keys = append(keys, h.Key)
				if h.Left {
					left = append(left, h.Key)
				}
				if h.Right {
					right = append(right, h.Key)
				}
			}
			for _, h := range hidden {
				gone = append(gone, h.Key)
			}
			for what, pair := range map[string][2]string{
				"drawn": {strings.Join(keys, ","), tc.drawn}, "hidden": {strings.Join(gone, ","), tc.hidden},
				"left": {strings.Join(left, ","), tc.left}, "right": {strings.Join(right, ","), tc.right},
			} {
				if pair[0] != pair[1] {
					t.Errorf("%s = %q, want %q", what, pair[0], pair[1])
				}
			}
		})
	}
}

func TestMovedOrderSwapsWithTheNeighbour(t *testing.T) {
	order := []string{"a", "b", "c"}
	for _, tc := range []struct {
		group, dir, want string
		bad              bool
	}{
		{"b", "left", "b,a,c", false},
		{"b", "right", "a,c,b", false},
		{"A", "right", "b,a,c", false},
		{"a", "left", "", true},
		{"c", "right", "", true},
		{"z", "left", "", true},
		{"b", "up", "", true},
	} {
		got, err := movedOrder(order, tc.group, tc.dir)
		if tc.bad != (err != nil) || !tc.bad && strings.Join(got, ",") != tc.want {
			t.Errorf("move %s %s = %v, %v; want %q", tc.group, tc.dir, got, err, tc.want)
		}
	}
	if !slices.Equal(order, []string{"a", "b", "c"}) {
		t.Error("movedOrder changed the order it was given")
	}
}

func TestListGroupsByStatusFollowTheWorkflow(t *testing.T) {
	def := core.WorkflowDefinition{States: []core.State{{Key: "todo"}, {Key: "doing", Label: "Doing"}, {Key: "done"}}}
	p := core.Project{ID: "p1", Key: "infra"}
	tasks := []core.Task{{Ref: "1", Status: "done", ProjectID: "p1"}, {Ref: "2", Status: "todo", ProjectID: "p1"},
		{Ref: "3", Status: "gone", ProjectID: "p1"}, {Ref: "4", Status: "done", ProjectID: "p1"}}
	groups := buildListGroups(ListGroupStatus, []core.Project{p}, tasks, map[string]core.WorkflowDefinition{"p1": def})
	var got []string
	for _, g := range groups {
		refs := make([]string, len(g.Tasks))
		for i, t := range g.Tasks {
			refs[i] = t.Ref
		}
		got = append(got, g.Head.Key+":"+strings.Join(refs, "+"))
		if g.Head.Count != len(g.Tasks) {
			t.Errorf("%s counts %d of %d", g.Head.Key, g.Head.Count, len(g.Tasks))
		}
	}
	if want := "todo:2,done:1+4,gone:3"; strings.Join(got, ",") != want {
		t.Errorf("groups = %v, want %s", got, want)
	}
	if groups[0].Head.Kind != groupStatus || len(groups[0].Head.Projects) != 1 || groups[0].Head.Status != "todo" {
		t.Errorf("a status group cannot add a task in its state: %+v", groups[0].Head)
	}
}
