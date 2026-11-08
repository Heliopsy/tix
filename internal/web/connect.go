// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/onboard"
)

// Routes of the Connect an agent screen and the documentation redirect.
const (
	RouteConnect      = "/connect"
	RouteConnectToken = "/connect/token" // #nosec G101 -- a url path, not a credential
	RouteDocs         = "/docs"
	RouteDocsPage     = "/docs/{page}"
)

// connectRoutes are the Connect an agent screen, its token form and the
// documentation redirect the menu and the screen link through.
func (h *handler) connectRoutes() []route {
	return []route{
		get(RouteConnect, "connect.html", h.showConnect, "WhoAmI", "GetTenant"),
		post(RouteConnectToken, h.createConnectToken, "CreateToken"),
		public(getFile(RouteDocs, h.redirectDocs)),
		public(getFile(RouteDocsPage, h.redirectDocs)),
	}
}

// tokenSlot marks where a snippet carries the token. The template draws it
// as a span reading $TIX_TOKEN, which assets/connect.js fills in once a token
// has been issued. A private-use character cannot occur in a URL or a key.
const tokenSlot = "TIX_TOKEN" // #nosec G101 -- a placeholder marker, not a credential

// tokenSlotHTML is what each slot is drawn as.
const tokenSlotHTML = `<span class="tokslot" data-token-slot>$TIX_TOKEN</span>` // #nosec G101 -- markup for a placeholder, not a credential

// slotted escapes a snippet and draws its token slots.
func slotted(text string) template.HTML {
	escaped := template.HTMLEscapeString(text)
	// #nosec G203 -- the text is escaped above; only the fixed slot markup is added.
	return template.HTML(strings.ReplaceAll(escaped, template.HTMLEscapeString(tokenSlot), tokenSlotHTML))
}

// connectTab is one tab of a nested tab set: a title, the snippets under it,
// the link that selects it and whether it is the one drawn.
type connectTab struct {
	ID     string
	Title  string
	Titled bool
	// Plain titles are product names, drawn as they are in every language.
	Plain bool
	Snips []onboard.Snippet
	Href  string
	On    bool
}

// connectTabSet is a nested tab strip and the query parameter naming its tab.
type connectTabSet struct {
	Param string
	Label string
	Tabs  []connectTab
}

// On is the selected tab's id.
func (s connectTabSet) On() string {
	for _, tab := range s.Tabs {
		if tab.On {
			return tab.ID
		}
	}
	return ""
}

// connectPane is one of the screen's main tabs: how a reader connects.
type connectPane struct {
	ID    string
	Title string
	Help  string
	Href  string
	On    bool
}

// connectParams are the query parameters the screen reads, the main tab first.
var connectParams = []string{"tab", "install", "client", "skill"}

// connectPanes are the main tabs in order, each with the documentation
// section its help link opens.
var connectPanes = []struct {
	ID, Title string
	Doc       helpDoc
}{
	{"cli", "CLI", helpDoc{"configuration.md", "contexts"}},
	{"mcp", "MCP", helpDoc{"mcp.md", "wiring-it-into-a-client"}},
	{"ssh", "SSH", helpDoc{"deployment.md", "commands-over-ssh"}},
	{"api", "API", helpDoc{"api.md", "authentication"}},
	{"skill", "Skill", helpDoc{"agents.md", "the-shape-of-a-worker"}},
}

// connectQuery is the screen's tab selection as the address names it.
type connectQuery url.Values

// connectQueryOf keeps only the parameters the screen reads.
func connectQueryOf(r *http.Request) connectQuery {
	q, in := url.Values{}, r.URL.Query()
	for _, p := range connectParams {
		if v := in.Get(p); v != "" {
			q.Set(p, v)
		}
	}
	return connectQuery(q)
}

// pick is the tab param names when it is one of ids, else the first.
func (q connectQuery) pick(param string, ids []string) string {
	if v := url.Values(q).Get(param); slices.Contains(ids, v) {
		return v
	}
	return ids[0]
}

// href is the screen's address with param set to id and the rest kept.
func (q connectQuery) href(param, id string) string {
	next := url.Values{}
	for k, v := range q {
		next[k] = slices.Clone(v)
	}
	next.Set(param, id)
	return RouteConnect + "?" + next.Encode()
}

// self is the screen's own address, for a form that comes back to it.
func (q connectQuery) self() string {
	if len(q) == 0 {
		return ""
	}
	return "?" + url.Values(q).Encode()
}

// tabSet marks the selected tab of a nested set and links each one.
func (q connectQuery) tabSet(param, label string, tabs []connectTab) connectTabSet {
	ids := make([]string, len(tabs))
	for i, tab := range tabs {
		ids[i] = tab.ID
	}
	on := q.pick(param, ids)
	for i := range tabs {
		tabs[i].Href, tabs[i].On = q.href(param, tabs[i].ID), tabs[i].ID == on
	}
	return connectTabSet{Param: param, Label: label, Tabs: tabs}
}

// panes are the main tabs with the selected one marked.
func (q connectQuery) panes() ([]connectPane, string) {
	ids := make([]string, len(connectPanes))
	for i, p := range connectPanes {
		ids[i] = p.ID
	}
	on := q.pick("tab", ids)
	out := make([]connectPane, len(connectPanes))
	for i, p := range connectPanes {
		out[i] = connectPane{ID: p.ID, Title: p.Title, Help: p.Doc.href(), Href: q.href("tab", p.ID), On: p.ID == on}
	}
	return out, on
}

// connectView is what the Connect an agent screen renders.
type connectView struct {
	URL      string
	Tenant   string
	Handle   string
	Panes    []connectPane
	Tab      string
	Self     string
	Install  connectTabSet
	Verify   onboard.Snippet
	TokenCmd onboard.Snippet
	Rotate   onboard.Snippet
	Export   onboard.Snippet
	CLI      []onboard.Snippet
	Login    *onboard.Snippet
	Preview  onboard.Snippet
	Clients  connectTabSet
	Skill    connectTabSet
	SSH      []onboard.Snippet
	REST     []onboard.Snippet
	CanIssue bool
	Picker   scopePicker
	Form     tokenForm
	Expiry   []tokenExpiry
	Errors   fieldErrors
	Secret   *oneTimeSecret
}

// Pane is the main tab called id.
func (v connectView) Pane(id string) connectPane {
	for _, p := range v.Panes {
		if p.ID == id {
			return p
		}
	}
	return connectPane{}
}

// sshAddress is where people reach the SSH listener: the configured public
// address, else the host name of the request's origin (the public web address
// where that applies) with the port the listener binds.
func (h *handler) sshAddress(r *http.Request) string {
	if h.sshListen == "" {
		return ""
	}
	if h.sshPublic != "" {
		return h.sshPublic
	}
	host := r.Host
	if u, err := url.Parse(baseURL(r)); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	} else if hn, _, err := net.SplitHostPort(r.Host); err == nil {
		host = hn
	}
	_, port, err := net.SplitHostPort(h.sshListen)
	if err != nil || port == "" {
		return host
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), port)
}

// tenantKey is the reader's tenant key, or "" when they may not read the
// tenant: GetTenant needs tenant:admin. Every command works without it.
func (h *handler) tenantKey(r *http.Request) string {
	tenant, err := h.svc.GetTenant(r.Context(), "")
	if err != nil {
		return ""
	}
	return tenant.Key
}

// baseURL is this deployment's address for links built from the request, the
// same derivation the calendar feed uses: httpapi.Origin.
func baseURL(r *http.Request) string { return httpapi.Origin(r) }

// showConnect renders the screen, with a token just issued here if there is one.
func (h *handler) showConnect(w http.ResponseWriter, r *http.Request) error {
	actor, err := core.RequireActor(r.Context())
	if err != nil {
		return err
	}
	form := tokenForm{Name: onboard.TokenName, Expiry: defaultTokenExpiry,
		Scopes: heldScopes(actor, onboard.AgentScopes())}
	return h.renderConnect(w, r, http.StatusOK, form, nil, readOneTimeCookie(w, r, connectTokenCookie, RouteConnect))
}

// renderConnect draws the screen, first time or after a refused token.
func (h *handler) renderConnect(w http.ResponseWriter, r *http.Request, status int,
	form tokenForm, errs fieldErrors, issued string) error {
	actor, err := core.RequireActor(r.Context())
	if err != nil {
		return err
	}
	t := onboard.Target{URL: baseURL(r), Tenant: h.tenantKey(r), Token: tokenSlot,
		Version: h.releases.state().Running, SSH: h.sshAddress(r)}
	q := connectQueryOf(r)
	v := connectView{URL: t.URL, Tenant: t.Tenant, Handle: actor.Handle, Self: q.self(),
		Verify: onboard.Verify(), Preview: onboard.MCPPreview(),
		TokenCmd: onboard.Snippet{ID: "token-cli", Lang: "sh", Text: onboard.TokenCommand()},
		Rotate:   onboard.Rotate(),
		Export:   onboard.Snippet{ID: "token-export", Lang: "sh", Text: "export TIX_TOKEN=" + tokenSlot},
		CLI:      onboard.CLI(t), Login: onboard.Login(t, h.email(r)), SSH: onboard.SSH(t), REST: onboard.REST(t),
		CanIssue: actor.HasScope(core.ScopeTokenAdmin), Picker: newScopePicker(actor, form.Scopes),
		Form: form, Expiry: tokenExpiryChoices, Errors: errs, Secret: issuedTokenSecret(issued)}
	v.Panes, v.Tab = q.panes()
	if v.Secret != nil {
		v.Secret.ID, v.Secret.Fill = "connect-token", true
	}
	var install, clients, skill []connectTab
	for _, s := range onboard.Install(t) {
		install = append(install, connectTab{ID: strings.TrimPrefix(s.ID, "install-"), Title: s.Title, Snips: []onboard.Snippet{s}})
	}
	for _, c := range onboard.MCPClients(t) {
		clients = append(clients, connectTab{ID: c.ID, Title: c.Name, Titled: true, Plain: true, Snips: c.Snippets})
	}
	for _, s := range onboard.Skill() {
		skill = append(skill, connectTab{ID: strings.TrimPrefix(s.ID, "skill-"), Title: s.Title, Snips: []onboard.Snippet{s}})
	}
	v.Install = q.tabSet("install", "Install with", install)
	v.Clients = q.tabSet("client", "MCP client", clients)
	v.Skill = q.tabSet("skill", "Install for", skill)
	return h.renderStatus(w, r, status, "connect.html", "Connect an agent", v)
}

// email is the reader's sign-in address, or "" for an actor without an account.
func (h *handler) email(r *http.Request) string {
	me, err := h.svc.WhoAmI(r.Context())
	if err != nil {
		return ""
	}
	return me.Email
}

// connectTokenCookie carries a token issued on this screen to the screen
// that shows it once, as the tokens screen's own cookie does.
const connectTokenCookie = "tix_connect_token" // #nosec G101 -- a cookie name, not a credential

// createConnectToken issues a token for the reader and shows it once.
func (h *handler) createConnectToken(w http.ResponseWriter, r *http.Request) error {
	form := tokenFormOf(r)
	expiresAt, err := form.expiresAt(time.Now())
	if err == nil {
		var issued *core.IssuedToken
		issued, err = h.svc.CreateToken(r.Context(), core.CreateTokenInput{
			Name: form.Name, Scopes: form.Scopes, ExpiresAt: expiresAt})
		if err == nil {
			// #nosec G124 -- one-time value, HttpOnly, cleared by the screen that shows it
			http.SetCookie(w, oneTimeCookie(connectTokenCookie, RouteConnect,
				issuedValue(issued.Token, issued.Scopes), h.secureCookie(r)))
			redirect(w, r, RouteConnect+connectQueryOf(r).self(), "token issued")
			return nil
		}
	}
	if !correctable(err) {
		return err
	}
	r.URL.Path = RouteConnect
	return h.renderConnect(w, r, core.KindOf(err).HTTPStatus(), form, refusal(err), "")
}

// docPages are the files under docs/ the redirect reaches. A test holds the
// list to the directory, so a page cannot be linked that does not exist.
var docPages = []string{
	"README.md", "agents.md", "api.md", "audit-log.md", "calendar.md", "commands.md",
	"configuration.md", "deployment.md", "development.md", "embedding.md", "features.md", "filtering.md",
	"import-export.md", "mcp.md", "migrating.md",
	"scaling.md", "scripting.md", "second-factor.md", "shell-completion.md", "specifications.md",
	"statistics.md", "tenancy.md", "testing.md", "theming.md", "tui.md", "upgrading.md",
	"web-ui.md", "workflows.md",
}

// docsRepo is where the documentation is read, at the running release's
// tag or at main for a development build.
const docsRepo = "https://github.com/heliopsy/tix/blob/"

// redirectDocs sends the reader to one documentation page. Rendered pages
// link here rather than off the server, which keeps every page free of an
// external origin; only a page in docPages is redirected. A section names
// the heading to land on, carried through as the target's fragment.
func (h *handler) redirectDocs(w http.ResponseWriter, r *http.Request) error {
	page := r.PathValue("page")
	if page == "" {
		page = "README.md"
	}
	if !slices.Contains(docPages, page) {
		return core.NotFound("no documentation page %q", page)
	}
	ref := "main"
	if v := h.releases.state().Running; semverTag(v) {
		ref = "v" + strings.TrimPrefix(v, "v")
	}
	target := docsRepo + ref + "/docs/" + page
	if s := r.URL.Query().Get(docsSectionParam); docsSection.MatchString(s) {
		target += "#" + s
	}
	// #nosec G710 -- the target is a constant origin, a page from a fixed list and a checked anchor.
	http.Redirect(w, r, target, http.StatusFound)
	return nil
}

// semverTag reports whether a version names a release tag.
func semverTag(v string) bool {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return false
		}
	}
	return true
}
