// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// RouteTaskTree answers one task's subtree as the fragment the task screen
// swaps in where a node offers to show its subtasks.
const RouteTaskTree = "/tasks/{ref}/tree"

// treeDrawDepth is how many levels of subtasks the task screen draws at
// once. TaskTree is asked for one level more, which is never drawn: it only
// says which nodes on the last drawn level have children of their own.
const treeDrawDepth = 3

// subtaskNode is one task in the drawn subtask tree. More counts the children
// it has that were not drawn, which is what its Show subtasks control offers.
type subtaskNode struct {
	Task     core.Task
	Children []subtaskNode
	More     int
	Level    int
}

// subtaskTree is the drawn tree under one task. Shallow reports a subtree too
// large to walk, drawn one level deep instead.
type subtaskTree struct {
	Root    string
	Nodes   []subtaskNode
	Count   int
	Shallow bool
}

// loadSubtaskTree walks a task's subtree treeDrawDepth levels deep. A subtree
// the service refuses as too large to walk falls back to direct subtasks,
// which is what the screen drew before it drew a tree.
func (h *handler) loadSubtaskTree(r *http.Request, ref core.TaskRef) (core.Task, subtaskTree, error) {
	flat, err := h.svc.TaskTree(r.Context(), ref, treeDrawDepth+1)
	shallow := false
	if core.IsKind(err, core.KindPrecondition) {
		flat, err = h.svc.TaskTree(r.Context(), ref, 1)
		shallow = true
	}
	if err != nil {
		return core.Task{}, subtaskTree{}, err
	}
	if len(flat) == 0 {
		return core.Task{}, subtaskTree{}, core.NotFound("task %q has no tree", ref.String())
	}
	depth := treeDrawDepth
	if shallow {
		depth = 1
	}
	tree := buildSubtaskTree(flat, depth)
	tree.Shallow = shallow
	return flat[0], tree, nil
}

// subtaskList is one drawn level of the tree with the id its list carries,
// which a fold button's aria-controls names.
type subtaskList struct {
	ID    string
	Nodes []subtaskNode
}

// List is the tree's top level under an id.
func (t subtaskTree) List(id string) subtaskList { return subtaskList{ID: id, Nodes: t.Nodes} }

// Sub is the level under a node, under an id.
func (n subtaskNode) Sub(id string) subtaskList { return subtaskList{ID: id, Nodes: n.Children} }

// subtreeFragment is what the subtree route renders: the tree and the id the
// page asked its list to carry.
type subtreeFragment struct {
	Tree subtaskTree
	ID   string
}

// subtreeID is the shape of an id the page may ask the fragment to carry: the
// one the node's own fold button already names.
var subtreeID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,200}$`)

// buildSubtaskTree folds TaskTree's pre-order list, root first, into nodes
// drawn depth levels deep. Children keep the order the service gave them.
func buildSubtaskTree(flat []core.Task, depth int) subtaskTree {
	if len(flat) == 0 {
		return subtaskTree{}
	}
	kids := map[string][]core.Task{}
	for _, t := range flat[1:] {
		kids[t.ParentID] = append(kids[t.ParentID], t)
	}
	out := subtaskTree{Root: flat[0].Ref}
	var grow func(parent string, level int) []subtaskNode
	grow = func(parent string, level int) []subtaskNode {
		var nodes []subtaskNode
		for _, t := range kids[parent] {
			out.Count++
			node := subtaskNode{Task: t, Level: level}
			if level < depth {
				node.Children = grow(t.ID, level+1)
			} else {
				node.More = len(kids[t.ID])
			}
			nodes = append(nodes, node)
		}
		return nodes
	}
	out.Nodes = grow(flat[0].ID, 1)
	return out
}

// showTaskTree answers one task's subtree as the list a node's Show subtasks
// control swaps in. It renders the task screen's own block, so a loaded level
// reads exactly as one drawn on load.
func (h *handler) showTaskTree(w http.ResponseWriter, r *http.Request) error {
	ref, err := core.ParseTaskRef(r.PathValue("ref"))
	if err != nil {
		return err
	}
	_, tree, err := h.loadSubtaskTree(r, ref)
	if err != nil {
		return err
	}
	id := r.URL.Query().Get("id")
	if !subtreeID.MatchString(id) {
		id = "subtree-" + strings.ToLower(ref.String())
	}
	v := h.newView(r, "task.html", "Subtasks", subtreeFragment{Tree: tree, ID: id})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	return h.templatesFor(r)["task.html"].ExecuteTemplate(w, "subtask-fragment", v)
}
