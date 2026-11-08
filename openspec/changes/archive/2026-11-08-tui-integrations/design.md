# Design

## One view with tabs, not three views

Webhooks, sync and retention need three different scopes, which argues for three views. They lost to one
view for two reasons.

Keys. Every capital worth having is already a cross-view key, and three more would have used up the last
of them for screens most readers never open. One key, `J`, opens the view, and the tabs inside it reuse
the board's own `←`/`→`, which the view has no other use for.

Gating still holds. `capability.TUIAccess` offers the view when the reader may perform any read bound to
it, and inside the view each tab is offered only when its own read is permitted. A reader holding only
`sync:admin` opens the view on the sync tab and never sees the other two named. There is no second
permission table: the tab asks the same `mayPerform` the footer does, which reads the registry.

## Self-contained

Other views are landing in parallel, so the shared files carry as little as possible: one field on
`Model` holding the whole integrations state, one case in each of the view, key, message and form
switches, and one confirmation kind per destructive call. The rendering is a pure function of a state
value, `IntegrationsView`, so it is tested without a model, as the tenant screen is.

## Reading

Opening the view, refreshing it and every write that lands read all of the tabs the reader may see in
one command, each read carrying its own refusal. Deliveries are read as one page of 50, newest first; a
log running past the page says so and names `tix webhook deliveries` for the rest, rather than adding a
pager to a terminal tab.

## Writing

The forms reuse the existing form widget. Two small additions make them possible:

- Secret fields are text fields with the form's shared `Secret` flag, drawn masked and gathered by the
  single-line input in password mode. They are never seeded, so a stored secret cannot reach the screen,
  and a blank answer keeps what is stored, which is what the service already does with an empty secret
  or credential.
- `FieldCondition.Not`, so a field can be shown for every system but one. Generic sync sources take a file
  and the two trackers take a URL, a project and a query; the form shows only what the chosen system
  accepts, and the service still refuses anything else.

Each form maps onto the same input the CLI builds: `WebhookInput`, `SyncSourceInput`, `RunSyncInput`,
`RetentionPolicy` and `PruneInput`. Durations are read by `core.ParseDuration`, the grammar the web form and
the CLI read.

## Destructive calls

Deleting an endpoint and deleting a source ask first, naming the URL or the source. A prune asks first only
when it writes; a dry run is the default and reports counts on the tab. A sync run writes tasks but does not
delete anything, and dry run is the form's default, so it is not held behind a second question, matching the
browser. Redelivering queues a copy of an event and is not destructive either.

## Results stay on the tab

A generated signing secret, a sync result or check, and a prune report are drawn on the tab that produced
them until the reader leaves the view, because the status bar holds one line and a sync result is several.
