## Decisions

- The JSON is rendered server-side into the button's `data-copy-text`, built by the same helper the Raw
  dialog's Copy all as JSON uses, so the two can never disagree. No extra request and no new script: the
  existing copy.js handles the Clipboard API, the `execCommand` fallback for non-secure pages and the
  Copied label.
- The button sits in the `raw-open` partial, so all three screens get it without touching their
  templates.
- Audit snapshots marshal core types whose secrets are tagged `json:"-"`, so the copy holds no token
  secret.
- TUI: `^` is free on the activity view and already means "copy to the clipboard" on the board, so it is
  reused rather than spending another key. It copies the `core.Event` the view holds.
