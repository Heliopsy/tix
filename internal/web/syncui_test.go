// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
)

// joinedNames closes up the break opportunities the screen puts inside an
// environment variable name, so an assertion is about the name a reader sees
// rather than about where it is allowed to wrap.
func joinedNames(page string) string {
	return strings.ReplaceAll(page, "<wbr>", "")
}

// section returns the markup between one heading and the end of the table
// that follows it, so an assertion about a table is not satisfied by prose
// elsewhere on the page. "ok" in particular occurs in half a dozen English
// words the screen already uses.
func section(t *testing.T, page, heading string) string {
	t.Helper()
	start := strings.Index(page, heading)
	if start < 0 {
		t.Fatalf("the page has no %s section", heading)
	}
	rest := page[start:]
	end := strings.Index(rest, "</table>")
	if end < 0 {
		t.Fatalf("the %s table is never closed", heading)
	}
	return rest[:end]
}

// addSource registers one import source through the screen itself, so a test
// exercises the form a reader actually submits rather than the service behind
// it.
func addSource(t *testing.T, b *browser, system, name string) {
	t.Helper()
	b.page("/sync")
	resp := b.post("/sync/sources", url.Values{"system": {system}, "name": {name}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("saving source = %d, want 303", resp.StatusCode)
	}
}

func TestSyncScreenSaysWhatItImportsAndWhatARunDoes(t *testing.T) {
	t.Parallel()
	page := newFixture(t).as("alice").page("/sync")

	// Each of these is a claim the screen has to make for somebody landing on
	// it cold to know what they are looking at: the direction of the traffic,
	// what "generic" is for, where the settings actually live, that a mapping
	// file exists at all, and what the second run does differently.
	for _, want := range []string{
		"never writes anything back",
		"CSV or JSON file on the machine running tix",
		"environment variables on the machine running tix",
		"mapping file",
		"external reference",
		"refresh instead of a second copy",
		"nothing at all: no tasks",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the sync screen does not say %q", want)
		}
	}
}

func TestSyncScreenNamesTheVariablesEachSourceReads(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	addSource(t, b, "generic", "night sync")
	page := joinedNames(b.page("/sync"))

	// The name-to-variable rule is the thing nobody can apply in their head,
	// so the screen has to do it: a source called "night sync" is configured
	// through TIX_SYNC_NIGHT_SYNC_*, not TIX_SYNC_NIGHT SYNC_* or anything
	// else a reader might guess.
	for _, want := range []string{
		"TIX_SYNC_NIGHT_SYNC_",
		"TIX_SYNC_NIGHT_SYNC_MAPPING",
		"TIX_SYNC_NIGHT_SYNC_FILE",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the sync screen does not name %q", want)
		}
	}
	if strings.Contains(page, "TIX_SYNC_NIGHT_SYNC_URL") {
		t.Error("the sync screen offers a generic source a url variable, which no file adapter reads")
	}
}

func TestSyncScreenNamesOnlyTheVariablesThatSystemReads(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	addSource(t, b, "jira", "platform")
	page := joinedNames(b.page("/sync"))

	for _, want := range []string{"TIX_SYNC_PLATFORM_URL", "TIX_SYNC_PLATFORM_QUERY",
		"TIX_SYNC_PLATFORM_TOKEN"} {
		if !strings.Contains(page, want) {
			t.Errorf("a jira source does not name %q", want)
		}
	}
	if strings.Contains(page, "TIX_SYNC_PLATFORM_FILE") {
		t.Error("a jira source offers a file variable, which no http adapter reads")
	}
}

func TestSyncScreenOffersNoFieldItCannotStore(t *testing.T) {
	t.Parallel()
	page := newFixture(t).as("alice").page("/sync")

	// A sync source record holds an id, a system and a name. The screen used
	// to collect a mapping path and a free-text configuration block as well,
	// neither of which was stored anywhere, so filling them in produced a
	// saved source and no working import.
	for _, dead := range []string{`name="mapping_path"`, `name="config"`} {
		if strings.Contains(page, dead) {
			t.Errorf("the sync screen still collects %s, which no source record keeps", dead)
		}
	}
}

func TestSyncScreenOffersAFullRefresh(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	addSource(t, b, "generic", "ops")
	page := b.page("/sync")

	// RunSync has always honoured Full; the screen simply never offered it,
	// so the only way to reconsider every record was the CLI.
	if !strings.Contains(page, `name="full"`) {
		t.Error("the sync screen offers no full refresh")
	}
}

func TestSyncRunsListsWhatRanAndWhatItDid(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	addSource(t, b, "generic", "ops")

	resp := b.post("/sync/run", url.Values{"source_id": {"src-ops"}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("running sync = %d, want 200", resp.StatusCode)
	}

	page := b.page("/sync/runs")
	runs := section(t, page, "<h2>Completed runs</h2>")
	// One assertion per column, with a value no other column carries, so a
	// count rendered into the wrong cell or dropped entirely is caught.
	for _, want := range []string{">ops<", ">generic<", ">2<", ">3<", ">5<",
		"assignee nobody has no tix actor"} {
		if !strings.Contains(runs, want) {
			t.Errorf("the completed runs table does not show %q", want)
		}
	}
	if strings.Contains(page, "No import has finished yet") {
		t.Error("the run history reports nothing after a run finished")
	}
}

func TestSyncRunsShowsNoRowForADryRun(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	addSource(t, b, "generic", "ops")

	resp := b.post("/sync/run", url.Values{"source_id": {"src-ops"}, "dry_run": {"1"}})
	defer func() { _ = resp.Body.Close() }()

	// A dry run writes nothing, so there is nothing for the history to read.
	// The screen has to say so, or an empty list after a dry run reads as a
	// broken page rather than as the correct answer.
	page := b.page("/sync/runs")
	if !strings.Contains(page, "No import has finished yet") {
		t.Error("a dry run left a row in the run history")
	}
	if !strings.Contains(page, "A dry run does not count") {
		t.Error("the empty run history does not say why a dry run is absent")
	}
}

func TestSyncRunsSaysWhatItCannotShow(t *testing.T) {
	t.Parallel()
	page := newFixture(t).as("alice").page("/sync/runs")

	// The two gaps are real and neither is visible from the table: a failed
	// run writes no completion entry, and a completed one carries no cursor.
	// Saying so is the difference between a screen that is honest and one
	// that looks complete and is not.
	for _, want := range []string{
		"writes nothing, so it",
		"stops without",
		"cannot show where the cursor stood",
		"Where each source stands",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the run history does not admit %q", want)
		}
	}
}

func TestSyncRunsReportsTheStateOfEverySource(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	addSource(t, b, "generic", "ops")

	state := section(t, b.page("/sync/runs"), "<h2>Where each source stands</h2>")
	if !strings.Contains(state, "never run") {
		t.Error("a source that has never run is not reported as such")
	}

	resp := b.post("/sync/run", url.Values{"source_id": {"src-ops"}})
	defer func() { _ = resp.Body.Close() }()
	state = section(t, b.page("/sync/runs"), "<h2>Where each source stands</h2>")
	if !strings.Contains(state, ">ok<") {
		t.Error("a source that ran does not report its status")
	}
	if strings.Contains(state, "never run") {
		t.Error("a source that ran is still reported as never run")
	}
}

// TestSyncRunsReadsTheActionTheServiceWrites is the drift guard the constants
// in sync.go point at. The run history is built by filtering the audit trail
// on one action string, which internal/service writes and does not export, so
// nothing but this comparison stops the two from parting company and leaving
// a screen that silently lists nothing for ever.
func TestSyncRunsReadsTheActionTheServiceWrites(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../service/sync.go")
	if err != nil {
		t.Fatalf("reading the service the screen mirrors: %v", err)
	}
	for _, c := range []struct{ name, want string }{
		{"auditSyncRun", "sync.run"},
		{"syncSourceSubject", "sync_source"},
		{"syncEntityTask", "task"},
	} {
		pattern := regexp.MustCompile(c.name + `\s*=\s*"([^"]*)"`)
		found := pattern.FindSubmatch(raw)
		if found == nil {
			t.Errorf("internal/service no longer declares %s; the run history filters on a string nothing writes", c.name)
			continue
		}
		if got := string(found[1]); got != c.want {
			t.Errorf("service %s = %q, but the run history reads %q", c.name, got, c.want)
		}
	}
}

func TestTenantMembersAreNamedRatherThanIdentified(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.page("/admin/tenant")
	resp := b.post("/admin/tenant/members",
		url.Values{"actor_id": {f.actorA.ID}, "role": {"admin"}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("adding a member = %d, want 303", resp.StatusCode)
	}

	members := membersTable(t, b.page("/admin/tenant"))

	// The table sits directly under a diagram calling these rows "people".
	// A 26-character ULID is not a person, and the handle is already there
	// to be looked up.
	if !strings.Contains(members, ">alice<") {
		t.Error("the members table does not name the member")
	}
	if !strings.Contains(members, `title="`+f.actorA.ID+`"`) {
		t.Error("the members table drops the identifier entirely rather than keeping it on the row")
	}
	if strings.Contains(members, ">"+f.actorA.ID+"<") {
		t.Errorf("the members table still renders the bare identifier %s as its cell", f.actorA.ID)
	}
}

// membersTable is the members table alone, so an assertion about it cannot be
// satisfied by the signed-in actor's own name in the page header.
func membersTable(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, "<h2>Members</h2>")
	if start < 0 {
		t.Fatal("the tenant screen has no members table")
	}
	rest := page[start:]
	end := strings.Index(rest, "</table>")
	if end < 0 {
		t.Fatal("the members table is never closed")
	}
	return rest[:end]
}

func TestSyncRunsDoNotLeakAcrossTenants(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	bob := f.as("bob")
	addSource(t, bob, "generic", "other tenant source")
	resp := bob.post("/sync/run", url.Values{"source_id": {"src-other tenant source"}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("running sync as bob = %d, want 200", resp.StatusCode)
	}

	// The history is read out of the audit trail, which is the one place a
	// screen can accidentally show another tenant's work: the filter names an
	// action, not a tenant, and the scoping has to come from the store.
	page := f.as("alice").page("/sync/runs")
	if strings.Contains(page, "other tenant source") {
		t.Error("the run history shows another tenant's import")
	}
	if !strings.Contains(page, "No import has finished yet") {
		t.Error("the run history is not empty for a tenant that has run nothing")
	}
}
