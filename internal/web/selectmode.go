// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "net/http"

// SelectParam turns on select mode on the task screen: the row boxes and the
// bulk bar are shown. It is how the mode works without scripting, and it is
// carried with the rest of the address so a bulk action returns into it.
const SelectParam = "select"

// selecting reports whether the address asks for select mode.
func selecting(r *http.Request) bool { return r.URL.Query().Get(SelectParam) == "1" }

// selectToggleHref is the Select toggle's link: this screen with select mode
// turned the other way, keeping everything else the address says except what
// a view switch also leaves behind.
func selectToggleHref(r *http.Request, route string) string {
	query := r.URL.Query()
	for _, k := range viewDropped {
		query.Del(k)
	}
	if selecting(r) {
		query.Del(SelectParam)
	} else {
		query.Set(SelectParam, "1")
	}
	if len(query) == 0 {
		return route
	}
	return route + "?" + query.Encode()
}
