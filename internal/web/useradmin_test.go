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

// personRow is the one list item of the user listing belonging to an account,
// so an assertion about what marks one row reads that row. Every row carries
// the same words, and the chip naming the reader is one span among them.
func personRow(t *testing.T, page, userID string) string {
	t.Helper()
	row := between(t, page, `id="user-`+userID+`"`, "</li>")
	if row == "" {
		t.Fatalf("the listing has no row for %q:\n%s", userID, page)
	}
	return row
}

// The chip marking the reader's own account belongs on exactly one row. Read
// across the whole page it is indistinguishable from marking every row, which
// is what leaving the identity comparison out does.
func TestOnlyTheReadersOwnAccountIsMarkedAsTheirs(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	admin := f.as("alice")
	for _, email := range []string{"mine@example.test", "theirs@example.test"} {
		created := admin.post("/admin/users", url.Values{"email": {email},
			"password": {"correct-horse-battery"}, "role": {"admin"}})
		_ = created.Body.Close()
		wantStatus(t, created, http.StatusSeeOther)
	}
	listing := admin.page("/admin/users")
	mine := userIDFor(t, listing, "mine@example.test")
	theirs := userIDFor(t, listing, "theirs@example.test")

	page := f.asActor(mine).page("/admin/users")
	if !strings.Contains(personRow(t, page, mine), `class="chip you"`) {
		t.Errorf("the reader's own account is not marked as theirs:\n%s",
			personRow(t, page, mine))
	}
	if strings.Contains(personRow(t, page, theirs), `class="chip you"`) {
		t.Errorf("somebody else's account is marked as the reader's:\n%s",
			personRow(t, page, theirs))
	}
}

// The listing's first line is a count of accounts by state, and it is the
// only place the screen says how many there are.
func TestTheUserListingCountsActiveAndDisabledAccounts(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, email := range []string{"one@example.test", "two@example.test", "three@example.test"} {
		created := b.post("/admin/users", url.Values{"email": {email},
			"password": {"correct-horse-battery"}, "role": {"member"}})
		_ = created.Body.Close()
		wantStatus(t, created, http.StatusSeeOther)
	}
	listing := b.page("/admin/users")
	off := userIDFor(t, listing, "three@example.test")
	disabled := b.post("/admin/users/update", url.Values{
		"id": {off}, "role": {"member"}, "disabled": {"1"}})
	_ = disabled.Body.Close()
	wantStatus(t, disabled, http.StatusSeeOther)

	summary := between(t, b.page("/admin/users"), `<p class="lede summary">`, "</p>")
	if !strings.Contains(summary, "<strong>2</strong> active") {
		t.Errorf("the listing does not count the accounts still in use:\n%s", summary)
	}
	// Read the whole figure, not its last digit: a count of -1 renders, and
	// contains the "1 disabled" a laxer assertion would accept.
	if !strings.Contains(summary, `class="held">1 disabled<`) {
		t.Errorf("the listing does not count the accounts turned off:\n%s", summary)
	}
}

// Changing a role has to change it. The guard that this screen does not
// change a role by accident is satisfied by a screen that cannot change one
// at all, so the deliberate case needs its own.
func TestChangingAUsersRoleTakesEffect(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	created := b.post("/admin/users", url.Values{"email": {"promoted@example.test"},
		"password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/admin/users")
	id := userIDFor(t, page, "promoted@example.test")
	saved := b.post("/admin/users/update", url.Values{"id": {id}, "role": {"admin"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after := b.page("/admin/users")
	if got := selectedRole(t, after, id); got != string(core.RoleAdmin) {
		t.Errorf("the role control still opens on %q after the role was changed to admin", got)
	}
	if !strings.Contains(personRow(t, after, id), `class="role role-admin">admin<`) {
		t.Errorf("the row does not show the role that was chosen:\n%s", personRow(t, after, id))
	}
}

// A display name the form can set, the form has to be able to remove. The
// input is rendered with the current name in it, so emptying it and saving is
// the only gesture the screen offers for "this account has no display name",
// and the service clears the name for any DisplayName the input carries,
// including an empty one.
func TestEmptyingTheDisplayNameRemovesIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/users", url.Values{"email": {"named@example.test"},
		"display_name": {"Named Person"}, "password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/admin/users")
	id := userIDFor(t, page, "named@example.test")
	if got := inputValue(t, personRow(t, page, id), "name-"+id); got != "Named Person" {
		t.Fatalf("the name input holds %q before it is emptied", got)
	}

	saved := b.post("/admin/users/update", url.Values{"id": {id},
		"display_name": {""}, "role": {"member"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after := b.page("/admin/users")
	if got := inputValue(t, personRow(t, after, id), "name-"+id); got != "" {
		t.Errorf("emptying the display name left %q on the account", got)
	}
}

// The avatar's initials are characters, not bytes. Every name the guards above
// use is ASCII, where the two are the same thing; a name that is not leaves a
// fragment of a character in the response, which is invalid UTF-8 and renders
// as a replacement glyph rather than as the reader's own initial.
func TestTheUserAvatarTakesWholeCharacters(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/users", url.Values{"email": {"emile@example.test"},
		"display_name": {"Émile Zola"}, "password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/admin/users")
	id := userIDFor(t, page, "emile@example.test")
	row := personRow(t, page, id)
	if !strings.Contains(row, `class="avatar" aria-hidden="true">ÉZ<`) {
		t.Errorf("the avatar does not read ÉZ:\n%s", row)
	}
	if strings.Contains(row, "\uFFFD") {
		t.Errorf("the row carries a replacement character where a name should be:\n%s", row)
	}
}

// The same property for a name of a single word, which takes the other branch.
func TestTheUserAvatarTakesAWholeCharacterFromAOneWordName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/users", url.Values{"email": {"otzi@example.test"},
		"display_name": {"Ötzi"}, "password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/admin/users")
	id := userIDFor(t, page, "otzi@example.test")
	row := personRow(t, page, id)
	if !strings.Contains(row, `class="avatar" aria-hidden="true">Ö<`) {
		t.Errorf("the avatar does not read Ö:\n%s", row)
	}
	if strings.Contains(row, "\uFFFD") {
		t.Errorf("the row carries a replacement character where a name should be:\n%s", row)
	}
}
