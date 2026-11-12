# Install the web interface as an app

## Why

The tix-mobile app offers launcher shortcuts (Quick add, Inbox, My work) and takes text shared from other
apps into quick add. The web interface could not be installed at all: no manifest, no service worker, so a
browser offered no Install, a phone had no home-screen entry with shortcuts, and nothing could be shared to
tix from another app.

## What Changes

- `GET /manifest.webmanifest`, linked from every page's head, names the app after the tenant (or tix before
  sign-in), opens `/` standalone, uses the tix icon and the light and dark page colours, and lists Quick add,
  Inbox and My work as launcher shortcuts in the reader's language.
- The manifest declares a share target, `GET /share` with `title`, `text` and `url`. It joins them into one
  line and redirects to the task list with that line in the quick add box, focused and previewed. Nothing is
  created until the reader presses Add. A signed-out share goes through sign-in and comes back.
- `/tasks?add=LINE` puts LINE in the quick add box and focuses it; the Quick add shortcut uses it empty.
- `GET /sw.js` is a service worker at scope `/` that caches nothing and lets every request go to the network.
  It exists so browsers that look for one offer Install. It is registered by a deferred script.
- No service, store or API change.
