# Design

## Add another is a form field, remembered by the server

The checkbox is a field of the quick-add form (`another=1`). The create handler reads it on every
quick add and writes `tix_add_another` (set when ticked, cleared when not), the way the lanes
grouping is remembered by the screen that uses it rather than from Settings. The listing draws the
checkbox ticked from that cookie. Nothing here needs a script.

With it ticked a successful add redirects to the listing it came from, as it does with Open a new task
off, and sets a one-shot `tix_quick_focus` cookie. The listing that reads it puts `autofocus` on the
box and clears it. A plain load focuses the box, and so does a boosted one, because htmx focuses an
`autofocus` element after a swap. The box is not focused on every arrival: an earlier build did that
and it cost every arrival the top of the page, so only the redirect after an add asks for it. A
cookie rather than a query parameter, because the listing copies its query into its pager, view and
chip links, and a parameter there would focus the box on every later page of the run.

Open a new task stays the preference for adds made without the checkbox (and for Add with details):
the checkbox is the more specific choice, made on the row it applies to.

## Clipboard

`navigator.clipboard.readText()` is the only way a page can read the clipboard, and it needs a
secure context and, in some browsers, a permission prompt. The button is drawn `hidden` and
`quickadd.js` reveals it where `readText` exists, so a browser without it never shows a dead button.
The pure part (the first non-empty line, trimmed) is `tix.firstLine` in `decide.js`.

The line goes into the box and is not added, so the reader sees the preview chips and can edit before
Enter. A failed or empty read shows the toast in the page's `#toast` region.

## Shortcut

`quickPaste` is bound to `ctrl+shift+v` and `meta+shift+v` in every scheme, matching the app. The
shortcut layer stands down in text fields, where ctrl+shift+v is the browser's paste as plain text,
so it only acts on the page itself; on a screen with no quick-add box the handler answers false and
the browser keeps the key. `tix.comboOf` used to drop Shift from a ctrl or meta letter chord; it now
names it, so ctrl+shift+a no longer reads as ctrl+a (select all).
