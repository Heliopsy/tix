# Tasks

## Web

- [x] `web`: scope the completion animation to the row that was acted on
- [x] test: a rendered list of completed tasks animates nothing
- [x] test: completing a task animates exactly one mark

## Theming

- [x] `docs/theming.md`: `tix tenant edit`, not `tix tenant set`
- [x] `docs/theming.md`: an unnamed theme resolves to the default, not a derived colour
- [x] `cmd/tenant.go`: the `--theme` flag help says the same
- [x] spec: record the default-theme behaviour as a modified requirement

## External sync

- [x] `service/sync.go`: a failed full refresh leaves the stored watermark alone
- [x] `service/sync.go`: the audit entry carries the watermark the run reached
- [x] `core`: an upstream-failure error kind, mapped to an exit code and an HTTP status
- [x] `service/sync.go`: `fail` reports upstream, not internal
- [x] `output`: render `SyncResult` in table, json and yaml
- [x] tests for each of the four

## Demo seed

- [x] `internal/demo` + `cmd/demo.go`: history replayed through the service on a steppable clock
- [x] descriptions, tags, due dates, assignees and custom fields
- [x] `internal/demo`: sign-in accounts on two of the seeded people, created through `Service.CreateUser`
- [x] `internal/demo`: `--reset` keeps the accounts it cannot recreate, and a re-seed re-credentials them
- [x] `cmd/demo.go`: `--password`, and the credentials reported in table, json and yaml
- [x] guard: a freshly seeded and a re-seeded database both authenticate the reported credentials
- [x] `internal/demo`: completions cluster into bursts and quiet stretches, uneven at 7, 14, 30 and 90 days
- [x] `internal/demo`: an abandoned claim sits on urgent work, so it lands on the first page of the default sort
- [ ] screenshots regenerate from the new seed

## Durations

- [x] `core.Duration.Human`, shared by the browser, command line and terminal
- [x] guard: the statistics table prints no machine-rendered duration
- [x] `core.ParseDuration`: Go syntax plus a day unit and spaced terms, so `Human` output is valid input
- [x] the command line, the HTTP window, the configuration reader and both unmarshal paths share it
- [x] `String` and `MarshalJSON` stay Go syntax, because snapshots round-trip through them
- [x] guard: for a spread of magnitudes, parsing `Human` output and rendering it again is identical text
- [ ] web: the retention fields read and show `30d` through `core.ParseDuration` and `Human`

## Honest output

- [x] a failed listing writes no document: no `[]` on stdout beside an error on stderr
- [x] a partial bracketed listing is left unterminated; a record stream keeps its lines
- [x] guard: every output format, empty stdout on a listing that fails

## Timezones

- [x] the binary embeds the zone database, so a base image without tzdata cannot silently
      downgrade a reader's timezone to the deployment default
- [x] guard: the import is pinned; `just release-dry` still builds all six `CGO_ENABLED=0` targets

## Web, found while reviewing the demo

- [x] the tick mark clears itself; `.is-target` keeps its deep-link meaning
- [x] a way back to the listing a task was opened from
- [x] the assignee field shows a handle, not an identifier
- [x] a Clear control on the filter, and the project filter says when it overrides visibility
- [x] lease badges: held, and claim expired
- [x] an expired claim leaves evidence on the task, so the sweep does not erase the one signal
      that says a holder took the work and stopped answering
- [x] the date says what it is; the reference reads as an identifier
- [x] Columns and Projects merged into one View panel
- [x] the status pill is a control, offering the workflow's own moves
- [x] assets carry a cache validator so an upgrade cannot serve a stale stylesheet
- [x] the status panel is a popover, so a listing's overflow cannot clip it
- [x] multi-step moves, route shown first, one audit entry per step
- [x] per-browser date format and timezone
- [x] `ListActors` end to end, and an assignee picker that suggests without constraining

## Web, second review pass

- [x] one pager partial, styled, used by all six keyset listings, with a position indicator
- [x] Previous, by way of a validated and bounded cursor trail in the URL; no change to core, store or API
- [x] the task list says who each task is assigned to
- [x] task dependencies read as reference and title, linked, not as raw identifiers
- [x] state categories get display labels at the presentation layer; the values do not move
- [x] a lease expiry is marked apart from every other kind of history entry
- [x] the sidebar background follows the grid rather than the viewport
- [x] the configuration groups default on for a tenant administrator, and a hidden screen expands its group
- [x] the expired-claim badge reads the durable evidence, says when and whose, and carries the claim count

## Assignee references

- [x] a handle resolves to an actor everywhere `--assignee` is taken: `task add`, `task ls`, `task edit`,
      the HTTP filter and the browser filter bar
- [x] resolution lives in `internal/service`, so every transport obeys one rule
- [x] an unknown assignee fails the listing as not found instead of answering with an empty list and a
      zero exit status
- [x] guard: an unknown handle in a filter must not produce an empty success

## Column preferences and the filter bar

- [x] the column cookie records the columns each listing hides, matching project visibility
- [x] a value in the earlier shown-set form is recognised by the absence of a version segment and
      converted, so no reader's choice is inverted and none is thrown away
- [x] guard: a column declared after a stored choice was written is shown without the cookie changing
- [x] guard: the widest choice the picker can produce fits the size the build reads back
- [x] a refused filter reports beside the filter box with the expression still in it, instead of
      replacing the listing with the error page
- [x] the view panel sizes its two halves by what they hold, and the rule between them follows the
      arrangement rather than a viewport width

## The rest of the filter, after the assignee

- [x] `service`: one resolver for every reference-shaped filter term, memoised across inclusion and
      exclusion, shape deciding identifier from name so an identifier costs no lookup
- [x] an unknown project key or identifier fails the listing; a key in any case resolves
- [x] a status is checked against the union of every workflow's states, so a cross-project listing naming
      a status only one workflow defines is answered rather than refused
- [x] a parent term resolves the reference form the interfaces display, not just an identifier
- [x] creator and claimant obey the assignee rule
- [x] a tag stays free-form: an unapplied tag is an empty page, not an error, and the reasoning is in the
      spec so it is not closed by tidiness later
- [x] guard: every refusal, watched to fail with the resolver broken
- [x] guard: the queries the rule must not refuse, so a narrower status check cannot ship
- [x] `claim next` obeys the same status rule: an undefined status is not an idle queue

## The release photographs

- [x] the task listing is named for what it renders: "Tasks" in the heading and in the navigation, since
      the page carries every actor's rows and `assignee:<handle>` already says "mine" precisely
- [x] guard: the heading read out of the h1 and the entry read out of the navigation, each watched to
      fail on its own so neither can be corrected while the other still claims otherwise
- [x] a retention window renders as `30d`, and the field parses what it renders through
      `core.ParseDuration` rather than `time.ParseDuration`, which knows no day
- [x] guard: the value the screen printed, submitted back and accepted, watched to fail both with the
      rendering reverted and with the parser reverted
- [x] guard: the refusals use values the wider grammar still rejects, since `720h` now passes
- [x] every row of the tenant tree carries a figure or says why it has none, a failed count included
- [x] the prune notice sits in a field-label row with the checkbox it explains, instead of falling onto
      a line of its own as an icon with no text
- [x] the tenant key is a labelled read-only field with the notice idiom its neighbours use, instead of
      a bare "Key default" paragraph between the theme help and the Save button
- [x] guard: the label, the notice, the read-only rendering and the absence of a name, each watched to
      fail on its own, plus the rule that paints a read-only field unlike an editable one
- [x] the task screen's history spans both columns below them, and its two short columns are sized down,
      so the one column holding a sentence stops wrapping on every row beside an empty gutter
- [x] guard: the panel's own tag, its position after the rail's close and the sizing rule, watched to
      fail against the previous markup
- [x] the task list no longer autofocuses its quick-add box: the shortcut scheme already binds a key to
      it, and focus on arrival took the screen away from the rows it exists to show
- [x] guard: the box read as an element rather than the page, watched to fail with autofocus restored
      and with the shortcut target removed
- [ ] the tenant screen still fetches a tenant listing it never renders; dropping it also needs the
      `tenant.list` web binding removed from `internal/capability`, which this change does not own

## Release

- [ ] `just ci` green
- [ ] docs and ROADMAP updated
- [ ] archive this change
