## ADDED Requirements

### Requirement: The web interface has a web app manifest

The server SHALL answer GET /manifest.webmanifest as application/manifest+json naming the app after the
signed-in tenant, or tix without a session, with start_url and scope /, standalone display, the tix icon, and
light and dark page colours, and every page SHALL link it.

#### Scenario: Installing from a signed-in browser

- **WHEN** a signed-in reader of tenant Acme Works opens the manifest
- **THEN** it names the app Acme Works, opens / standalone and lists the tix icon

### Requirement: The manifest offers launcher shortcuts

The manifest SHALL list Quick add, Inbox and My work as shortcuts, labelled in the reader's language, opening
the task list with quick add focused, the inbox and My work.

#### Scenario: Shortcut labels in Bulgarian

- **WHEN** a reader whose language is Bulgarian opens the manifest
- **THEN** the shortcuts read Бързо добавяне, Входящи and Моята работа

### Requirement: Text shared to tix opens quick add

The manifest SHALL declare a GET share target at /share taking title, text and url. The route SHALL join them
into one line and redirect to the task list with that line in the quick add box, and SHALL create nothing.

#### Scenario: Sharing a page

- **WHEN** another app shares the title Release notes and the url `https://example.test/notes`
- **THEN** the task list opens with `Release notes https://example.test/notes` in quick add and no task exists yet

#### Scenario: Sharing while signed out

- **WHEN** a signed-out browser opens a share
- **THEN** it is sent to sign in and, once signed in, lands on quick add holding the shared line

### Requirement: The service worker caches nothing

The server SHALL serve a service worker at /sw.js with scope / as JavaScript that is not cached, and the
worker SHALL store no response and leave every request to the network.

#### Scenario: Fetching the worker

- **WHEN** a browser fetches /sw.js
- **THEN** it gets text/javascript with Cache-Control no-cache, and the source uses no cache storage
