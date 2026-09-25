// SPDX-License-Identifier: AGPL-3.0-or-later

package demo

import "github.com/heliopsy/tix/internal/core"

// tasks is the backlog the demo tenant is shown with: what was asked for, who
// picked it up, what they said about it and when it landed.
var tasks = []taskSeed{
	{
		key: "infra-ha", project: "infra",
		title: "Move the primary database onto a replicated pair",
		body: "The single Postgres instance is the last thing in the estate with no failover. " +
			"Stand up a synchronous replica, put pgbouncer in front of both and rehearse a promotion before we depend on it.",
		tags: []string{"database", "reliability"}, priority: core.PriorityHighest,
		assignee: "tom", creator: "nadia", created: 0, hour: 9, due: 4,
		fields: map[string]any{"environment": "production", "change_window": "Saturday 02:00-04:00 UTC"},
		end:    outcomeDone, started: 0, done: 2, doneHour: 16, doneBy: "tom",
		comments: []commentSeed{
			{day: 1, hour: 10, by: "nadia", body: "Promotion rehearsal went through in 40 seconds. Good enough to call this done once the runbook is written up."},
			{day: 2, hour: 15, by: "tom", body: "Replica is streaming with under a second of lag. Failover tested twice, both clean."},
		},
	},
	{
		key: "infra-tls", project: "infra",
		title: "Automate certificate renewal before the wildcard expires",
		body: "Two certificates were renewed by hand last quarter and one of them lapsed for eleven minutes. " +
			"Move everything onto cert-manager with a DNS-01 solver so renewal stops being somebody's calendar reminder.",
		tags: []string{"tls", "automation"}, priority: core.PriorityHigh,
		assignee: "lena", creator: "nadia", created: 5, hour: 8, due: 8,
		fields: map[string]any{"environment": "production"},
		end:    outcomeDone, started: 5, done: 5, doneHour: 15, doneBy: "lena",
		comments: []commentSeed{
			{day: 5, hour: 12, by: "lena", body: "DNS-01 is working against the staging issuer. Switching the production issuer over tomorrow morning."},
		},
	},
	{
		key: "infra-backups", project: "infra",
		title: "Verify nightly backups by restoring into a scratch cluster",
		body: "Backups have been running green for a year and nobody has ever restored one. " +
			"Add a nightly job that restores the most recent dump into a throwaway namespace and asserts a row count.",
		tags: []string{"backup", "reliability"}, priority: core.PriorityHigh,
		assignee: "atlas", creator: "tom", created: 2, hour: 8, due: 12,
		fields: map[string]any{"environment": "staging"},
		end:    outcomeDone, started: 3, done: 9, doneHour: 7, doneBy: "atlas",
		comments: []commentSeed{
			{day: 6, hour: 13, by: "atlas", body: "First restore failed on a missing extension. Added it to the scratch image and the second run passed."},
		},
	},
	{
		key: "infra-runners", project: "infra",
		title: "Right-size the CI runner pool",
		body: "Queue time went past twenty minutes during the European morning while the pool sat idle overnight. " +
			"Scale the pool on queue depth instead of a fixed count and cap the spend.",
		tags: []string{"ci", "cost"}, priority: core.PriorityNormal,
		assignee: "raj", creator: "tom", created: 6, hour: 10, due: 16,
		fields: map[string]any{"environment": "production"},
		end:    outcomeDone, started: 8, done: 13, doneHour: 15, doneBy: "raj",
		comments: []commentSeed{
			{day: 9, hour: 11, by: "tom", body: "Queue time is under four minutes on the morning peak now, and the overnight pool is empty. Spend is down about a third."},
		},
	},
	{
		key: "infra-secrets", project: "infra",
		title: "Rotate the long-lived deploy secrets",
		body: "Three service accounts still use keys minted in the first week of the project. " +
			"Rotate them, shorten the lifetime and record where each one is consumed so the next rotation is boring.",
		tags: []string{"security"}, priority: core.PriorityHigh,
		assignee: "nadia", creator: "lena", created: 10, hour: 9, due: 21,
		fields: map[string]any{"environment": "production", "change_window": "any weekday evening"},
		end:    outcomeDone, started: 12, done: 19, doneHour: 11, doneBy: "nadia",
		comments: []commentSeed{
			{day: 15, hour: 16, by: "nadia", body: "Found a fourth key in the image build pipeline that nobody had written down. Rotated it too."},
		},
	},
	{
		key: "infra-kernel", project: "infra",
		title: "Roll the node pool onto the patched kernel",
		body: "The published advisory affects the container runtime on every worker node. " +
			"Drain and replace nodes one at a time, keeping at least two thirds of capacity up throughout.",
		tags: []string{"security", "kubernetes"}, priority: core.PriorityHighest,
		assignee: "tom", creator: "nadia", created: 20, hour: 8, due: 30,
		fields: map[string]any{"environment": "production", "change_window": "rolling, business hours"},
		end:    outcomeDone, started: 21, done: 28, doneHour: 12, doneBy: "tom",
		comments: []commentSeed{
			{day: 24, hour: 15, by: "nadia", body: "Two nodes came back with the old kernel because the image tag was cached. Pinning the digest instead."},
		},
	},
	{
		key: "infra-observability", project: "infra",
		title: "Bring the platform onto one observability stack",
		body: "Metrics live in one system, logs in another and traces in a third, each with its own login. " +
			"Consolidate onto a single stack so an incident is one query rather than three tabs.",
		tags: []string{"observability", "epic"}, priority: core.PriorityHigh,
		assignee: "lena", creator: "nadia", created: 30, hour: 9, due: 100,
		fields: map[string]any{"environment": "production"},
		end:    outcomeDoing, started: 31,
		comments: []commentSeed{
			{day: 45, hour: 11, by: "nadia", body: "Metrics are migrated. Logs are the long pole here, mostly because of retention costs."},
		},
	},
	{
		key: "infra-metrics", project: "infra",
		title: "Ship node and pod metrics into the new stack",
		body: "Point the collectors at the new endpoint and keep the old one receiving in parallel for a fortnight. " +
			"Dashboards move only once both series agree.",
		tags: []string{"observability"}, priority: core.PriorityNormal,
		assignee: "atlas", creator: "lena", created: 31, hour: 10, due: 42,
		parent: "infra-observability",
		fields: map[string]any{"environment": "production"},
		end:    outcomeDone, started: 32, done: 37, doneHour: 9, doneBy: "atlas",
	},
	{
		key: "infra-logs", project: "infra",
		title: "Move log shipping off the legacy forwarder",
		body: "The forwarder is two major versions behind and drops lines under back pressure without saying so. " +
			"Replace it, and add a synthetic line per minute so silent loss is visible on a graph.",
		tags: []string{"observability", "logging"}, priority: core.PriorityNormal,
		assignee: "lena", creator: "lena", created: 32, hour: 14, due: 88,
		parent: "infra-observability",
		fields: map[string]any{"environment": "production"},
		end:    outcomeDoing, started: 34,
	},
	{
		key: "infra-cost", project: "infra",
		title: "Publish a monthly infrastructure cost report",
		body: "Spend is discussed once a quarter from a screenshot somebody took. " +
			"Generate a monthly breakdown per project and post it where the whole team can see it.",
		tags: []string{"cost", "reporting"}, priority: core.PriorityLow,
		assignee: "scout", creator: "raj", created: 12, hour: 13, due: 50,
		fields: map[string]any{"environment": "development"},
		end:    outcomeDone, started: 41, done: 46, doneHour: 10, doneBy: "scout",
	},
	{
		key: "infra-dns", project: "infra",
		title: "Move DNS to the new registrar",
		body: "The current registrar has no API worth automating against and two of our zones are edited by hand. " +
			"Transfer the zones, then manage records from the same repository as everything else.",
		tags: []string{"dns", "migration"}, priority: core.PriorityNormal,
		assignee: "raj", creator: "nadia", created: 55, hour: 10, due: 70,
		fields: map[string]any{"environment": "production"},
		end:    outcomeBlocked, started: 58,
		comments: []commentSeed{
			{day: 60, hour: 9, by: "raj", body: "Transfer is stuck behind a registrar lock that only the original account holder can lift. Chasing it."},
		},
	},
	{
		key: "infra-terraform", project: "infra",
		title: "Split the Terraform root module per environment",
		body: "One root module holds production and staging, so every staging change plans against production too. " +
			"Split it, keep the shared pieces as modules and give each environment its own state.",
		tags: []string{"terraform", "refactor"}, priority: core.PriorityNormal,
		assignee: "tom", creator: "tom", created: 64, hour: 11, due: 94,
		fields: map[string]any{"environment": "staging"},
		end:    outcomeDoing, started: 70,
		comments: []commentSeed{
			{day: 78, hour: 10, by: "tom", body: "Staging plan is clean against its own state. Production still has three resources that were created by hand and need importing."},
		},
	},
	{
		key: "infra-netpol", project: "infra",
		title: "Default-deny network policies in the staging namespace",
		body: "Everything can talk to everything, which means a compromised sidecar reaches the database directly. " +
			"Start with staging, write the allow rules from observed traffic, then repeat in production.",
		tags: []string{"security", "kubernetes"}, priority: core.PriorityHigh,
		assignee: "lena", creator: "nadia", created: 80, hour: 9, due: 97,
		fields: map[string]any{"environment": "staging"},
		end:    outcomeTodo,
	},
	{
		key: "infra-drill", project: "infra",
		title: "Run a disaster recovery drill in the secondary region",
		body: "The secondary region has never served traffic and the failover document has never been followed end to end. " +
			"Block out an afternoon, fail over deliberately and write down everything that surprised us.",
		tags: []string{"reliability", "drill"}, priority: core.PriorityNormal,
		assignee: "tom", creator: "nadia", created: 84, hour: 10, due: 101,
		fields: map[string]any{"environment": "production"},
		end:    outcomeTodo,
	},

	{
		key: "web-skeleton", project: "web",
		title: "Lay out the application shell and navigation",
		body: "Every screen currently reimplements its own header and sidebar, and they have already drifted apart. " +
			"Build one shell with the navigation, the project switcher and the theme toggle, and mount the pages inside it.",
		tags: []string{"ui", "foundation"}, priority: core.PriorityHighest,
		assignee: "nadia", creator: "nadia", created: 0, hour: 10, due: 6,
		end: outcomeDone, started: 1, done: 3, doneHour: 17, doneBy: "nadia",
		comments: []commentSeed{
			{day: 2, hour: 9, by: "lena", body: "The project switcher needs to remember the last project per person, otherwise everyone lands on the same list every morning."},
		},
	},
	{
		key: "web-board", project: "web",
		title: "Board view with drag and drop between columns",
		body: "The list view is fine for triage but useless for standup, where people want to see the columns fill up. " +
			"Render one column per workflow state and move a task by dropping it, writing the transition through the API.",
		tags: []string{"ui", "board"}, priority: core.PriorityHigh,
		assignee: "lena", creator: "nadia", created: 2, hour: 9, due: 10,
		end: outcomeDone, started: 3, done: 7, doneHour: 16, doneBy: "lena",
		comments: []commentSeed{
			{day: 5, hour: 14, by: "lena", body: "Drop target maths was off by the scroll offset on long columns. Fixed, and added a test for the scrolled case."},
		},
	},
	{
		key: "web-darkmode", project: "web",
		title: "Dark theme that follows the system preference",
		body: "Half the team works in a dark editor and then gets a white page when they open the board. " +
			"Derive both themes from one set of tokens and follow the system preference unless the person overrides it.",
		tags: []string{"ui", "theme"}, priority: core.PriorityNormal,
		assignee: "mint", creator: "lena", created: 11, hour: 8, due: 14,
		end: outcomeDone, started: 11, done: 11, doneHour: 15, doneBy: "mint",
	},
	{
		key: "web-filters", project: "web",
		title: "Saved filters in the task list",
		body: "People retype the same three filters every morning and then lose them on a refresh. " +
			"Let a filter be named and saved, and put the saved ones in the sidebar under the project.",
		tags: []string{"ui", "filters"}, priority: core.PriorityNormal,
		assignee: "raj", creator: "tom", created: 11, hour: 10, due: 20,
		end: outcomeDone, started: 12, done: 17, doneHour: 15, doneBy: "raj",
		comments: []commentSeed{
			{day: 14, hour: 16, by: "tom", body: "Saving a filter that includes a free text query is surprising when the query goes stale. Let us save the structured part only for now."},
		},
	},
	{
		key: "web-keyboard", project: "web",
		title: "Keyboard shortcuts for triage",
		body: "Triaging fifty tasks with a mouse is slow enough that people do it in the CLI instead. " +
			"Add j/k navigation, a shortcut to assign and one to transition, and a help overlay listing them.",
		tags: []string{"ui", "keyboard"}, priority: core.PriorityNormal,
		assignee: "tom", creator: "lena", created: 17, hour: 9, due: 26,
		end: outcomeDone, started: 18, done: 23, doneHour: 12, doneBy: "tom",
		comments: []commentSeed{
			{day: 20, hour: 15, by: "nadia", body: "Please keep the shortcuts off single letters that conflict with the browser's own find-as-you-type."},
		},
	},
	{
		key: "web-charts", project: "web",
		title: "Statistics page with throughput and lead time",
		body: "The numbers exist in the API and nowhere a person can see them. " +
			"Draw completions per day, the lead time distribution and the leaderboard, with a note saying what the leaderboard counts.",
		tags: []string{"ui", "stats"}, priority: core.PriorityHigh,
		assignee: "nadia", creator: "nadia", created: 22, hour: 10, due: 33,
		end: outcomeDone, started: 24, done: 30, doneHour: 16, doneBy: "nadia",
		comments: []commentSeed{
			{day: 27, hour: 11, by: "tom", body: "Careful with the empty state here. A leaderboard with one row and no caption reads like a ranking."},
		},
	},
	{
		key: "web-mobile", project: "web",
		title: "Make the task detail page usable on a phone",
		body: "The detail page overflows horizontally below about 420 pixels and the comment box is unreachable. " +
			"Reflow the metadata into a stacked layout and keep the composer pinned.",
		tags: []string{"ui", "responsive"}, priority: core.PriorityNormal,
		assignee: "lena", creator: "raj", created: 33, hour: 13, due: 44,
		end: outcomeDone, started: 35, done: 40, doneHour: 14, doneBy: "lena",
		comments: []commentSeed{
			{day: 36, hour: 11, by: "raj", body: "Tested on a small phone and the composer is reachable, but the status chips still wrap awkwardly at the narrowest width."},
		},
	},
	{
		key: "web-a11y", project: "web",
		title: "Fix the accessibility failures on the board",
		body: "The columns are divs with click handlers, so nothing works from a keyboard and the screen reader announces nothing useful. " +
			"Give the board real roles, a focus order and live-region announcements when a card moves.",
		tags: []string{"ui", "accessibility"}, priority: core.PriorityHigh,
		assignee: "scout", creator: "nadia", created: 20, hour: 9, due: 56,
		end: outcomeDone, started: 46, done: 52, doneHour: 11, doneBy: "scout",
	},
	{
		key: "web-search", project: "web",
		title: "Full text search across titles, bodies and comments",
		body: "Search matches titles only, so anything described in the body is unfindable. " +
			"Wire the search box to the engine's own index and show which field matched.",
		tags: []string{"ui", "search"}, priority: core.PriorityHigh,
		assignee: "raj", creator: "tom", created: 58, hour: 10, due: 78,
		end: outcomeDoing, started: 62,
	},
	{
		key: "web-attachments", project: "web",
		title: "Attach files to a task from the browser",
		body: "Screenshots get pasted into chat and then lost when somebody archives the channel. " +
			"Accept a drop onto the detail page, store it as an artifact and show a thumbnail inline.",
		tags: []string{"ui", "artifacts"}, priority: core.PriorityNormal,
		assignee: "lena", creator: "lena", created: 66, hour: 11, due: 92,
		end: outcomeDoing, started: 71,
	},
	{
		key: "web-realtime", project: "web",
		title: "Live updates on the board over the event stream",
		body: "Two people triaging at once overwrite each other's view until somebody refreshes. " +
			"Subscribe to the event stream and apply transitions as they arrive, with a subtle highlight on the moved card.",
		tags: []string{"ui", "realtime"}, priority: core.PriorityHigh,
		assignee: "tom", creator: "nadia", created: 70, hour: 9, due: 86,
		end: outcomeBlocked, started: 73,
		comments: []commentSeed{
			{day: 75, hour: 10, by: "tom", body: "Blocked on the reconnect semantics in the stream. Reopening once the backend decides what a resumed cursor means."},
		},
	},
	{
		key: "web-print", project: "web",
		title: "Printable sprint summary",
		body: "The weekly review is still a person reading a screen aloud. " +
			"Add a print stylesheet and a one-page summary of what closed, what slipped and what is blocked.",
		tags: []string{"ui", "reporting"}, priority: core.PriorityLow,
		assignee: "mint", creator: "raj", created: 79, hour: 14, due: noDue,
		end: outcomeTodo,
	},
	{
		key: "web-onboarding", project: "web",
		title: "First-run tour for a brand new tenant",
		body: "A new installation opens on an empty board with no indication of what to do next. " +
			"Show a short tour that creates the first project and the first task, and never show it again afterwards.",
		tags: []string{"ui", "onboarding"}, priority: core.PriorityLow,
		assignee: "nadia", creator: "nadia", created: 85, hour: 10, due: 99,
		end: outcomeTodo,
	},

	{
		key: "agents-claim", project: "agents",
		title: "Claim protocol with leases and heartbeats",
		body: "Two agents picked up the same task last week and both opened a change for it. " +
			"Give a claim a lease and a token, renew on a heartbeat and release automatically when the lease expires.",
		tags: []string{"protocol", "reliability"}, priority: core.PriorityHighest,
		assignee: "atlas", creator: "nadia", created: 8, hour: 9, due: 18,
		end: outcomeDone, started: 9, done: 15, doneHour: 10, doneBy: "atlas",
		comments: []commentSeed{
			{day: 12, hour: 16, by: "atlas", body: "Lease renewal is in. Sweeping expired leases every thirty seconds, which reverts the task to todo rather than leaving it stuck in doing."},
		},
	},
	{
		key: "agents-retry", project: "agents",
		title: "Back off and retry instead of failing the whole run",
		body: "A single upstream timeout aborts an agent run that had already done twenty minutes of useful work. " +
			"Retry the failed step with exponential backoff and record each attempt as an artifact.",
		tags: []string{"reliability"}, priority: core.PriorityHigh,
		assignee: "scout", creator: "raj", created: 21, hour: 8, due: 25,
		end: outcomeDone, started: 21, done: 21, doneHour: 15, doneBy: "scout",
	},
	{
		key: "agents-lease", project: "agents",
		title: "Report a stuck agent instead of silently holding the lease",
		body: "An agent that wedges keeps renewing its lease forever and the task never returns to the queue. " +
			"Bound the total hold time and emit an event when a run exceeds it.",
		tags: []string{"reliability", "observability"}, priority: core.PriorityHigh,
		assignee: "mint", creator: "atlas", created: 20, hour: 11, due: 30,
		dependsOn: []string{"agents-claim"},
		end:       outcomeDone, started: 21, done: 26, doneHour: 9, doneBy: "mint",
	},
	{
		key: "agents-budget", project: "agents",
		title: "Give every agent run a token budget",
		body: "One misconfigured loop spent more in an afternoon than the fleet usually spends in a week. " +
			"Attach a budget to each run, refuse to start without one and stop cleanly when it is exhausted.",
		tags: []string{"cost", "safety"}, priority: core.PriorityHighest,
		assignee: "atlas", creator: "nadia", created: 28, hour: 9, due: 38,
		end: outcomeDone, started: 29, done: 35, doneHour: 15, doneBy: "atlas",
		comments: []commentSeed{
			{day: 31, hour: 14, by: "nadia", body: "Budget should be per run and per day. A run that respects its own budget can still loop a hundred times."},
		},
	},
	{
		key: "agents-handoff", project: "agents",
		title: "Hand a task back to a human with the context intact",
		body: "When an agent gives up, the task returns to the queue with no record of what it tried. " +
			"Write the attempt summary as a comment and the raw trace as an artifact before releasing the lease.",
		tags: []string{"protocol", "ux"}, priority: core.PriorityHigh,
		assignee: "raj", creator: "lena", created: 40, hour: 10, due: 52,
		dependsOn: []string{"agents-claim"},
		end:       outcomeDone, started: 42, done: 49, doneHour: 12, doneBy: "raj",
	},
	{
		key: "agents-sandbox", project: "agents",
		title: "Run every agent in a sandbox with no ambient credentials",
		body: "Agents currently inherit the runner's environment, which includes a deploy key nobody meant to expose. " +
			"Start each run in a clean container and inject only the scoped token for the task it claimed.",
		tags: []string{"security", "safety"}, priority: core.PriorityHighest,
		assignee: "scout", creator: "nadia", created: 32, hour: 9, due: 66,
		end: outcomeDone, started: 56, done: 64, doneHour: 16, doneBy: "scout",
		comments: []commentSeed{
			{day: 59, hour: 11, by: "scout", body: "Sandbox is up. Two agents broke immediately because they were reading credentials from the environment, which is rather the point."},
		},
	},
	{
		key: "agents-queue", project: "agents",
		title: "Priority-aware claim ordering",
		body: "Agents take whatever is oldest, so a highest-priority incident task waits behind week-old chores. " +
			"Order the queue by priority first and age second, and make the ordering visible in the CLI.",
		tags: []string{"protocol"}, priority: core.PriorityHigh,
		assignee: "atlas", creator: "tom", created: 62, hour: 10, due: 83,
		end: outcomeDoing, started: 68,
		comments: []commentSeed{
			{day: 79, hour: 14, by: "atlas", body: "Ordering is in behind a flag. Waiting on a day of real traffic before turning it on for the whole fleet."},
		},
	},
	{
		key: "agents-eval", project: "agents",
		title: "Score agent output before it is merged",
		body: "Nobody can say whether the fleet is getting better or worse, because nothing is measured. " +
			"Run a small evaluation on each completed task and record the score alongside the run.",
		tags: []string{"quality", "metrics"}, priority: core.PriorityNormal,
		assignee: "mint", creator: "nadia", created: 72, hour: 11, due: 95,
		end: outcomeDoing, started: 77,
	},
	{
		key: "agents-ratelimit", project: "agents",
		title: "Rate limit the fleet against the upstream API",
		body: "Six agents working at once trip the upstream limit and every one of them retries into the same wall. " +
			"Share a token bucket across the fleet and queue rather than retry.",
		tags: []string{"reliability"}, priority: core.PriorityHigh,
		assignee: "atlas", creator: "raj", created: 76, hour: 9, due: 88,
		end: outcomeBlocked, started: 78,
		comments: []commentSeed{
			{day: 80, hour: 9, by: "raj", body: "Blocked until the shared bucket has somewhere to live. It cannot be in process memory once we run more than one scheduler."},
		},
	},
	{
		key: "agents-audit", project: "agents",
		title: "Make every agent action attributable in the audit log",
		body: "An agent acting through a shared token is indistinguishable from any other holder of it. " +
			"Mint one token per agent and record the token identity on every entry it writes.",
		tags: []string{"security", "audit"}, priority: core.PriorityNormal,
		assignee: "scout", creator: "nadia", created: 82, hour: 10, due: 98,
		end: outcomeTodo,
	},
	{
		key: "agents-docsbot", project: "agents",
		title: "Let the docs agent open tasks for stale pages",
		body: "Documentation rots quietly and only gets noticed when somebody follows it and fails. " +
			"Have the docs agent compare pages against the code they describe and file a task when they disagree.",
		tags: []string{"docs", "automation"}, priority: core.PriorityLow,
		assignee: "mint", creator: "lena", created: 86, hour: 13, due: noDue,
		end: outcomeTodo,
	},

	{
		key: "docs-quickstart", project: "docs",
		title: "Quickstart that gets somebody to their first task in two minutes",
		body: "The current getting-started page explains the architecture before it says how to install anything. " +
			"Rewrite it as install, first project, first task, and move the architecture to its own page.",
		tags: []string{"guide"}, priority: core.PriorityHigh,
		assignee: "mint", creator: "nadia", created: 24, hour: 10, due: 36,
		end: outcomeDone, started: 26, done: 33, doneHour: 14, doneBy: "mint",
		comments: []commentSeed{
			{day: 29, hour: 9, by: "nadia", body: "Tried the draft on somebody who had never seen the tool. They were stuck at the database path, so make that step explicit."},
		},
	},
	{
		key: "docs-api", project: "docs",
		title: "Reference page for the HTTP API",
		body: "The API is documented in the handler comments and nowhere a caller can read it. " +
			"Generate the reference from the same source as the routes so it cannot drift.",
		tags: []string{"reference", "api"}, priority: core.PriorityNormal,
		assignee: "mint", creator: "raj", created: 43, hour: 8, due: 48,
		end: outcomeDone, started: 43, done: 43, doneHour: 16, doneBy: "mint",
	},
	{
		key: "docs-selfhost", project: "docs",
		title: "Self-hosting guide with a worked example",
		body: "People ask the same four questions about reverse proxies, TLS and the database path every time. " +
			"Write one guide with a complete compose file and answer them in place.",
		tags: []string{"guide", "ops"}, priority: core.PriorityNormal,
		assignee: "nadia", creator: "tom", created: 26, hour: 9, due: 62,
		end: outcomeDone, started: 50, done: 58, doneHour: 15, doneBy: "nadia",
	},
	{
		key: "docs-cli", project: "docs",
		title: "Regenerate the CLI reference on every release",
		body: "The reference was generated by hand eight months ago and three commands have changed since. " +
			"Generate it in CI and fail the build when the checked-in copy differs.",
		tags: []string{"reference", "ci"}, priority: core.PriorityNormal,
		assignee: "mint", creator: "lena", created: 60, hour: 10, due: 74,
		end: outcomeDone, started: 62, done: 70, doneHour: 11, doneBy: "mint",
		comments: []commentSeed{
			{day: 65, hour: 13, by: "lena", body: "Generation works. The diff check needs to ignore the version line or every release will fail the build for no reason."},
		},
	},
	{
		key: "docs-agents", project: "docs",
		title: "Explain how an agent authenticates and claims work",
		body: "Everything about the agent protocol lives in the heads of two people and one design document. " +
			"Document token minting, scopes, the claim loop and what happens when a lease expires.",
		tags: []string{"guide", "agents"}, priority: core.PriorityHigh,
		assignee: "mint", creator: "nadia", created: 68, hour: 10, due: 84,
		dependsOn: []string{"agents-claim", "agents-handoff"},
		end:       outcomeDoing, started: 74,
	},
	{
		key: "docs-troubleshooting", project: "docs",
		title: "Troubleshooting page for the errors people actually hit",
		body: "The support channel answers the same locked-database and wrong-tenant questions every week. " +
			"Collect them into a page organised by the error message people see.",
		tags: []string{"guide", "support"}, priority: core.PriorityNormal,
		assignee: "lena", creator: "raj", created: 74, hour: 13, due: 89,
		end: outcomeDoing, started: 80,
		comments: []commentSeed{
			{day: 85, hour: 10, by: "lena", body: "Started from the last three months of the support channel. The locked database question alone came up eleven times."},
		},
	},
	{
		key: "docs-screenshots", project: "docs",
		title: "Refresh every screenshot in the documentation",
		body: "The screenshots predate the current shell and none of them match what a new user sees. " +
			"Retake them from a seeded demo database so they show a populated product.",
		tags: []string{"polish"}, priority: core.PriorityNormal,
		assignee: "nadia", creator: "nadia", created: 81, hour: 11, due: 96,
		end: outcomeTodo,
	},
	{
		key: "docs-changelog", project: "docs",
		title: "Write release notes a person would read",
		body: "The changelog is a list of commit subjects, which tells nobody what changed for them. " +
			"Group each release by what it means for a user and link the reference pages that changed.",
		tags: []string{"release"}, priority: core.PriorityLow,
		assignee: "raj", creator: "nadia", created: 87, hour: 14, due: noDue,
		end: outcomeTodo,
	},
	{
		key: "docs-i18n", project: "docs",
		title: "Decide whether the documentation gets translated",
		body: "Two contributors have offered translations and we have no answer about how they would be kept current. " +
			"Decide the policy before accepting the first one, and write the decision down.",
		tags: []string{"decision"}, priority: core.PriorityLowest,
		assignee: "lena", creator: "nadia", created: 72, hour: 15, due: 80,
		end: outcomeBlocked, started: 75,
	},

	{
		key: "ops-runbook", project: "ops",
		title: "Runbook for the checkout latency alert",
		body: "The alert has fired four times and been handled four different ways, twice by guessing. " +
			"Write the runbook from what actually worked, with the queries inline.",
		tags: []string{"oncall", "runbook"}, priority: core.PriorityHigh,
		assignee: "tom", creator: "nadia", created: 44, hour: 9, due: 58,
		fields: map[string]any{"severity": "sev2", "paged": true},
		end:    outcomeDone, started: 46, done: 55, doneHour: 13, doneBy: "tom",
		comments: []commentSeed{
			{day: 50, hour: 9, by: "tom", body: "Wrote it from the last incident rather than from memory, which turned up two queries nobody had written down."},
		},
	},
	{
		key: "ops-alerts", project: "ops",
		title: "Cut the alerts that nobody acts on",
		body: "On-call gets around thirty pages a week and acts on maybe four of them. " +
			"Review every alert, delete the ones with no action and downgrade the rest to a dashboard.",
		tags: []string{"oncall", "noise"}, priority: core.PriorityHigh,
		assignee: "raj", creator: "tom", created: 28, hour: 10, due: 64,
		fields: map[string]any{"severity": "sev3", "paged": false},
		end:    outcomeDone, started: 52, done: 61, doneHour: 16, doneBy: "raj",
		comments: []commentSeed{
			{day: 56, hour: 11, by: "tom", body: "Went from 31 to 9 pages a week on the sample I replayed. The disk one alone was a third of them."},
		},
	},
	{
		key: "ops-oncall", project: "ops",
		title: "Publish the on-call rota six weeks ahead",
		body: "People find out they are on call the Friday before, which makes it impossible to plan around. " +
			"Generate the rota from the team list and publish it to the shared calendar.",
		tags: []string{"oncall", "process"}, priority: core.PriorityNormal,
		assignee: "lena", creator: "nadia", created: 73, hour: 8, due: 76,
		fields: map[string]any{"severity": "sev3", "paged": false},
		end:    outcomeDone, started: 73, done: 73, doneHour: 15, doneBy: "lena",
	},
	{
		key: "ops-postmortem", project: "ops",
		title: "Postmortem for the 40 minute API outage",
		body: "A config push removed the readiness probe and every pod was rolled at once. " +
			"Write it up blamelessly, with the timeline, the contributing causes and the two changes that stop a repeat.",
		tags: []string{"incident", "postmortem"}, priority: core.PriorityHighest,
		assignee: "tom", creator: "nadia", created: 72, hour: 9, due: 78,
		fields: map[string]any{"severity": "sev1", "paged": true},
		end:    outcomeDone, started: 72, done: 76, doneHour: 15, doneBy: "tom",
		comments: []commentSeed{
			{day: 74, hour: 10, by: "nadia", body: "Keep the timeline in UTC and include the moment we noticed as well as the moment it started. The gap is the interesting part."},
			{day: 76, hour: 14, by: "tom", body: "Two actions out of it: a staged rollout for config, and an alert on readiness failures that fires before the rollout completes."},
		},
	},
	{
		key: "ops-chaos", project: "ops",
		title: "Kill a random pod every hour in staging",
		body: "Staging is more stable than production because nothing ever disrupts it, which hides every restart bug. " +
			"Introduce controlled disruption during working hours only, so somebody is around when it finds something.",
		tags: []string{"reliability", "chaos"}, priority: core.PriorityNormal,
		assignee: "atlas", creator: "tom", created: 70, hour: 13, due: 90,
		fields: map[string]any{"severity": "sev3", "paged": false},
		end:    outcomeDoing, started: 75,
	},
	{
		key: "ops-backpressure", project: "ops",
		title: "Shed load instead of falling over",
		body: "Under a burst the API accepts every request and then times out on all of them, which is the worst outcome. " +
			"Add a queue limit and return a clear retry-after once it is full.",
		tags: []string{"reliability"}, priority: core.PriorityHigh,
		assignee: "raj", creator: "nadia", created: 78, hour: 10, due: 92,
		fields: map[string]any{"severity": "sev2", "paged": false},
		end:    outcomeDoing, started: 83,
		comments: []commentSeed{
			{day: 86, hour: 15, by: "nadia", body: "Please make the retry-after honest. A fixed sixty seconds sends the whole herd back at the same moment."},
		},
	},
	{
		key: "ops-statuspage", project: "ops",
		title: "Stand up a public status page",
		body: "During the last outage the only signal to users was silence, and support fielded it by hand. " +
			"Put up a status page and make updating it part of the incident process.",
		tags: []string{"incident", "comms"}, priority: core.PriorityNormal,
		assignee: "lena", creator: "nadia", created: 83, hour: 11, due: 100,
		fields: map[string]any{"severity": "sev3", "paged": false},
		end:    outcomeTodo,
	},
	{
		key: "ops-capacity", project: "ops",
		title: "Capacity plan for the next two quarters",
		body: "Growth has been absorbed by overprovisioning and nobody knows how much headroom is left. " +
			"Model the current trend against the cluster limits and say when we need more.",
		tags: []string{"planning", "cost"}, priority: core.PriorityNormal,
		assignee: "nadia", creator: "nadia", created: 66, hour: 14, due: 79,
		fields: map[string]any{"severity": "sev3", "paged": false},
		end:    outcomeBlocked, started: 69,
	},
	{
		key: "ops-secrets-audit", project: "ops",
		title: "Audit who can read production secrets",
		body: "The access list has only ever grown and includes two people who left last year. " +
			"Review it, remove what is stale and set a quarterly reminder to do it again.",
		tags: []string{"security", "process"}, priority: core.PriorityHigh,
		assignee: "tom", creator: "nadia", created: 88, hour: 9, due: 97,
		fields: map[string]any{"severity": "sev2", "paged": false},
		end:    outcomeTodo,
	},

	{
		key: "data-warehouse", project: "data",
		title: "Stand up the warehouse and load the first three tables",
		body: "Every report is a query against a production replica, which is both slow and risky. " +
			"Stand up the warehouse, load orders, customers and events, and point one report at it.",
		tags: []string{"warehouse", "foundation"}, priority: core.PriorityHigh,
		assignee: "raj", creator: "nadia", created: 30, hour: 9, due: 70,
		fields: map[string]any{"dataset": "core"},
		end:    outcomeDone, started: 56, done: 67, doneHour: 12, doneBy: "raj",
	},
	{
		key: "data-etl", project: "data",
		title: "Replace the nightly dump with incremental loads",
		body: "The full dump takes six hours and is already the longest thing in the night. " +
			"Move to change-data-capture for the three largest tables and keep the dump as a weekly fallback.",
		tags: []string{"pipeline"}, priority: core.PriorityHigh,
		assignee: "atlas", creator: "raj", created: 68, hour: 10, due: 82,
		fields:    map[string]any{"dataset": "orders"},
		dependsOn: []string{"data-warehouse"},
		end:       outcomeDone, started: 70, done: 79, doneHour: 8, doneBy: "atlas",
		comments: []commentSeed{
			{day: 73, hour: 15, by: "atlas", body: "Incremental load is down to eleven minutes. The weekly full run now finishes before anybody is awake."},
		},
	},
	{
		key: "data-dbt", project: "data",
		title: "Model the reporting layer with tests on every model",
		body: "Reports each define their own joins and two of them disagree about what an active customer is. " +
			"Define the models once, test uniqueness and referential integrity, and have the reports read the models.",
		tags: []string{"modelling", "quality"}, priority: core.PriorityNormal,
		assignee: "nadia", creator: "raj", created: 71, hour: 11, due: 85,
		fields:    map[string]any{"dataset": "reporting"},
		dependsOn: []string{"data-warehouse"},
		end:       outcomeDone, started: 73, done: 82, doneHour: 14, doneBy: "nadia",
		comments: []commentSeed{
			{day: 77, hour: 10, by: "raj", body: "Uniqueness test on the customer key failed immediately, which is exactly the disagreement the two reports were built on."},
		},
	},
	{
		key: "data-quality", project: "data",
		title: "Alert when a pipeline lands late or lands empty",
		body: "A pipeline failed silently for three days and the dashboards just showed a flat line. " +
			"Assert freshness and row counts after each load and page the owner when either fails.",
		tags: []string{"pipeline", "quality"}, priority: core.PriorityHigh,
		assignee: "lena", creator: "raj", created: 76, hour: 9, due: 88,
		fields: map[string]any{"dataset": "orders"},
		end:    outcomeDone, started: 78, done: 84, doneHour: 10, doneBy: "lena",
		comments: []commentSeed{
			{day: 81, hour: 16, by: "lena", body: "Freshness checks are live on the three big loads. Row count assertions need a baseline first, so those land tomorrow."},
		},
	},
	{
		key: "data-lineage", project: "data",
		title: "Show where each reported number comes from",
		body: "When a figure looks wrong, tracing it back to a source table takes an afternoon of reading SQL. " +
			"Capture lineage during the build and expose it next to each model.",
		tags: []string{"modelling"}, priority: core.PriorityNormal,
		assignee: "scout", creator: "nadia", created: 80, hour: 10, due: 90,
		fields: map[string]any{"dataset": "reporting"},
		end:    outcomeDone, started: 81, done: 86, doneHour: 13, doneBy: "scout",
	},
	{
		key: "data-export", project: "data",
		title: "Let people export a filtered task list as CSV",
		body: "Two teams keep asking for the same extract and somebody runs it by hand each time. " +
			"Expose the export behind the existing filters so they can take it themselves.",
		tags: []string{"reporting", "self-service"}, priority: core.PriorityNormal,
		assignee: "mint", creator: "lena", created: 88, hour: 8, due: 90,
		fields: map[string]any{"dataset": "reporting"},
		end:    outcomeDone, started: 88, done: 88, doneHour: 16, doneBy: "mint",
	},
	{
		key: "data-retention", project: "data",
		title: "Apply a retention policy to the event tables",
		body: "Raw events are kept forever and are now the largest thing in the warehouse by an order of magnitude. " +
			"Agree a retention period, aggregate what is older and delete the raw rows.",
		tags: []string{"cost", "retention"}, priority: core.PriorityNormal,
		assignee: "raj", creator: "nadia", created: 82, hour: 13, due: 96,
		fields: map[string]any{"dataset": "events"},
		end:    outcomeDoing, started: 86,
		comments: []commentSeed{
			{day: 87, hour: 11, by: "raj", body: "Ninety days of raw events and monthly aggregates beyond that has been agreed. Writing the deletion job now."},
		},
	},
	{
		key: "data-pii", project: "data",
		title: "Mask personal data in the analytics copies",
		body: "Analysts have full names and email addresses in a dataset none of them needs them in. " +
			"Hash the identifiers at load time and keep the mapping in the one place that is allowed to hold it.",
		tags: []string{"security", "privacy"}, priority: core.PriorityHighest,
		assignee: "nadia", creator: "nadia", created: 84, hour: 10, due: 94,
		fields: map[string]any{"dataset": "customers"},
		end:    outcomeTodo,
	},
	{
		key: "data-selfserve", project: "data",
		title: "Give the team a query interface that is not production",
		body: "People run exploratory queries against the replica because it is the only thing they have credentials for. " +
			"Point a query console at the warehouse and take the replica credentials away.",
		tags: []string{"self-service"}, priority: core.PriorityNormal,
		assignee: "tom", creator: "raj", created: 87, hour: 11, due: 99,
		fields: map[string]any{"dataset": "core"},
		end:    outcomeTodo,
	},
	{
		key: "data-costs", project: "data",
		title: "Attribute warehouse spend to the teams that query it",
		body: "The warehouse bill is a single line item and every team assumes it is somebody else's. " +
			"Tag queries by team and break the bill down monthly.",
		tags: []string{"cost", "reporting"}, priority: core.PriorityLow,
		assignee: "raj", creator: "nadia", created: 79, hour: 15, due: 93,
		fields: map[string]any{"dataset": "billing"},
		end:    outcomeBlocked, started: 82,
	},
}
