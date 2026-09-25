// SPDX-License-Identifier: AGPL-3.0-or-later

package demo

import (
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// fixtureDays is the span the tables below are written against. A seed asked
// for a different window scales the offsets onto it.
const fixtureDays = 90

// noDue marks a task nobody put a date on.
const noDue = -1

// Statuses of the builtin workflow the fixture moves tasks through.
const (
	statusDoing   = "doing"
	statusBlocked = "blocked"
	statusDone    = "done"
)

// outcome is the state a seeded task is left in at the end of the window.
type outcome string

// Outcomes.
const (
	outcomeTodo    outcome = "todo"
	outcomeDoing   outcome = "doing"
	outcomeBlocked outcome = "blocked"
	outcomeDone    outcome = "done"
)

// agentScopes are what a fleet agent's token carries: enough to pick work up,
// move it and report on it, and nothing that administers the installation.
var agentScopes = []core.Scope{
	core.ScopeTaskRead, core.ScopeTaskWrite, core.ScopeTaskTransition,
	core.ScopeTaskClaim, core.ScopeCommentWrite, core.ScopeArtifactWrite,
	core.ScopeProjectRead, core.ScopeWorkflowRead, core.ScopeEventSubscribe,
}

// projectSeed is one list the demo tenant works out of.
type projectSeed struct {
	key         string
	name        string
	description string
	color       core.ProjectColor
	icon        string
}

var projects = []projectSeed{
	{key: "infra", name: "Infrastructure", color: core.ColorSlate, icon: "\U0001F3D7",
		description: "Clusters, networking and everything the rest of the estate runs on."},
	{key: "web", name: "Web UI", color: core.ColorBlue, icon: "\U0001F5A5",
		description: "The browser interface: boards, filters, charts and the settings screens."},
	{key: "agents", name: "Agent Fleet", color: core.ColorViolet, icon: "\U0001F916",
		description: "The autonomous workers that claim queued tasks and report back."},
	{key: "docs", name: "Documentation", color: core.ColorTeal, icon: "\U0001F4DA",
		description: "Guides, reference pages and the examples people copy out of them."},
	{key: "ops", name: "Operations", color: core.ColorAmber, icon: "\U0001F6E0",
		description: "On-call, incidents, alerting and the routine that keeps them rare."},
	{key: "data", name: "Data Platform", color: core.ColorGreen, icon: "\U0001F4CA",
		description: "Warehouse, pipelines and the reporting everyone else reads."},
}

// personSeed is one identity the fixture writes history as.
type personSeed struct {
	handle string
	name   string
	email  string
	role   core.Role
	agent  bool
	// signIn gives this person a password, so the demo can be opened in a
	// browser as them. Only people get one: an agent authenticates with the
	// token minted beside it, and a password on it would describe a way of
	// running a fleet that the product does not have.
	signIn bool
}

// The two accounts that can sign in are an admin and a member, because the
// screens differ by role and a demo showing only the administrator's view
// hides half of what the product does. No viewer is given a password: a viewer
// may not create or move work, so an eighth identity would be needed to hold
// the role, and it would author nothing anywhere in the seeded history.
var people = []personSeed{
	{handle: "nadia", name: "Nadia Petrova", email: "nadia@demo.invalid", role: core.RoleAdmin, signIn: true},
	{handle: "tom", name: "Tom Okafor", email: "tom@demo.invalid", role: core.RoleMember, signIn: true},
	{handle: "lena", name: "Lena Fischer", email: "lena@demo.invalid", role: core.RoleMember},
	{handle: "raj", name: "Raj Mehta", email: "raj@demo.invalid", role: core.RoleMember},
	{handle: "atlas", name: "Atlas (build agent)", email: "atlas@demo.invalid", role: core.RoleMember, agent: true},
	{handle: "scout", name: "Scout (triage agent)", email: "scout@demo.invalid", role: core.RoleMember, agent: true},
	{handle: "mint", name: "Mint (docs agent)", email: "mint@demo.invalid", role: core.RoleMember, agent: true},
}

// fieldSeed is one typed custom field, so the values the tasks carry render as
// fields rather than as free text.
type fieldSeed struct {
	project string
	def     core.FieldDefInput
}

var fieldDefs = []fieldSeed{
	{project: "infra", def: core.FieldDefInput{
		Key: "environment", Label: "Environment", Type: core.FieldEnum, Indexed: true, Position: 1,
		EnumOptions: []string{"production", "staging", "development"}}},
	{project: "infra", def: core.FieldDefInput{
		Key: "change_window", Label: "Change window", Type: core.FieldString, Position: 2}},
	{project: "ops", def: core.FieldDefInput{
		Key: "severity", Label: "Severity", Type: core.FieldEnum, Indexed: true, Position: 1,
		EnumOptions: []string{"sev1", "sev2", "sev3"}}},
	{project: "ops", def: core.FieldDefInput{
		Key: "paged", Label: "Woke somebody up", Type: core.FieldBool, Position: 2}},
	{project: "data", def: core.FieldDefInput{
		Key: "dataset", Label: "Dataset", Type: core.FieldString, Indexed: true, Position: 1}},
}

// commentSeed is one comment, by one actor, on one day.
type commentSeed struct {
	day  int
	hour int
	by   string
	body string
}

// taskSeed is one task and the whole life it had inside the window.
type taskSeed struct {
	key      string
	project  string
	title    string
	body     string
	tags     []string
	priority core.Priority
	assignee string
	creator  string

	created int
	hour    int
	due     int

	fields    map[string]any
	parent    string
	dependsOn []string

	end      outcome
	started  int
	done     int
	doneHour int
	doneBy   string

	comments []commentSeed
}

// abandonTTL is the lease each seeded agent takes before it stops answering.
const abandonTTL = 15 * time.Minute

// abandonSweepDelay is how long after a lease lapses the sweeper gets to it.
const abandonSweepDelay = time.Minute

// abandonRetryGap is the wait between one claim lapsing and the next attempt
// on the same task, so a repeatedly dying holder reads as a series of tries
// rather than as one long outage.
const abandonRetryGap = 40 * time.Minute

// abandonedSeed is a task an agent claimed and never gave back: the worker
// stopped answering, the lease ran out and the sweeper cleared it, leaving the
// evidence the list renders as "claim expired".
type abandonedSeed struct {
	key    string
	holder string
	// claims is how many times the task was picked up and dropped. Above one
	// is the signal the state exists for: a holder that keeps dying on the
	// same work.
	claims int
	// ago is how long before the end of the window the last claim lapsed. It
	// is measured from the end rather than from a fixture day because the
	// evidence only reads as recent for core.LeaseExpiryEvidenceWindow, and
	// the fixture days are backdated by months.
	ago time.Duration
}

// abandoned is deliberately short. Two dropped claims in a backlog of this
// size is what a fleet that mostly works looks like; a screen full of them
// would describe a different product.
//
// agents-audit carries the highest priority, so the default urgency ordering
// puts it on the first page of the task list. Both dropped claims used to sit
// on low-priority work, which left them past row fifty: the badge and the
// pager could not be seen at once, on any page. Work that matters is also
// what a fleet keeps picking up, so the urgent one is where a repeatedly
// dying holder belongs.
var abandoned = []abandonedSeed{
	{key: "agents-audit", holder: "scout", claims: 4, ago: 3 * time.Hour},
	{key: "agents-docsbot", holder: "mint", claims: 1, ago: 9 * time.Hour},
}
