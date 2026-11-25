// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/tasklink"
)

// shareHelp is the section the Share menu's (?) opens.
var shareHelp = helpDoc{"web-ui.md", "sharing-a-task"}

// shareView is the task screen's Share menu (templates/share.html). Path is
// what assets/share.js widens against the page's own origin; URL is the same
// address drawn from the request, shown as a plain link for a reader without
// scripting. Neither names a tenant: the interface takes the tenant from the
// session and refuses one named in the request (rejectTenantOverride).
type shareView struct {
	ID    string
	Ref   string
	Title string
	Path  string
	URL   string
	Help  string
}

// newShare builds the Share menu for one task as this request reached it.
func newShare(r *http.Request, t core.Task) shareView {
	id := "share-" + t.Ref
	if dialogRequest(r) {
		id = "dlg-" + id
	}
	return shareView{
		ID:    id,
		Ref:   output.ShowRef(t.Ref),
		Title: t.Title,
		Path:  tasklink.Path(t.Ref),
		URL:   tasklink.URL(baseURL(r), t.Ref),
		Help:  shareHelp.href(),
	}
}

func init() {
	jsMessages = append(jsMessages,
		"Link copied", "Markdown link copied", "Key and title copied")
}
