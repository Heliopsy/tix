## ADDED Requirements

### Requirement: Screens are drawn in the language the browser asks for

The browser interface SHALL negotiate a language from the request's `Accept-Language` header, choosing
the highest-weighted language it ships, matching an exact tag before its base language, and SHALL
draw English when the header names none it ships. Every screen SHALL carry the chosen language in
the root element's `lang` attribute and SHALL draw its text, flashes, page title, error heading and
hint, and fixed labels in that language. Text a language's catalog lacks SHALL be drawn in English.
The interface SHALL ship English and Bulgarian.

#### Scenario: A Bulgarian browser

- **WHEN** a browser sends `Accept-Language: bg-BG,bg;q=0.9,en;q=0.5` and opens the task screen
- **THEN** the root says `lang="bg"` and the navigation, the flash and the error screen are in Bulgarian

#### Scenario: A language the build does not ship

- **WHEN** a browser sends `Accept-Language: de-DE,fr;q=0.8`
- **THEN** the screen is drawn in English with `lang="en"`

### Requirement: Text written by scripts follows the page's language

For a language other than English, every screen SHALL carry, in its head, the translations of the
texts the shipped scripts write themselves, and the scripts SHALL use them. A script SHALL fall back
to the English text when no translation is carried. An English page SHALL carry no such table.

#### Scenario: The sidebar toggle in Bulgarian

- **WHEN** a Bulgarian page is loaded
- **THEN** the head carries the Bulgarian text for "Collapse sidebar" and the toggle is labelled with it

### Requirement: Every user-facing template string goes through the catalog

No template SHALL write visible text, or a `title`, `placeholder`, `aria-label`, `alt`,
`data-rail-name`, `aria-valuetext` or `data-confirm` value, other than through the translation
helpers, except text inside `code`, `kbd`, `pre` and `samp`. Every language other than English SHALL
hold an entry for every key the interface uses and no entry it does not, and each entry SHALL format
the same number of arguments and carry the same markup as its key. A screen exempted from this check
SHALL be named, with its reason, in the test that makes it.

#### Scenario: A new English literal

- **WHEN** a template gains `<p class="group">Data</p>`
- **THEN** the test suite fails naming the template and the literal

#### Scenario: A missing translation

- **WHEN** a key used by a template has no Bulgarian entry
- **THEN** the test suite fails naming the key and where it is used

### Requirement: Unfamiliar concepts carry an info tip

Beside a concept a newcomer cannot decode from the screen (claims and leases, tokens and scopes,
the filter syntax, recurrence, stars, list, board and lane views, workflows, states and categories,
webhooks, connections, sync, domains, import and export, retention, SSH keys, statistics figures,
the activity feed) the interface SHALL offer an info tip: a disclosure that works without
scripting, from the keyboard and on touch, is named "About" its topic for assistive technology,
explains the concept in one or two plain sentences in the page's language, and names the `docs/`
page that says more where one exists. A tip SHALL NOT link to another origin.

#### Scenario: The view switch explains itself

- **WHEN** a reader opens the tip beside the task screen's view switch
- **THEN** it explains list, board and lanes and names `docs/web-ui.md`, and its button is announced as "About Views"
