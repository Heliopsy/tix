// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// A screen that presents a part of a listing as the whole of it is the defect
// these guards are about. Each reads what the screen renders rather than what
// a helper returned: a walk that reaches the end while the template prints a
// figure from somewhere else is the shape that has passed review here before.

// shapeFigure returns the number one row of the tenant diagram prints, and -1
// where the row prints a word in the figure's place. The row, not the page:
// the diagram carries five other figures and a page-wide match is satisfied
// by any of them.
func shapeFigure(t *testing.T, page, name string) int {
	t.Helper()
	row := shapeRow(t, page, name)
	if !strings.Contains(row, `<span class="count">`) {
		return -1
	}
	return atoi(t, between(t, row, `<span class="count">`, "</span>"))
}

// atoi reads a figure the screen rendered.
func atoi(t *testing.T, raw string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		t.Fatalf("the row's figure %q is not a number: %v", raw, err)
	}
	return n
}

// tokenRowCount counts the rows the token table renders.
func tokenRowCount(t *testing.T, page string) int {
	t.Helper()
	rows := tokenRows(t, page)
	if strings.Contains(rows, "No tokens.") {
		return 0
	}
	return strings.Count(rows, "<tr>")
}

// visibilityControl returns the project visibility form, so an assertion
// about what that control says reads the control: both screens carrying it
// also carry prose about hiding projects, and a page-wide match would be
// satisfied by the help text above it.
func visibilityControl(t *testing.T, page string) string {
	t.Helper()
	form := between(t, page, `class="visibilityform"`, "</form>")
	if form == "" {
		t.Fatalf("the page renders no project visibility control")
	}
	return form
}

// statsPicker returns the project control of the statistics screen, so an
// assertion about what can be chosen reads the control rather than the page:
// a project key reaches the leaderboard and the accent map too.
func statsPicker(t *testing.T, page string) string {
	t.Helper()
	picker := between(t, page, `<select id="stats-project"`, "</select>")
	if picker == "" {
		t.Fatalf("the statistics screen renders no project picker")
	}
	return picker
}

// The diagram counted ListTokens with an empty actor, which has always meant
// the caller's own, while the screen it links to shows a tenant administrator
// every token of the tenant. The figure and the screen one click away
// disagreed, and the row sat between Members, Domains and Workflows, which are
// all tenant-wide.
func TestTheTenantTokenFigureIsTheTenantsNotTheReadersOwn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, theirs := seedOtherActorsToken(t, f, "carol", "carols-agent")

	b := f.as("alice")
	issued := issueToken(t, b, url.Values{"name": {"mine"}, "scopes": {"task:read"}})
	_ = issued.Body.Close()

	tokens := b.page("/admin/tokens")
	onScreen := tokenRowCount(t, tokens)
	if onScreen != 2 {
		t.Fatalf("the token screen shows %d rows, want the reader's own and carol's:\n%s",
			onScreen, tokenRows(t, tokens))
	}
	if !strings.Contains(tokenRows(t, tokens), theirs.ID) {
		t.Fatalf("the token screen does not list carol's token at all")
	}

	figure := shapeFigure(t, b.page("/admin/tenant"), "API tokens")
	if figure == 1 {
		t.Errorf("the API tokens row shows 1, which is the reader's own count, not the tenant's %d", onScreen)
	}
	if figure != onScreen {
		t.Errorf("the API tokens row shows %d while the screen it links to shows %d rows",
			figure, onScreen)
	}
}

// seedActorsPastAPage fills the directory past one listing page of
// core.MaxPageLimit and returns an actor that cannot be on the first page.
// The handles are chosen so the ordering, which is by handle, puts the
// returned one last.
func seedActorsPastAPage(t *testing.T, f *fixture) core.Actor {
	t.Helper()
	for i := range core.MaxPageLimit + 5 {
		seedActor(t, f.store, f.tenantA.ID, fmt.Sprintf("crowd%04d", i), core.RoleMember)
	}
	far := seedActor(t, f.store, f.tenantA.ID, "zzz-last-of-all", core.RoleMember)
	listed, next, err := f.svc.ListActors(f.ctx(), core.Page{Limit: core.MaxPageLimit})
	if err != nil {
		t.Fatalf("listing actors: %v", err)
	}
	if next == "" {
		t.Fatalf("one page of %d holds the whole directory, so nothing is past it", core.MaxPageLimit)
	}
	for _, a := range listed {
		if a.ID == far.ID {
			t.Fatalf("actor %q is on the first page, so this seed proves nothing", far.Handle)
		}
	}
	return far
}

// The admin listing read one page of core.MaxPageLimit actors and dropped the
// cursor, so a tenant with more actors than that lost the tail of its
// credentials off the screen an operator reads during an incident: the token
// they came to revoke was simply not there.
func TestTheTokenListingReachesATokenHeldPastTheFirstActorPage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	far := seedActorsPastAPage(t, f)
	issued, err := f.svc.CreateToken(adminCtx(f), core.CreateTokenInput{
		Name: "far-agent", ActorID: far.ID, Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("minting the far actor's token: %v", err)
	}

	rows := tokenRows(t, f.as("alice").page("/admin/tokens"))
	if !strings.Contains(rows, issued.ID) {
		t.Errorf("the listing does not carry the token of an actor past the first page")
	}
	if revokeControlFor(rows, issued.ID) == "" {
		t.Errorf("the listing offers no revocation for that token, so it cannot be ended here")
	}
}

// The picker read one page of core.DefaultPageLimit projects, so a tenant past
// fifty of them could not filter the statistics by its fifty-first. The
// figures were never wrong; the control simply did not offer the project.
func TestTheStatisticsPickerOffersAProjectPastTheFirstPage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	far := seedProjectsPastAPage(t, f)

	page := b.page("/stats")
	picker := statsPicker(t, page)
	if !strings.Contains(picker, `value="`+far+`"`) {
		t.Fatalf("the picker offers no option for %q, which is past the first listing page", far)
	}
	if want := core.DefaultPageLimit + 11 + 1; strings.Count(picker, "<option") != want {
		t.Errorf("the picker offers %d options, want %d projects plus \"Every project\"",
			strings.Count(picker, "<option"), want)
	}

	filtered := b.page("/stats?project=" + far)
	if !strings.Contains(statsPicker(t, filtered), `value="`+far+`" selected`) {
		t.Errorf("choosing that project does not come back selected")
	}
}

// Every screen whose listing can stop before the end says so, in the register
// the rest of the interface uses, and says nothing when it reached the end.
// Both directions: a template that printed the caveat unconditionally would
// satisfy the first half on its own.
func TestAListingThatCouldNotReadEverythingSaysSo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		actors  bool
		path    string
		says    string
		read    func(t *testing.T, page string) string
		figures string
	}{
		{
			name: "the token table", actors: true, path: "/admin/tokens",
			says: "not every token of the tenant",
			read: func(t *testing.T, page string) string { t.Helper(); return page },
		},
		{
			name: "the tenant diagram's API tokens row", actors: true, path: "/admin/tenant",
			says: "incomplete", figures: "API tokens",
			read: func(t *testing.T, page string) string {
				t.Helper()
				return shapeRow(t, page, "API tokens")
			},
		},
		{
			name: "the tenant diagram's Projects row", path: "/admin/tenant",
			says: "incomplete", figures: "Projects",
			read: func(t *testing.T, page string) string {
				t.Helper()
				return shapeRow(t, page, "Projects")
			},
		},
		{
			name: "the statistics picker", path: "/stats",
			says: "does not list every project in this tenant",
			read: func(t *testing.T, page string) string { t.Helper(); return page },
		},
		{
			name: "the task screen's visibility control", path: "/tasks",
			says: "not the whole set",
			read: visibilityControl,
		},
		{
			name: "the settings visibility control", path: "/settings",
			says: "not the whole set",
			read: visibilityControl,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			whole := newFixture(t)
			if said := c.read(t, whole.as("alice").page(c.path)); strings.Contains(said, c.says) {
				t.Errorf("%s says it is short when the walk reached the end:\n%s", c.name, said)
			}
			if c.figures != "" {
				if figure := shapeFigure(t, whole.as("alice").page("/admin/tenant"), c.figures); figure < 0 {
					t.Errorf("the %s row carries no figure when the walk reached the end", c.figures)
				}
			}

			short := newFixture(t)
			short.svc.actorPagesNeverEnd = c.actors
			short.svc.projectPagesNeverEnd = !c.actors
			page := short.as("alice").page(c.path)
			said := c.read(t, page)
			if !strings.Contains(said, c.says) {
				t.Errorf("%s presents a partial listing as the whole one:\n%s", c.name, said)
			}
			if c.figures != "" {
				if figure := shapeFigure(t, page, c.figures); figure >= 0 {
					t.Errorf("the %s row prints %d as a total when its walk stopped short", c.figures, figure)
				}
			}
		})
	}
}
