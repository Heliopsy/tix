package web

import (
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

// putWorkflow saves the edited state machine.
func (h *handler) putWorkflow(w http.ResponseWriter, r *http.Request) error {
	in := core.WorkflowInput{
		Key:  field(r, "key"),
		Name: field(r, "name"),
		Definition: core.WorkflowDefinition{
			Initial:     field(r, "initial"),
			States:      parseStates(field(r, "states")),
			Transitions: parseTransitions(field(r, "transitions")),
		},
		Migrate: parseMigrations(field(r, "migrate")),
	}
	wf, err := h.svc.PutWorkflow(r.Context(), in)
	if err != nil {
		return err
	}
	redirect(w, r, RouteWorkflows+"/"+wf.Key, "workflow saved")
	return nil
}

// parseStates reads the editor's state lines.
func parseStates(raw string) []core.State {
	var out []core.State
	for _, line := range lines(raw) {
		parts := strings.Split(line, "|")
		state := core.State{Key: strings.TrimSpace(parts[0])}
		state.Label = state.Key
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			state.Label = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 && strings.EqualFold(strings.TrimSpace(parts[2]), "terminal") {
			state.Terminal = true
		}
		out = append(out, state)
	}
	return out
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
