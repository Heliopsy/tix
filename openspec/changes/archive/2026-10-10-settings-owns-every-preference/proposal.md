# One settings screen owns every per-browser preference

## Why

The browser keeps nine display preferences. The screen called Settings exposed six of them.

The three it did not were the three that are not switches: which optional columns each listing draws
(`tix_columns`), which projects the task screen includes (`tix_hidden_projects`), and whether that
screen is a list or a board (`tix_task_view`). Each was editable only from the screen it affects, and
the settings screen said so in one line -- "Which columns a listing shows is chosen on the listing
itself, not here" -- which is true, and is also the whole complaint. A reader who opens Settings to
find out what they can change is told that some of what they can change is somewhere else, and not
where.

That is worse than untidy for two of the three. The project visibility choice is reachable only from
inside a disclosure on the task screen, and it is the preference most able to make a reader think
their work has disappeared: a shorter task list with no visible cause. And a hidden key naming a
project that has since been deleted could not be cleared from anywhere, because the control that
would clear it renders a checkbox per live project and a deleted project has none -- while the key
itself was still being sent to the service as a filter exclusion, where an unknown project key is
refused as not found, so the task screen answered with a not-found page. The one screen that could
have cleared the cookie was the screen that no longer rendered.

## What Changes

- **Settings owns all nine preferences.** It gains the view switch, the project visibility control,
  and one column picker per listing whose columns can be chosen.
- **Each of those three is one control rendered in two places, not two controls.** One constructor
  builds the state (`prefsFor`), one partial renders the form, one reader resolves the cookie, one
  handler stores it. The screen that uses a preference and the settings screen execute the same
  template against the same value, so there is no second default to drift from and no second reading
  of the cookie. A guard changes a preference in one place and asserts the *rendered control* in the
  other, not the stored cookie.
- **`tix_columns` gets one section per listing rather than one global control**, because the
  preference is genuinely per listing and a single control cannot express it. Each section is the
  same picker that listing offers beside itself.
- **`tix_hidden_projects` names each project rather than showing its key alone** -- on settings there
  is no task list in front of the reader to decode a key against -- and names the hidden keys no
  project answers to, which have no checkbox because the project they name is gone. Submitting the
  form forgets them.
- **A hidden key for a deleted project no longer takes the task screen down.** What no longer exists
  excludes nothing, so it is dropped from the filter rather than sent to a service that refuses it.
- **`tix_task_view` keeps its two states.** The list is what an absent cookie means, and the switch
  on settings has the same two positions as the one on the task screen. No third "unset" position is
  introduced anywhere.
- **Every preference says what it does**, in the register the rest of the screen uses: a `muted`
  paragraph the control is `aria-describedby`.
- **`web.Preferences` is the list the screen is held to.** A preference added to the package with no
  control on the screen fails a test, and so does one with no row in the documentation table. The
  table was previously a hand-maintained transcription of a list the code already held.

## Impact

- `internal/web`: `prefs.go` (new), `visibility.go`, `columns.go`, `tasks.go`, `session.go`,
  `render.go`, `templates/partials.html`, `templates/tasks.html`, `templates/settings.html`,
  `assets/app.css`.
- No new route, no service operation, no capability entry: every preference already had a handler,
  and this change gives three of them a second place to be rendered from rather than a second way to
  be stored.
- `docs/web-ui.md`.

Not changed, and deliberately: where a preference lives. A browser preference stays a cookie on that
browser. The terminal interface and the command line keep their own configuration keys, which is a
real divergence rather than an oversight, and the documentation now says so instead of implying one
Settings screen governs both surfaces.

`tix_sync_lower` was named in the request as a tenth preference. There is no such cookie and no such
preference: the string occurs once in the repository, in `internal/web/sync_internal_test.go`, as one
of the lowercase values `envNameSafe` is asserted to reject. Nothing was built for it.
