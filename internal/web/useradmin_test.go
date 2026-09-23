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

// selectedRole finds which option the role control for one account is opened
// with, which is what a browser submits when the reader changes some other
// field on the same form and saves.
func selectedRole(t *testing.T, page, userID string) string {
	t.Helper()
	open := strings.Index(page, `<select id="role-`+userID+`"`)
	if open < 0 {
		t.Fatalf("no role control for user %q:\n%s", userID, page)
	}
	end := strings.Index(page[open:], "</select>")
	if end < 0 {
		t.Fatalf("role control for %q is not closed", userID)
	}
	control := page[open : open+end]
	match := regexp.MustCompile(`<option value="([^"]*)"\s+selected>`).FindStringSubmatch(control)
	if match == nil {
		return ""
	}
	return match[1]
}

// userIDFor finds the identifier the listing rendered for one email, which the
// row carries as its own anchor so a link from the activity feed can reach it.
func userIDFor(t *testing.T, page, email string) string {
	t.Helper()
	at := strings.Index(page, email)
	if at < 0 {
		t.Fatalf("the listing does not show %q:\n%s", email, page)
	}
	ids := regexp.MustCompile(`id="user-([^"]+)"`).FindAllStringSubmatch(page[:at], -1)
	if len(ids) == 0 {
		t.Fatalf("no row identifier before %q", email)
	}
	return ids[len(ids)-1][1]
}

// The role a user holds lives on their membership, not on the account, so the
// listing has to bring the two together. When it did not, the edit control
// listed every role with none of them selected and a browser submitted the
// first one: opening an administrator to tick "disabled" and pressing save
// demoted them to a viewer, silently, with the audit trail recording it as an
// intended change.
func TestEditingAUserDoesNotChangeTheirRole(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/users", url.Values{"email": {"keeper@example.test"},
		"password": {"correct-horse-battery"}, "role": {"admin"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/admin/users")
	id := userIDFor(t, page, "keeper@example.test")

	if got := selectedRole(t, page, id); got != string(core.RoleAdmin) {
		t.Fatalf("the role control opens on %q, but the account holds %q: saving any "+
			"other change on this form would write the wrong role",
			got, core.RoleAdmin)
	}
	if !strings.Contains(page, `<span class="role role-admin">admin</span>`) {
		t.Errorf("the listing does not show the role the account holds:\n%s", page)
	}

	// Submit exactly what the rendered form carries, changing only the state.
	saved := b.post("/admin/users/update", url.Values{
		"id": {id}, "display_name": {""}, "role": {selectedRole(t, page, id)}, "disabled": {"1"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after := b.page("/admin/users")
	if got := selectedRole(t, after, id); got != string(core.RoleAdmin) {
		t.Fatalf("disabling the account changed its role to %q", got)
	}
	if !strings.Contains(after, `<span class="state off"`) {
		t.Errorf("the account does not read as disabled:\n%s", after)
	}
}

// An account the screen cannot resolve a membership for must not have one
// proposed for it, or the same silent write happens by another route.
func TestAnAccountWithNoMembershipProposesNoRole(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/users", url.Values{"email": {"stray@example.test"},
		"password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/admin/users")
	id := userIDFor(t, page, "stray@example.test")

	removed := b.post("/admin/tenant/members/remove", url.Values{"actor_id": {id}})
	_ = removed.Body.Close()
	wantStatus(t, removed, http.StatusSeeOther)

	after := b.page("/admin/users")
	if got := selectedRole(t, after, id); got != "" {
		t.Fatalf("a role was proposed for an account holding none: %q", got)
	}
	if !strings.Contains(after, "leave unchanged") {
		t.Errorf("the control does not offer to leave the role alone:\n%s", after)
	}
	if !strings.Contains(after, `class="role role-none"`) {
		t.Errorf("the row does not say the account has no role:\n%s", after)
	}
}

// The listing has to read as a set of people, each with an identity, an
// authority and a state, rather than as a grid of equal cells.
func TestTheUserListingShowsWhoEachAccountIs(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/users", url.Values{"email": {"rosa@example.test"},
		"display_name": {"Rosa Klebb"}, "password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/admin/users")
	for _, want := range []string{
		`class="peoplelist"`, `class="avatar"`, `>RK<`,
		`class="person-name"`, `Rosa Klebb`, `class="person-mail mono">rosa@example.test<`,
		`class="role role-member">member<`, `class="state on">active<`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the user listing is missing %q:\n%s", want, page)
		}
	}
	// A row has to be addressable, because the activity feed links a user
	// entry straight at it.
	if !strings.Contains(page, `id="user-`) {
		t.Errorf("a user row carries no anchor for a link to land on")
	}
}
