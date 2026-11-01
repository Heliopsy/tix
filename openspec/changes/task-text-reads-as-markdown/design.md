# Design

## One renderer, on the server

`internal/markdown` wraps goldmark and is the only place Markdown is parsed. The browser never parses
Markdown: the Preview toggle posts the text to `/markdown/preview` and shows what comes back, so the
preview is the page.

goldmark was chosen over writing a parser because CommonMark's edge cases are where sanitising
renderers go wrong, and over the other Go options because it has no dependencies, needs no cgo, and lets
every unsafe node be drawn by our own function. It replaces the default rendering of raw HTML, HTML
blocks, links, autolinks, images, headings, list items and task-list checkboxes.

## What is never rendered

- Raw HTML, inline or block, is escaped and shown as typed. goldmark's default drops it with a comment,
  which loses text the writer typed.
- A link is kept only when its address parses as absolute http or https with a host, or mailto. Anything
  else, `javascript:`, `data:`, relative paths, protocol-relative addresses, renders as its label text.
  Links carry `rel="noopener noreferrer nofollow"` and no `target`, because no link in the app opens a new
  tab.
- Images are never drawn. The two safe options were proxying, which makes the server fetch arbitrary
  addresses on a writer's behalf, and https with `referrerpolicy=no-referrer`, which still tells the
  image's origin the reader's address and when they opened the task. A link labelled by the alt text
  fetches nothing until somebody chooses to follow it.
- Table alignment is an `align` attribute rather than goldmark's default `style`, so no rendering needs
  `style-src 'unsafe-inline'`. There is no app-wide CSP yet (see ROADMAP.md); the output is written so
  that a strict one holds when it arrives.
- Headings move two levels down, under the card's `h2`. Soft line breaks render as `<br>`, so text
  written before this change keeps its lines.

## Checklist items are numbered once

Items are counted in document order by walking the parsed tree, which is also the order the renderer
draws them. `SetItem` finds item N the same way and rewrites the one character between its brackets,
located from the start of the item's first line, so the rest of the text is byte-for-byte unchanged and
a marker in a code block is never touched.

The browser draws each tickable item as a submit button with `role="checkbox"` and `aria-checked`, named
by the item's text, inside one form carrying the CSRF token and the task's version. The button posts
`check=N` or `uncheck=N`, so a second press that crossed a refresh asks for the same state rather than
flipping it back. It works without a script; with htmx the task screen swaps in place and feel.js flips
`aria-checked` at once, restoring the snapshot on a refusal.

## The count is computed on read

`Task.Checklist` is set by the service on `GetTask`, `ListTasks`, `TaskTree`, `UpdateTask` and
`CheckTaskItem`, not by the store. That keeps it out of audit snapshots, history diffs and exports, and
needs no column or migration. Parsing a body costs microseconds and a body without `[` skips the parse.

## The filter reads markers, not Markdown

`has:checklist` and `checklist:open` must be answered the same way by SQLite, PostgreSQL and the
terminal's in-memory matcher. A literal substring is the predicate all three answer identically, so the
filter looks for a bullet (`-`, `*` or `+`), a space, `[ ]`, `[x]` or `[X]`, and a space. It can disagree with the count for a
marker inside a code block or an ordered-list item; docs/filtering.md says so.

## Alternatives not taken

- Rendering with glamour in the terminal and command line: it is not a dependency, and plain text is
  what a pipe wants.
- A stored checklist count: it would need a migration and a second source of truth for the body.
