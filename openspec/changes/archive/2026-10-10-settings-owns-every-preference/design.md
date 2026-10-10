# Design

## The problem: two controls for one value

Putting a preference on the settings screen that is already editable from the screen it affects
creates two editors for one cookie. Two editors for one value is how the two come to disagree, and
the disagreement is not usually a bug anybody writes on purpose. It arrives as drift:

- Two defaults. One control falls back to "show everything", the other to the declared defaults, and
  a reader who has never chosen sees different answers on the two screens.
- Two validations. One drops an unknown key, the other keeps it, so what the form round-trips depends
  on which form you used.
- Two readings. One writes the cookie and the other reads something adjacent -- a view field
  populated separately, a second call to the resolver with a different argument -- so a value is
  stored and never shown, or shown and never stored.
- Two labels for one thing, which is the cheapest to produce and the hardest to notice.

The test that would catch the third of these only catches it if it asserts the *rendered state of the
other control*. A test that writes the cookie through one form and then reads the cookie back passes
over exactly the failure being prevented, because a cookie both controls write and only one reads is
still a cookie with the right value in it.

## Three options, and which was taken

**One shared partial rendered in both places, reading one state.** Taken.

**A live summary plus a reset on settings, the editor left on the listing.** Rejected. It cannot
disagree, because there is only one editor, and it is cheap. It also cannot express "hide the Updated
column on Tokens", which is what a reader opening Settings to change a column wants to do. The user
asked for the screen to own every preference; a read-only mirror does not own anything. Reset-only
also makes the screen's answer to "can I change this here" a qualified no, which is the state the
change is undoing.

**The contextual control becomes a link to Settings.** Rejected, and this is the one worth arguing
about. It is the cleanest possible answer to the drift problem: one control, one place, nothing to
keep in step. It is rejected because of where the two preferences that matter are used. Choosing
which projects a task list shows is a thing you do *while looking at the task list* -- you tick, you
apply, the list under the control redraws, and you tick again. Sending that to another screen and
back turns one decision into three navigations, and the same is true of a column picker: the only way
to know whether a listing reads better without the Updated column is to look at the listing. The
contextual control is not a convenience here, it is where the decision is actually made.

So the design keeps both and makes "both" mean "one".

## How "one control in two places" is enforced

Four things are singular, and that is the whole mechanism:

| Singular | Where |
| --- | --- |
| The state | `prefsFor` in `internal/web/prefs.go`, the only constructor of `prefsView`, `columnFormView`, `visibilityFormView` and `taskViewFormView` |
| The form | `column-choices`, `project-choices` and `taskview-switch` in `templates/partials.html`, one `{{define}}` each |
| The reader | `columnsOf`, `hiddenProjects` and `taskViewOf`, called from `prefsFor` and nowhere a template can reach |
| The handler | `setColumns`, `setVisibility` and `setTaskView`, unchanged, one route each |

Each partial takes its own small view type rather than the page's `view`, which is what makes the
singularity structural rather than a convention. A template that wanted to render a column picker
differently would need a second value to render it against, and the only thing that produces one is
`newColumnForm`. The screen a listing sits on gets its picker through `view.ColumnForm()`, which
calls that same constructor; the settings screen gets one per listing from the same call in a loop.

The task screen no longer resolves the view a second time: it reads `prefs.TaskView.Current`, so the
board it decides to build and the position the switch shows are one answer rather than two calls that
happen to agree.

### The guard

`TestAPreferenceChangedInContextIsReflectedOnSettings` changes each of the three on the screen that
uses it and asserts the settings screen's rendered control: `aria-current` on the right segment of
the switch, `checked` on the right project checkbox, `checked` on the right column checkbox. It never
looks at a cookie. `TestAPreferenceChangedOnSettingsReachesTheScreenThatUsesIt` is the same journey
in the other direction, and asserts the effect as well as the control -- the board is drawn, the
listing's `<thead>` has lost the column, the task screen accounts for the hidden project.

Both read one control out of the page rather than searching the document: `prefForm` cuts the form
by its action, `columnPicker` cuts one picker by the hidden field naming its listing, `tableHead`
cuts the header row. A settings screen holding seven column pickers is exactly the page where a
document-wide substring search passes while the section under test is wrong.

## `tix_columns`: one section per listing

The preference is a map from listing to hidden columns. A single global control cannot express it:
the listings do not share a vocabulary (`Tasks` has `assignee` and `due`, `Tokens` has `scopes` and
`expires`), and the columns worth keeping differ per listing, which is why the preference is per
listing in the first place.

So settings renders one `<details>` per listing, each holding that listing's own picker. The
alternative -- a summary line per listing with a Reset -- was rejected for the reason above: it tells
a reader what they chose and refuses to let them choose.

The order and the words come from `columnPages`, which is a second list beside `columnSets`, and a
second list is the thing AGENTS.md says rots. `TestSettingsOffersEveryListingsColumns` holds them
together: it reads the listings off `columnSets` through `ColumnListings()` and insists the screen
carries a picker for each, then counts the pickers so a listing cannot be offered twice or left out.
`columnPages` therefore decides only naming and order, which is judgement, and judgement stays
reviewed.

## `tix_hidden_projects`: naming projects, and keys that name nothing

On the task screen the key alone was readable, because the rows underneath carry the project names.
On settings there is nothing to decode a key against, so the shared control renders the project's
name and its key. Both screens get the name, which is a change to the task screen too, and an
improvement there: a reader who joined last week knows "Platform Infrastructure" and not `infra`.

A hidden key naming a project that no longer exists is the case worth getting right, and it was
worse than cosmetic:

1. The control renders a checkbox per *live* project, so a stale key has no checkbox and cannot be
   unticked.
2. The key was still appended to `filter.Exclude.ProjectKeys`, and the service refuses an unknown
   project key as not found -- deliberately, because in a filter an unknown key is a typo.
3. So the task screen answered with a not-found page, and the control that would have cleared the
   cookie was on the screen that no longer rendered. The cookie was unreachable from the interface.

Two changes. `liveHiddenKeys` filters the exclusion to keys a project answers to, so what no longer
exists excludes nothing. `staleHiddenProjects` names the rest, and the control says so in a sentence,
because the alternative is a task list one project shorter with nothing on the page accounting for it.
Submitting the form forgets them, since what is stored is derived from the projects the form offered
-- so the sentence is also the fix, and needs no button of its own.

An archived project reads as stale, because the control lists unarchived projects, which is what it
listed before. The sentence says "no project on this list answers to it" rather than claiming
deletion, which is what the screen can actually prove.

## `tix_task_view`: still two states

The cookie has two values: `board`, and everything else -- absent, empty, stale, or something another
program left on the host -- which is the list. `taskViewOf` collapses them, `setTaskView` stores
nothing for the list and expires the cookie, and the switch has two positions.

The settings control is the same partial, so it has the same two positions. No "follow the default"
option was added, and none should be: it would be a third state that means the same as one of the
two, and the two screens would then disagree about which of the two it means.

## Where a preference lives, on the other surfaces

The terminal interface has a settings screen too, and it does **not** write to these cookies or to
anything like them. `internal/tui/settings.go` writes the CLI's own configuration keys --
`tui.keymap`, `output.time_format`, `output.timezone`, `output.color`, `tui.motion` -- so a format
chosen there is the format `tix task ls` prints. That is deliberate and documented in that file: a
browser has only a cookie, while somebody at a terminal already has a configuration file, and a
second place to write a timezone down would be a second place to disagree with the command line.

The consequence is a genuine divergence, and the documentation now states it rather than leaving a
reader to infer that one Settings screen governs both:

| Preference | Browser | Terminal / CLI |
| --- | --- | --- |
| Date format | `tix_time_format` cookie | `output.time_format` |
| Timezone | `tix_timezone` cookie | `output.timezone` |
| Keyboard scheme | `tix_keyscheme` cookie | `tui.keymap` |
| Colour scheme | `tix_theme` cookie | `output.color` (a different question: colour on or off, not which palette) |
| Advanced screens, drag to move, columns, hidden projects, task view | cookie | no equivalent |
| Row pulse | no equivalent | `tui.motion` |

Setting the timezone in the browser does not change what `tix task ls` prints, and setting it in the
terminal does not change what the browser renders. Unifying them was considered and not done here: it
is a change to where a preference is stored, not to which screen owns it, and it would mean a signed-in
browser writing to a server-side per-actor record that does not exist yet. It is named as a
divergence instead of being papered over.

## The three preferences that had no explained home

Two of the three named in the request already had a control on the settings screen, placed there when
the sidebar disclosure was retired; what they lacked was not a home but a reviewer checking. The
third does not exist. All three are recorded here because a switch whose label was guessed is worse
than no switch.

**`tix_advanced`** decides whether the Configure and Data groups are in the sidebar: workflows,
tenant, domains, users, tokens, SSH keys, status, connections, webhooks, import/export and sync. It
has three stored states, not two -- `1` shown, `0` hidden, absent -- and absent follows the reader: a
tenant administrator gets the groups, anybody else does not, because every screen in them refuses a
non-administrator and an entry leading to a refusal is worse than no entry. Being on one of those
screens expands its group whatever the preference says. The existing label and help text say this.

**`tix_drag_move`** decides whether a board card can be dragged between columns. Its polarity is the
opposite of `tix_advanced`: absent, or any value other than `0`, means on. Each card's own Move
disclosure is always present regardless, because dragging is unreachable from a keyboard and unusable
for anyone who cannot hold a pointer gesture, so turning the preference off costs nothing. The
existing label and help text say this.

**`tix_sync_lower`** is not a preference. It is not a cookie, it is not read anywhere, and no handler
writes it. The string occurs exactly once in the repository -- `internal/web/sync_internal_test.go`,
in the list of values `envNameSafe` must reject, where its job is to be a lowercase name. Nothing was
labelled, and nothing should be.

## Why the settings screen uses `muted` help and not `field-info`

`field-info` is the small "i" disclosure a form field's label carries when its meaning is not obvious
from the label: a workflow migration, a Go duration string. It belongs next to a label inside a dense
form, which is how `workflow.html`, `tenant.html` and `tokens.html` use it.

The settings screen is not a dense form. Every control already carries an open `<p class="muted">`
with an `id` the control points at through `aria-describedby`, visible without being opened, because
on a screen whose entire subject is explaining nine preferences the explanation should not be nine
things to click. The new sections follow the screen they are on rather than the convention of other
screens, and the difference is deliberate.

## What a reviewer would want to argue with

- **The settings screen got long.** Seven column pickers, each a disclosure, under one paragraph.
  They are collapsed and the alternative was a control that cannot change anything.
- **The task screen's project control changed too.** One partial means a change for settings is a
  change for both. Showing the name beside the key is the change, and it is defensible on its own;
  it is still a change made to a screen nobody complained about.
- **Settings now lists projects**, which costs the same walk the task screen pays
  (`allProjects`, up to 40 pages of `MaxPageLimit`). A settings screen that was previously free of
  service calls beyond `WhoAmI` now makes one. The alternative is a visibility control that cannot
  name what it governs.
- **`ColumnListings()` is exported for a test.** It reads a package-private map, and nothing in the
  product calls it. It is exported because the guard lives in `web_test`, which the rest of this
  package's screen tests also do.
- **`liveHiddenKeys` fixes a not-found bug in the same change.** It could have been its own commit.
  It is here because the stale-key guard this change was asked for is what found it, and the guard
  cannot pass without it.
