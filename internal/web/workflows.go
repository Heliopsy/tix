// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// workflowRoutes are the workflow list and editor screens.
func (h *handler) workflowRoutes() []route {
	return []route{
		get(RouteWorkflows, "workflows.html", h.showWorkflows, "ListWorkflows"),
		post(RouteWorkflows, h.putWorkflow, "PutWorkflow"),
		get(RouteWorkflow, "workflow.html", h.showWorkflow, "GetWorkflow"),
		post(RouteWorkflowDel, h.deleteWorkflow, "DeleteWorkflow"),
	}
}

// workflowsView is what the workflow list renders.
type workflowsView struct {
	Workflows []core.Workflow
}

// showWorkflows renders every workflow the tenant defines.
func (h *handler) showWorkflows(w http.ResponseWriter, r *http.Request) error {
	workflows, err := h.svc.ListWorkflows(r.Context())
	if err != nil {
		return err
	}
	return h.render(w, r, "workflows.html", "Workflows", workflowsView{Workflows: workflows})
}

// workflowView is what the workflow editor renders.
type workflowView struct {
	Workflow    core.Workflow
	States      string
	Transitions string
}

// showWorkflow renders one workflow's states and transitions for editing.
func (h *handler) showWorkflow(w http.ResponseWriter, r *http.Request) error {
	wf, err := h.svc.GetWorkflow(r.Context(), r.PathValue("key"))
	if err != nil {
		return err
	}
	return h.render(w, r, "workflow.html", "Workflow "+wf.Key, workflowView{
		Workflow:    *wf,
		States:      renderStates(wf.Definition.States),
		Transitions: renderTransitions(wf.Definition.Transitions),
	})
}

// renderStates writes states in the editor's line format.
func renderStates(states []core.State) string {
	out := make([]string, 0, len(states))
	for _, s := range states {
		terminal := "open"
		if s.Terminal {
			terminal = "terminal"
		}
		out = append(out, s.Key+"|"+s.Label+"|"+terminal)
	}
	return strings.Join(out, "\n")
}

// renderTransitions writes transitions in the editor's line format.
func renderTransitions(transitions []core.Transition) string {
	out := make([]string, 0, len(transitions))
	for _, t := range transitions {
		out = append(out, t.From+">"+t.To)
	}
	return strings.Join(out, "\n")
}

// putWorkflow saves the edited state machine, keeping every stored field the
// editor does not render.
func (h *handler) putWorkflow(w http.ResponseWriter, r *http.Request) error {
	states, err := parseStates(field(r, "states"))
	if err != nil {
		return err
	}
	key := strings.ToLower(field(r, "key"))
	migrate := parseMigrations(field(r, "migrate"))
	in := core.WorkflowInput{
		Key:  key,
		Name: field(r, "name"),
		Definition: core.WorkflowDefinition{
			Initial:     field(r, "initial"),
			States:      states,
			Transitions: parseTransitions(field(r, "transitions")),
		},
		Migrate: migrate,
	}
	stored, err := h.storedDefinition(r, key)
	if err != nil {
		return err
	}
	if stored != nil {
		in.Definition = mergeDefinition(*stored, in.Definition, migrate)
	}
	wf, err := h.svc.PutWorkflow(r.Context(), in)
	if err != nil {
		return err
	}
	redirect(w, r, RouteWorkflows+"/"+wf.Key, "workflow saved")
	return nil
}

// storedDefinition returns the definition already held under the key, or nil
// when the editor is defining a workflow that does not exist yet.
func (h *handler) storedDefinition(r *http.Request, key string) (*core.WorkflowDefinition, error) {
	if key == "" {
		return nil, nil
	}
	wf, err := h.svc.GetWorkflow(r.Context(), key)
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &wf.Definition, nil
}

// mergeDefinition keeps what the editor does not show. The form edits state
// keys, labels and terminality, the list of edges and the initial state;
// every other field a state, a transition or the definition carries is read
// back from what is stored rather than defaulted away. A key the form renamed
// is followed through the migration lines, the only place the form says that
// a new key is an old one under another name; a state or a transition the
// form genuinely dropped is genuinely dropped.
func mergeDefinition(stored, edited core.WorkflowDefinition, migrate map[string]string) core.WorkflowDefinition {
	out := edited
	out.DefaultLease = stored.DefaultLease
	renamed := renamedFrom(migrate)
	for i, s := range out.States {
		old, ok := stored.State(s.Key)
		if !ok {
			old, ok = stored.State(renamed[s.Key])
		}
		if !ok {
			continue
		}
		old.Key, old.Label, old.Terminal = s.Key, s.Label, s.Terminal
		if to, moved := migrate[old.RevertTo]; moved {
			old.RevertTo = to
		}
		out.States[i] = old
	}
	for i, t := range out.Transitions {
		old, ok := stored.CanTransition(t.From, t.To)
		if !ok {
			old, ok = stored.CanTransition(formerKey(renamed, t.From), formerKey(renamed, t.To))
		}
		if !ok {
			continue
		}
		old.From, old.To = t.From, t.To
		out.Transitions[i] = old
	}
	return out
}

// renamedFrom inverts the migration lines, so a key in the form can be traced
// back to the stored key it replaces. Two old keys collapsing onto one new key
// leave no single predecessor, so that key carries nothing over.
func renamedFrom(migrate map[string]string) map[string]string {
	out := make(map[string]string, len(migrate))
	for from, to := range migrate {
		if _, clash := out[to]; clash {
			out[to] = ""
			continue
		}
		out[to] = from
	}
	return out
}

// formerKey returns the stored key a form key replaces, or the key itself.
func formerKey(renamed map[string]string, key string) string {
	if was := renamed[key]; was != "" {
		return was
	}
	return key
}

// parseStates reads the editor's state lines.
func parseStates(raw string) ([]core.State, error) {
	var out []core.State
	for _, line := range lines(raw) {
		parts := strings.Split(line, "|")
		state := core.State{Key: strings.TrimSpace(parts[0])}
		state.Label = state.Key
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			state.Label = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			terminal, err := parseTerminal(strings.TrimSpace(parts[2]))
			if err != nil {
				return nil, core.Invalid("state %q: %s", state.Key, err)
			}
			state.Terminal = terminal
		}
		out = append(out, state)
	}
	return out, nil
}

// parseTerminal reads the third field of a state line. An empty field leaves
// the state open; a word naming neither answer is refused rather than read as
// "open", because the reader who wrote it meant something by it.
func parseTerminal(raw string) (bool, error) {
	switch strings.ToLower(raw) {
	case "":
		return false, nil
	case "true", "1", "on", "yes", "terminal":
		return true, nil
	case "false", "0", "off", "no", "open":
		return false, nil
	default:
		return false, fmt.Errorf("%q is neither true nor false", raw)
	}
}

// parseTransitions reads the editor's transition lines.
func parseTransitions(raw string) []core.Transition {
	var out []core.Transition
	for _, line := range lines(raw) {
		from, to, found := strings.Cut(line, ">")
		if !found {
			continue
		}
		out = append(out, core.Transition{
			From: strings.TrimSpace(from), To: strings.TrimSpace(to)})
	}
	return out
}

// parseMigrations reads the editor's state migration lines.
func parseMigrations(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range lines(raw) {
		from, to, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		out[strings.TrimSpace(from)] = strings.TrimSpace(to)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// deleteWorkflow removes a workflow no project uses.
func (h *handler) deleteWorkflow(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteWorkflow(r.Context(), r.PathValue("key")); err != nil {
		return err
	}
	redirect(w, r, RouteWorkflows, "workflow deleted")
	return nil
}
