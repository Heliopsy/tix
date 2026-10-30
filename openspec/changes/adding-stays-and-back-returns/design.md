# Design

## Changing the default without overriding a choice

`tix_open_after_add` was read as "absent or anything but `0` opens the task", and the toggle wrote
`0` for off and an empty value for on. Three populations exist:

| Stored | How it got there | New reading |
| --- | --- | --- |
| no cookie | never touched the preference | stays on the list (the new default) |
| `0` | turned it off | stays on the list |
| empty value | turned it off, then on again | opens the task |

The empty value is the only trace of an explicit "on", so it is honoured: `openAfterAdd` is now
"present and not `0`". Only browsers that never chose change behaviour, which is the migration asked
for, and it needs no server-side state because the preference never had any. The toggle now writes
`1` and `0`, so no future reading depends on the empty value.

The settings screen's text is left as it is: it describes what each position does, not which is
the default, and that screen is being redesigned separately.

## Where back goes

In order: the `next` the link carried; the referring page when it is a listing (`/tasks` or one
project's board), minus any flash; the `next` of the referring page when that page is this same task
screen; otherwise `/tasks`. The referrer is the same-origin address the browser already sends, so no
cookie is added to remember the listing. `taskPath`, the redirect every task-screen mutation answers
with, appends the resolved `next`, which is what keeps the link after the second save as well as the
first. `next` still goes through `safeNext`.

The flash and the `flashref` a redirect leaves are stripped from every address that is carried
forward (`here`, `screenHref`, `fragmentHref`, the create referrer), so adding twice in a row cannot
carry the first task's toast into the second.

## Marking the new row

A class, `is-flashed`, on the row or card whose reference is the page's `flashref`, rather than
`:target`. The body is boosted, and a boosted redirect never puts its fragment in the address bar,
so `:target` would match only with scripting off. The ring is an outline so it replaces neither the
card's shadow nor the tint stripe.

## Fields on cards

The list already consulted the column set; cards did not. Rather than a second preference for cards,
the same keys apply wherever the card draws that field. Cards still draw no assignee, project (on a
board of one project) or due marker, so those keys only affect the list, as before. Star and Repeats
default on, and the column cookie records what is hidden, so every existing browser shows both.
