# The description editor gets a Rich text mode

## Why

Descriptions are Markdown, and the editor is a textarea with a Preview toggle. That suits people who
write Markdown out of habit and nobody else: a table or a checklist has to be typed out by hand, and
the only way to see the result is to flip to a preview that cannot be edited. People asked for a
mode that edits the formatted text directly, while keeping Markdown as what is stored.

## What Changes

- The description editor on the task screen and in the New task form gets a mode switch: Rich text
  and Markdown. Markdown is the textarea and its Preview, unchanged. Rich text is a ProseMirror view
  with a toolbar for bold, italic, strikethrough, headings, bulleted and numbered lists, checklists,
  quotes, code, links and tables, with `@` completion.
- The textarea stays the form field in both modes. Rich text writes the Markdown it serialises back
  into it on every edit, so the form still posts Markdown in `body`, and the server, HTTP API, MCP,
  CLI and history see nothing new. Opening and closing Rich text writes nothing.
- A text that would not come back from Rich text as it went in stays in Markdown, with a line saying
  so. The check parses, serialises and renders both versions with one renderer and compares them.
- The chosen mode is remembered per browser in `localStorage`. Markdown is the default.
- The library is ProseMirror with prosemirror-markdown and markdown-it, bundled once into
  `internal/web/assets/prosemirror.min.js` from exact pins under `scripts/vendor/prosemirror`, and
  fetched only when a page first needs Rich text.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-tasks`: the description editor's modes.

## Impact

Web only: `assets/richtext.js`, the vendored library, `app.css`, the icon sprite, two templates,
`layout.html`, the script strings and their Bulgarian. No service, store, HTTP or CLI change. The
project description stays a plain textarea, because a project's description is shown as plain text,
not Markdown.
