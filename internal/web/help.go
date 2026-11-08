// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// helpDoc is the documentation section a screen's help button opens: a file
// under docs/ and the GitHub anchor of a heading in it.
type helpDoc struct {
	Page    string
	Section string
}

// helpDocs maps every screen's route to the section that explains it. A test
// renders each screen in scripts/screenshots/routes.json and fails on one
// missing here, and another fails on a page or heading docs/ does not have.
var helpDocs = map[string]helpDoc{
	RouteLogin:                           {"web-ui.md", "signing-in"},
	RouteSettings:                        {"web-ui.md", "the-settings-screen"},
	"/settings/notifications":            {"web-ui.md", "notification-channels"},
	"/settings/notifications/{id}":       {"web-ui.md", "notification-channels"},
	RouteRelease:                         {"web-ui.md", "the-release-screen"},
	"/activity":                          {"web-ui.md", "activity"},
	"/stats":                             {"statistics.md", "what-each-figure-means"},
	"/admin/status":                      {"web-ui.md", "server-administration"},
	"/projects":                          {"web-ui.md", "projects"},
	"/projects/{key}":                    {"web-ui.md", "the-project-page"},
	"/projects/{key}/settings":           {"web-ui.md", "project-settings"},
	"/projects/{key}/delete":             {"web-ui.md", "deleting-a-project"},
	"/projects/{key}/fields":             {"workflows.md", "custom-fields"},
	"/projects/{key}/import":             {"web-ui.md", "importing-into-a-project"},
	"/projects/{key}/import/{id}/{step}": {"web-ui.md", "importing-into-a-project"},
	"/filters/{filter}":                  {"web-ui.md", "my-work-saved-filters-and-recently-viewed"},
	"/mine":                              {"web-ui.md", "my-work-saved-filters-and-recently-viewed"},
	"/inbox":                             {"web-ui.md", "the-inbox"},
	"/workflows":                         {"workflows.md", "defining-a-workflow"},
	"/workflows/{key}":                   {"workflows.md", "defining-a-workflow"},
	"/templates":                         {"web-ui.md", "task-templates"},
	"/templates/new":                     {"web-ui.md", "task-templates"},
	"/templates/{id}":                    {"web-ui.md", "task-templates"},
	"/tasks/{ref}/template":              {"web-ui.md", "task-templates"},
	"/tasks":                             {"web-ui.md", "the-task-list"},
	"/tasks/{ref}":                       {"web-ui.md", "the-task-screen"},
	"/tasks/{ref}/copy":                  {"web-ui.md", "copying-a-task"},
	"/actors":                            {"web-ui.md", "the-directory"},
	"/actors/{id}":                       {"web-ui.md", "the-actor-screen"},
	"/tasks/{ref}/tree":                  {"web-ui.md", "the-task-screen"},
	"/admin/tenant":                      {"tenancy.md", "tenants"},
	"/admin/domains":                     {"tenancy.md", "domains"},
	"/admin/users":                       {"tenancy.md", "members-and-roles"},
	"/admin/tokens":                      {"agents.md", "tokens-and-scopes"},
	"/admin/ssh-keys":                    {"deployment.md", "enrolling-a-key"},
	"/admin/global":                      {"tenancy.md", "global-admins"},
	"/admin/global/tenants/{key}":        {"tenancy.md", "global-admins"},
	"/admin/global/accounts/{email}":     {"tenancy.md", "account-history"},
	"/account":                           {"tenancy.md", "account-history"},
	RouteSwitch:                          {"tenancy.md", "switching-tenant"},
	"/admin/connections":                 {"web-ui.md", "server-administration"},
	"/admin/webhooks":                    {"api.md", "webhooks"},
	"/transfer":                          {"migrating.md", "moving-a-whole-tenant"},
	"/sync":                              {"migrating.md", "importing-from-an-external-tracker"},
	"/sync/runs":                         {"migrating.md", "importing-from-an-external-tracker"},
	"/bundles":                           {"migrating.md", "sharing-configuration-between-installations"},
	RouteConnect:                         {"web-ui.md", "connect-an-agent"},
}

// href is the help link: the documentation redirect with the section to land
// on, so the reader reaches the heading in the release they run.
func (d helpDoc) href() string {
	return RouteDocs + "/" + d.Page + "?" + url.Values{docsSectionParam: {d.Section}}.Encode()
}

// docsSectionParam names the heading /docs/PAGE lands on.
const docsSectionParam = "section"

// docsSection is the shape of a GitHub heading anchor, which is all the
// redirect passes on.
var docsSection = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,127}$`)

// helpFor is the help link of the screen a request draws, or "" for one with
// none. A refused form draws its screen from a POST, so the screen is found by
// the GET route its path names.
func (h *handler) helpFor(r *http.Request) string {
	if h.mux == nil {
		return ""
	}
	probe := &http.Request{Method: http.MethodGet, URL: &url.URL{Path: r.URL.Path}, Host: r.Host}
	_, pattern := h.mux.Handler(probe)
	_, route, _ := strings.Cut(pattern, " ")
	d, ok := helpDocs[route]
	if !ok {
		return ""
	}
	return d.href()
}
