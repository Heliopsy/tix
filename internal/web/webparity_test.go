// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// The browser's two parity gaps and three shortfalls: no screen said who the
// reader was, nothing opened an actor, the task screen drew direct subtasks
// only, and the token and key screens could not list another actor's
// credentials although the command line and the API could.

// profileSection is the Profile section of the settings screen, and nothing
// past it.
func profileSection(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<section class="setgroup" id="profile">`, "</section>")
}

// profileValue is one row's value in the Profile section.
func profileValue(t *testing.T, section, name string) string {
	t.Helper()
	return between(t, section, `data-profile="`+name+`">`, "</span></div>")
}

func TestSettingsProfileSaysWhoTheReaderIs(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	section := profileSection(t, f.as("alice").page("/settings"))
	for name, want := range map[string]string{
		"handle": "alice", "id": f.actorA.ID, "kind": "user", "role": "admin",
		"tenant": "acme", "global": "No", "method": "Browser session", "scopes": `<code class="evchip">*</code>`,
	} {
		if got := profileValue(t, section, name); !strings.Contains(got, want) {
			t.Errorf("profile %s = %q, want it to name %q", name, got, want)
		}
	}
	if !strings.Contains(section, `href="/actors/`+f.actorA.ID+`"`) {
		t.Errorf("the Profile section does not link to the reader's actor screen:\n%s", section)
	}
	nav := between(t, f.as("alice").page("/settings"), `<nav class="setnav"`, "</nav>")
	if !strings.Contains(nav, `href="#profile"`) {
		t.Errorf("the settings navigation does not link Profile:\n%s", nav)
	}
}

// A reader who may not read the tenant record is shown its identifier, not a
// key the screen would have had to guess.
func TestSettingsProfileShowsTheTenantIDToANonAdministrator(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	section := profileSection(t, f.as("viewer").page("/settings"))
	tenant := profileValue(t, section, "tenant")
	if !strings.Contains(tenant, f.tenantA.ID) || strings.Contains(tenant, "acme") {
		t.Errorf("a viewer's tenant row = %q, want the identifier %s", tenant, f.tenantA.ID)
	}
	if role := profileValue(t, section, "role"); !strings.Contains(role, "viewer") {
		t.Errorf("a viewer's role row = %q", role)
	}
	if scopes := profileValue(t, section, "scopes"); strings.Contains(scopes, "task:write") || !strings.Contains(scopes, "audit:read") {
		t.Errorf("a viewer's scopes row is not the viewer role's scopes: %q", scopes)
	}
}

// actorFacts is the facts list of the actor screen.
func actorFacts(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<dl class="actorfacts">`, "</dl>")
}

func TestTheActorScreenShowsAnActorByHandleOrIdentifier(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	agent := seedActor(t, f.store, f.tenantA.ID, "ci-bot", core.RoleMember)
	b := f.as("alice")
	for _, ref := range []string{"ci-bot", agent.ID} {
		facts := actorFacts(t, b.page("/actors/"+ref))
		for _, want := range []string{"ci-bot", agent.ID} {
			if !strings.Contains(facts, want) {
				t.Errorf("/actors/%s does not name %q:\n%s", ref, want, facts)
			}
		}
	}
	resp := b.get("/actors/nobody-here")
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusNotFound)
}

func TestTheActorScreenListsRecentActivityToAnAuditReader(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "fix the router")
	page := b.page("/actors/alice")
	recent := between(t, page, `<section class="actoractivity"`, "</section>")
	if !strings.Contains(recent, ref) {
		t.Errorf("alice's recent activity does not name the task she created, %s:\n%s", ref, recent)
	}
	if !strings.Contains(recent, `href="/activity?actor=`+f.actorA.ID+`"`) {
		t.Errorf("the recent activity does not lead to the feed narrowed to alice:\n%s", recent)
	}

	before := f.svc.auditCalls.Load()
	blind := between(t, f.as("tokenkeeper").page("/actors/alice"), `<section class="actoractivity"`, "</section>")
	if !strings.Contains(blind, "needs the audit:read scope") || strings.Contains(blind, ref) {
		t.Errorf("a reader without audit:read is not told why the list is missing:\n%s", blind)
	}
	if f.svc.auditCalls.Load() != before {
		t.Errorf("the screen asked for the audit log on behalf of a reader without audit:read")
	}
}

func TestTheActorScreenOffersCredentialsOnlyToATokenAdministrator(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	links := func(as string) string {
		return between(t, f.as(as).page("/actors/alice"), `<ul class="plain actorlinks">`, "</ul>")
	}
	if l := links("tokenkeeper"); !strings.Contains(l, `href="/admin/tokens?actor=`+f.actorA.ID+`"`) ||
		!strings.Contains(l, `href="/admin/ssh-keys?actor=`+f.actorA.ID+`"`) {
		t.Errorf("a token administrator is not offered the actor's credentials:\n%s", l)
	}
	if l := links("member:" + f.actorA.ID); strings.Contains(l, "/admin/tokens") || strings.Contains(l, "/admin/ssh-keys") {
		t.Errorf("a member is offered credential screens they cannot open:\n%s", l)
	}
}

func TestActorsNamedOnAScreenLinkToTheirActorScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "linked people")
	resp := b.post("/tasks/"+ref+"/comments", url.Values{"body": {"looking"}})
	_ = resp.Body.Close()
	resp = b.post("/tasks/"+ref, url.Values{"title": {"linked people"}, "version": {"1"}, "assignee": {"alice"}})
	_ = resp.Body.Close()
	link := `href="/actors/` + f.actorA.ID + `"`

	page := b.page("/tasks/" + ref)
	byline := between(t, page, `<p class="byline">`, "</p>")
	if !strings.Contains(byline, link) {
		t.Errorf("a comment's author does not link to their actor screen:\n%s", byline)
	}
	creator := between(t, page, `<dt>Created by</dt>`, "</dd>")
	if !strings.Contains(creator, link) {
		t.Errorf("the creator does not link to their actor screen:\n%s", creator)
	}
	assignee := between(t, page, `data-prop="assignee">`, "</dt>")
	if !strings.Contains(assignee, link) {
		t.Errorf("the assignee does not link to their actor screen:\n%s", assignee)
	}
	directory := between(t, b.page("/actors"), `<tr id="actor-`+f.actorA.ID+`">`, "</tr>")
	if !strings.Contains(directory, link) {
		t.Errorf("the directory's handle does not link to the actor screen:\n%s", directory)
	}
}

// seedChain makes a chain of subtasks n deep under a new task and returns
// every reference, the root first.
func seedChain(t *testing.T, f *fixture, n int) []string {
	t.Helper()
	b := f.as("alice")
	refs := []string{b.createTask("infra", "root")}
	for i := range n {
		task, err := f.svc.CreateTask(adminCtx(f), core.CreateTaskInput{
			ProjectRef: "infra", Title: "level " + string(rune('1'+i)), ParentRef: refs[len(refs)-1]})
		if err != nil {
			t.Fatalf("creating level %d: %v", i+1, err)
		}
		refs = append(refs, task.Ref)
	}
	return refs
}

// subtaskSection is the Subtasks section of a task screen.
func subtaskSection(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<section class="tsec" id="subtasks">`, "</section>")
}

// treeLinks are the task references a stretch of the tree links to, in order.
var treeLinks = regexp.MustCompile(`<a class="mono" href="/tasks/([^"#]+)">`)

func linkedRefs(html string) []string {
	var out []string
	for _, m := range treeLinks.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestTheTaskScreenDrawsThreeLevelsAndOffersTheRest(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	refs := seedChain(t, f, 5)
	section := subtaskSection(t, f.as("alice").page("/tasks/"+refs[0]))
	got := linkedRefs(section)
	if strings.Join(got, " ") != strings.Join(refs[1:4], " ") {
		t.Errorf("the tree links %v, want the three levels %v", got, refs[1:4])
	}
	more := between(t, section, `class="btn link small treemore"`, "</a>")
	if !strings.Contains(more, `href="/tasks/`+refs[3]+`#subtasks"`) ||
		!strings.Contains(more, `hx-get="/tasks/`+refs[3]+`/tree?id=`) || !strings.Contains(more, "Show 1 subtask") {
		t.Errorf("the third level does not offer its subtasks:\n%s", more)
	}

	fragment := f.as("alice").page("/tasks/" + refs[3] + "/tree?id=subtree-x")
	if !strings.HasPrefix(strings.TrimSpace(fragment), `<ul class="rellist rows subtree" id="subtree-x">`) {
		t.Errorf("the subtree fragment is not the list under the id asked for:\n%s", fragment)
	}
	if got := linkedRefs(fragment); strings.Join(got, " ") != strings.Join(refs[4:], " ") {
		t.Errorf("the subtree fragment links %v, want %v", got, refs[4:])
	}
	odd := f.as("alice").page("/tasks/" + refs[3] + `/tree?id="><script>`)
	if strings.Contains(odd, "<script>") {
		t.Errorf("the fragment carries an id the page did not draw:\n%s", odd)
	}
}

func TestASubtaskTreeNodeCarriesAFoldButtonForItsOwnList(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	refs := seedChain(t, f, 2)
	section := subtaskSection(t, f.as("alice").page("/tasks/"+refs[0]))
	row := between(t, section, `<li class="treenode" data-level="1">`, `</div>`)
	button := between(t, row, `<button type="button" class="ghost small treetoggle"`, "</button>")
	if !strings.Contains(button, `aria-expanded="true"`) || !strings.Contains(button, " hidden>") {
		t.Errorf("the fold button is not drawn hidden and expanded:\n%s", button)
	}
	controls := regexp.MustCompile(`aria-controls="([^"]+)"`).FindStringSubmatch(button)
	if controls == nil || !strings.Contains(section, `<ul class="rellist rows subtree" id="`+controls[1]+`">`) {
		t.Errorf("the fold button controls no list in the tree:\n%s", section)
	}
	leaf := between(t, section, `<li class="treenode" data-level="2">`, `</li>`)
	if strings.Contains(leaf, "treetoggle") {
		t.Errorf("a leaf carries a fold button:\n%s", leaf)
	}
}

func TestTheCredentialScreensListANamedActor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	carol, theirs := seedOtherActorsToken(t, f, "carol", "carols-agent")
	if _, err := f.svc.EnrolSSHKey(adminCtx(f), core.EnrolSSHKeyInput{
		ActorID: carol.ID, PublicKey: webKeyBob, Label: "carols-laptop"}); err != nil {
		t.Fatal(err)
	}
	b := f.as("tokenkeeper")

	tokens := b.page("/admin/tokens?actor=carol")
	if rows := tokenRows(t, tokens); !strings.Contains(rows, "carols-agent") || !strings.Contains(rows, `href="/actors/`+carol.ID+`"`) {
		t.Errorf("the token screen filtered to carol does not list her token with its owner:\n%s", rows)
	}
	if revokeControlFor(tokens, theirs.ID) == "" {
		t.Errorf("no revoke control is offered for the token the filter listed")
	}
	if !strings.Contains(tokens, "Listing what carol holds.") {
		t.Errorf("the token screen does not say whose tokens it lists")
	}

	keys := between(t, b.page("/admin/ssh-keys?actor="+carol.ID), "<tbody>", "</tbody>")
	if !strings.Contains(keys, webFingerprintBob) {
		t.Errorf("the key screen filtered to carol does not list her key:\n%s", keys)
	}
	own := between(t, b.page("/admin/ssh-keys"), "<tbody>", "</tbody>")
	if strings.Contains(own, webFingerprintBob) {
		t.Errorf("the unfiltered key screen lists somebody else's key:\n%s", own)
	}

	missing := b.page("/admin/ssh-keys?actor=nobody-here")
	if !strings.Contains(missing, "No actor of this tenant is named nobody-here.") {
		t.Errorf("a filter naming nobody is not reported beside the box")
	}
}
