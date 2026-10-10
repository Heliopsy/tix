# Saves that mean what they say, listings that carry what they have

## Why

Four defects in the browser, each one a screen reporting success while doing something other than what
the reader asked.

**A blank box did not clear the value.** The service clears a custom field whose key arrives with a null
value: `mergeTaskFields` deletes every key an update names and re-adds only the ones carrying a value, so
a named key with no value is precisely how a field is emptied, and an omitted key is "leave as is". The
browser dropped blank keys from the update map instead of sending them, so clearing a box and saving
reported "task saved" and changed nothing. The account form had the same shape for the display name:
`DisplayName` was sent only for a non-empty name, so a name could be set and never removed. Twelve lines
above it, the tenant theme already argued the other position — "the empty option clears the theme, so the
field is always sent and always applied: treating "" as unset would make the picker one-way."

**Hiding one project removed every task past the fiftieth project.** The task screen listed projects with
no limit, so the store answered with `core.DefaultPageLimit` rows and the cursor was discarded; the screen
then turned the visibility choice into an explicit inclusive filter naming the projects to show, which
could only name projects from that one page. Untick any single project in a tenant with more than fifty
and every task in every project past the first page left the screen, with no pager, no count and no
message, while the control said exactly one project was hidden. The same truncated listing was the tenant
diagram's authoritative project count and the lookup that resolves a task's project when it is ticked
done, so that count was a sample and that lookup answered Not found.

**The activity feed dropped matched entries that appeared on no page.** The free-text box was applied by
the screen over pages the store had already returned. A screen assembled that way cannot report a cursor:
the only cursor it holds names the end of the last store page it read, so every match it trimmed off the
end of a screenful was stepped over by Next rather than shown on it. Walking the feed to its end showed
50 of 55 matching entries; five appeared nowhere.

**The delivery log's second page was unreachable.** The handler read the cursor parameter and computed
the next cursor onto the view, and the template referenced neither. Older deliveries existed and were
reachable only by hand-typing `?cursor=`.

## What Changes

- **A form field that arrives empty clears the value; a form that does not name the field changes
  nothing.** Presence of the key decides, not the value it carries, so a caller naming a subset of fields
  changes only the subset it named. A required custom field emptied this way is refused with a validation
  error rather than stored empty.
- **The task listing excludes the projects put away instead of naming the ones to show.** The exclusion is
  read from the reader's own preference rather than derived from a listing, so it is complete whatever a
  listing reached, and it names a handful of keys rather than one per project in the tenant.
- **Every browser screen that needs the whole set of projects walks the listing to its end.** The
  visibility control, the accent map, the workflow lookup and the tenant diagram's count all did their
  work on a page of it.
- **The activity screen's free-text box becomes part of the audit filter, answered by the store.** A page
  the store answers whole is a page whose cursor is the store's own, so Next resumes where the screen
  stopped. `core.AuditFilter` gains `Text`, both engines answer it, and the HTTP API accepts it. The empty
  result no longer qualifies itself with the window it managed to read, because there is no longer one.
- **The delivery log carries the pager every other keyset listing carries.**

## Impact

- `internal/core`: `AuditFilter.Text`, additive.
- `internal/store/sql`, `internal/store/sqlite`, `internal/store/postgres`: one shared substring predicate,
  each engine naming its own columns, because the snapshots are JSONB on one and TEXT on the other.
- `internal/httpapi`, `internal/client`: `text` on the audit listing.
- `internal/web`: the save rules, the project walk, the activity scan, the delivery pager.
- The activity screen's empty-state wording changes, and `activityView.Scanned` is removed with the window
  it described.
