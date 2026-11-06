## Context

Keys are lower-cased on the way in (`service.CreateProject`, `UpdateProject`, transfer,
`core.ParseTaskRef`) and stored lower case. The store matches a key exactly; `lookupProject`
retries lower case so a typed `INFRA` reaches `infra`. The project key rename work made old keys
and refs redirect, comparing case-insensitively.

## Decisions

### Upper case is a rendering, not a value

Keys stay stored, addressed and serialised lower case. Only what a person reads changes, through
two helpers in `internal/output`: `ShowKey` and `ShowRef` (which leaves a stable identifier
untouched). The web templates call them as `key` and `ref`; CLI tables and text, the TUI and the
calendar feed summary call them directly.

Rejected: storing keys upper case. Every JSON consumer, webhook receiver, saved filter, bookmark and
script that compares `ref` values would see a different string, and every existing database would
need a migration that rewrites refs inside event payloads and audit entries too. The gain over
rendering is nothing a reader can see.

### API and machine formats keep the stored value

JSON, YAML, NDJSON and CSV, webhook payloads, the event stream and the internal palette JSON carry
`infra-8` exactly as before. They are read by programs, which compare values; changing them is a
breaking change for no reader's benefit. Calendar feed UIDs and URLs also stay as they are, so a
calendar app does not see every event replaced; only the SUMMARY a person reads changes.

### Canonical URL stays lower case

Links the interface draws keep pointing at `/tasks/infra-8`, so existing bookmarks, browser
history and shared links stay canonical. `/tasks/INFRA-8` answers the same task directly rather
than redirecting: the old-ref redirect already compares case-insensitively, and a redirect for case
alone would only cost a round trip.

### Input is case-insensitive everywhere

`ParseTaskRef` lower-cases the key, `lookupProject` retries lower case, and the filter resolver
settles project terms through `lookupProject`. Four service paths still called `tx.GetProject`
directly (copy target, statistics project, sync mapping, a new task's project) and now use
`lookupProject`. The confirmation typed to delete a project's tasks was already compared with
`EqualFold`.

### The picker label is a browser cookie

Every display preference in the web interface is a cookie listed in `web.Preferences`; the settings
screen says so in its first line, and the guards `TestSettingsOwnsEveryPreference` and
`TestEveryPreferenceIsDocumented` hold the list to the screen and the docs. Group arrangement and
saved filters are stored server side, but they describe the tenant's data (which groups, which
filters) rather than how a reader likes things drawn. A label style is the latter, and someone may
well want keys on a terminal-heavy laptop and names on a tablet, so it follows the tint and the task
view into a cookie, `tix_project_label`.

### Narrow screens

A native `<select>` cuts its text at the end, which is where the key sits in the default style. The
name is shortened to 24 characters with an ellipsis before the label is built, so the key is whole in
every option. On a phone the add box and a heading's add field are capped to a few characters; their
options carry the key alone in `data-short`, and `forms.js` swaps it in while
`(max-width: 639px)` matches and back when it stops. The add box's cap grew from 5.25rem to 6.5rem so
an eight-letter key fits. A custom dropdown would let CSS cut only the name, but it would cost the
native control's keyboard and mobile behaviour, so none is built. A text icon (emoji) leads the label
when a project has one, since an option holds text; colour and image icons need markup an option
cannot hold and are left out.

## Risks

- Tests that pinned lower-case rendering changed to the upper-case form; behaviour guards that read
  hrefs, ids and values still read the lower-case value, which did not change.
- Concurrent template work (task page overhaul, sidebar) touches the same files; the change is kept
  to helper calls so a rebase is line-local.
