// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// accountLogView is one page of an account's log. Email names the account
// when a global admin reads somebody else's; it is empty for the reader's own.
type accountLogView struct {
	Email   string
	Entries []core.AccountLogEntry
	Pager   pager
}

// showAccountLog draws the reader's own account log.
func (h *handler) showAccountLog(w http.ResponseWriter, r *http.Request) error {
	entries, next, err := h.svc.GetAccountLog(r.Context(), core.Page{
		Cursor: r.URL.Query().Get(CursorParam), Limit: h.rowsPerPage(r), Direction: core.Descending,
	})
	if err != nil {
		return err
	}
	return h.render(w, r, accountLogTemplate, "Your account", accountLogView{
		Entries: entries, Pager: newPager(r, RouteAccount, next, len(entries), "changes", SizeParam),
	})
}

// showGlobalAccountLog draws any account's log for a global admin.
func (h *handler) showGlobalAccountLog(w http.ResponseWriter, r *http.Request) error {
	email := strings.TrimSpace(r.PathValue("email"))
	entries, next, err := h.svc.GetAccountLogFor(r.Context(), email, core.Page{
		Cursor: r.URL.Query().Get(CursorParam), Limit: h.rowsPerPage(r), Direction: core.Descending,
	})
	if err != nil {
		return err
	}
	return h.render(w, r, accountLogTemplate, "An account's history", accountLogView{
		Email: email, Entries: entries,
		Pager: newPager(r, globalAccountPath(email), next, len(entries), "changes", SizeParam),
	})
}

// globalAccountPath is the address of an account's log in the Global admin area.
func globalAccountPath(email string) string {
	return strings.Replace(RouteGlobalAccount, "{email}", url.PathEscape(email), 1)
}

// accountLogTemplate draws both account log screens.
const accountLogTemplate = "accountlog.html"
