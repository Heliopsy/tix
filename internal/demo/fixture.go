// SPDX-License-Identifier: AGPL-3.0-or-later

package demo

import "github.com/heliopsy/tix/internal/core"

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
}

var people = []personSeed{
	{handle: "nadia", name: "Nadia Petrova", email: "nadia@demo.invalid", role: core.RoleAdmin},
	{handle: "tom", name: "Tom Okafor", email: "tom@demo.invalid", role: core.RoleMember},
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
