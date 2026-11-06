// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"net"
	"net/http"
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
const tokenSlot = "TIX_TOKEN"

// tokenSlotHTML is what each slot is drawn as.
const tokenSlotHTML = `<span class="tokslot" data-token-slot>$TIX_TOKEN</span>`

// slotted escapes a snippet and draws its token slots.
func slotted(text string) template.HTML {
	escaped := template.HTMLEscapeString(text)
	// #nosec G203 -- the text is escaped above; only the fixed slot markup is added.
	return template.HTML(strings.ReplaceAll(escaped, template.HTMLEscapeString(tokenSlot), tokenSlotHTML))
}

// connectTab is one tab of a tab set: a title and the snippets under it.
type connectTab struct {
	ID     string
	Title  string
	Titled bool
	// Plain titles are product names, drawn as they are in every language.
	Plain bool
	Snips []onboard.Snippet
}

// connectView is what the Connect an agent screen renders.
type connectView struct {
	URL      string
	Tenant   string
	Handle   string
	Install  []connectTab
	Verify   onboard.Snippet
	TokenCmd onboard.Snippet
	Export   onboard.Snippet
	CLI      []onboard.Snippet
	Preview  onboard.Snippet
	Clients  []connectTab
	Skill    []connectTab
	SSH      []onboard.Snippet
	REST     []onboard.Snippet
	CanIssue bool
	Picker   scopePicker
	Form     tokenForm
	Expiry   []tokenExpiry
	Errors   fieldErrors
	Secret   *oneTimeSecret
}

// sshAddress is where people reach the SSH listener: the configured public
// address, else the browser's host name with the port the listener binds.
func (h *handler) sshAddress(r *http.Request) string {
	if h.sshListen == "" {
		return ""
	}
	if h.sshPublic != "" {
		return h.sshPublic
	}
	host := r.Host
	if hn, _, err := net.SplitHostPort(r.Host); err == nil {
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

// baseURL is this deployment's address as the request reached it, the same
// derivation the calendar feed uses.
func baseURL(r *http.Request) string {
	return httpapi.SchemeFrom(r.Context()) + "://" + r.Host
}

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
	v := connectView{URL: t.URL, Tenant: t.Tenant, Handle: actor.Handle,
		Verify: onboard.Verify(), Preview: onboard.MCPPreview(),
		TokenCmd: onboard.Snippet{ID: "token-cli", Lang: "sh", Text: onboard.TokenCommand()},
		Export:   onboard.Snippet{ID: "token-export", Lang: "sh", Text: "export TIX_TOKEN=" + tokenSlot},
		CLI:      onboard.CLI(t), SSH: onboard.SSH(t), REST: onboard.REST(t),
		CanIssue: actor.HasScope(core.ScopeTokenAdmin), Picker: newScopePicker(actor, form.Scopes),
		Form: form, Expiry: tokenExpiryChoices, Errors: errs, Secret: issuedTokenSecret(issued)}
	if v.Secret != nil {
		v.Secret.ID, v.Secret.Fill = "connect-token", true
	}
	for _, s := range onboard.Install(t) {
		v.Install = append(v.Install, connectTab{ID: s.ID, Title: s.Title, Snips: []onboard.Snippet{s}})
	}
	for _, c := range onboard.MCPClients(t) {
		v.Clients = append(v.Clients, connectTab{ID: "mcp-" + c.ID, Title: c.Name, Titled: true, Plain: true, Snips: c.Snippets})
	}
	for _, s := range onboard.Skill() {
		v.Skill = append(v.Skill, connectTab{ID: s.ID, Title: s.Title, Snips: []onboard.Snippet{s}})
	}
	return h.renderStatus(w, r, status, "connect.html", "Connect an agent", v)
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
			redirect(w, r, RouteConnect, "token issued")
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
	"README.md", "agents.md", "api.md", "calendar.md", "commands.md", "configuration.md",
	"deployment.md", "development.md", "features.md", "filtering.md", "mcp.md", "migrating.md",
	"scaling.md", "scripting.md", "second-factor.md", "shell-completion.md", "specifications.md",
	"statistics.md", "tenancy.md", "testing.md", "theming.md", "tui.md", "upgrading.md",
	"web-ui.md", "workflows.md",
}

// docsRepo is where the documentation is read, at the running release's
// tag or at main for a development build.
const docsRepo = "https://github.com/heliopsy/tix/blob/"

// redirectDocs sends the reader to one documentation page. Rendered pages
// link here rather than off the server, which keeps every page free of an
// external origin; only a page in docPages is redirected.
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
	// #nosec G710 -- the target is a constant origin and a page from a fixed list.
	http.Redirect(w, r, docsRepo+ref+"/docs/"+page, http.StatusFound)
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
