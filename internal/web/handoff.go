// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/handoff"
	"github.com/heliopsy/tix/internal/output"
)

// HandoffDialog is the id of the task screen's hand to an agent dialog.
const HandoffDialog = "handoff"

// handoffView is the hand to an agent dialog: the prompt for an agent with
// the tix CLI, the one for an agent with the MCP tools, and each client's
// way in. The server is the address this request reached, so it is the one
// the reader can already see works.
type handoffView struct {
	Server   string
	Prompt   string
	MCP      string
	Snippets []handoff.Snippet
}

// newHandoff builds the dialog for one task as this request reached it.
func newHandoff(r *http.Request, t core.Task) handoffView {
	task := handoff.Task{
		Ref:    output.ShowRef(t.Ref),
		Title:  t.Title,
		Server: baseURL(r),
	}
	return handoffView{
		Server:   task.Server,
		Prompt:   handoff.Prompt(task),
		MCP:      handoff.MCPPrompt(task),
		Snippets: handoff.For(task),
	}
}
