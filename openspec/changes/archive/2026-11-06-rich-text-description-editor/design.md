# Design

## Library

| Option | Licence | Upkeep | Shape | Verdict |
| --- | --- | --- | --- | --- |
| Toast UI Editor 3.2.2 | MIT | last release February 2023 | prebuilt UMD with a built-in Markdown/WYSIWYG switch, about 600 KB minified | unmaintained, its own Markdown parser, loads images |
| Milkdown 7 (Crepe) | MIT | active | ProseMirror plus remark/unified, ESM with many packages, needs bundling | larger than ProseMirror alone for the same editing core |
| TipTap 3 | MIT | active | ProseMirror with its own extension layer, ESM only, Markdown through an extension | needs bundling anyway, and adds a layer over the part we would configure |
| ProseMirror + prosemirror-markdown + markdown-it | MIT (entities: BSD-2-Clause) | active, one maintainer since 2015 | ESM packages, bundled once with esbuild into one IIFE | chosen |
| Hand-rolled contenteditable | ours | ours | no dependency | execCommand is deprecated, and lists, checklists and tables are exactly where it loses text |

There is no prebuilt single-file ProseMirror. The repository has no bundler, so the bundle is built
once by `scripts/vendor/prosemirror` (pinned `package.json` and lockfile, esbuild, `just
vendor-prosemirror`) and committed, the way `htmx.min.js` is committed, with its sources, versions,
licences and digest in `REUSE.toml`. The build is reproducible: rebuilding gives the same bytes.

Size: 362,443 bytes, 126,974 gzipped (`gzip -9`). It is not in the layout: `richtext.js` (small, every page)
injects it the first time a page needs Rich text, so pages without a description editor, and readers
who stay in Markdown, never fetch it. The `entities` package is aliased to its decoder alone, which
saves 23 KB of encoder tables nothing calls.

## Fidelity

markdown-it parses (CommonMark plus GFM tables and strikethrough, `linkify`, hard breaks, and a small
rule for `[ ]`/`[x]` task items), prosemirror-markdown builds the document and a serialiser writes it
back. The serialiser is tuned to the server's dialect: a line break is written as a newline because
the server draws hard wraps; a bare URL is written back bare, never `<...>`; checklist boxes go after
the bullet; tables are GFM pipe tables with their alignment row and escaped pipes.

Before Rich text opens a text, `check` renders the original and the serialised text with that
markdown-it and compares the HTML. Any difference keeps Markdown with a notice. A single `~` in plain
text also keeps Markdown, because the server strikes through `~one~` and markdown-it does not, so the
render comparison cannot see that loss. Known texts that stay in Markdown: a list item that opens with
a code block, quote or heading, an empty list item, and a single-tilde strikethrough.

The round-trip corpus (`internal/web/jstest/testdata/richtext-corpus.json`) is checked twice: by the
node suite through the editor's own code, and by `internal/markdown` through goldmark, the renderer
the task screen uses, so a rewrite that markdown-it accepts but the server draws differently fails.

What a rewrite normalises, once someone edits: `_em_` becomes `*em*`, `__strong__` becomes `**strong**`,
setext headings become `#`, indented code becomes fenced, `3)` becomes `3.`, reference links become
inline, entities become their characters, a table gets outer pipes and a `---` rule. Each renders the
same. Opening Rich text without editing writes nothing, so a save without an edit is still refused as
unchanged and history shows no diff.

## Safety

Raw HTML is plain text in both parsers, so it is shown as typed. An image is drawn as a dashed chip with
its alt text, never an `<img>`, so opening a description loads nothing. A link in the editor carries an
`href` only for http, https and mailto. The pages set no Content-Security-Policy today; the editor
still uses no `eval`, no inline script and no inline `style` attribute, so it would hold under one.

## Mode and Preview

The mode is one `localStorage` key, `tix.descriptionMode`, read and written inside try/catch, and
Markdown when storage is refused. Preview is kept in Markdown mode and hidden in Rich text, where the
text is already drawn formatted. Preview still differs in one way: it links references and handles,
which Rich text shows as plain text.

## Mentions

Rich text runs the same `mentionAt` and `mentionMatches` from `decide.js` as `mention.js` over the text
before the caret in the current block, and draws the same `.mention-menu` listbox under the editor.
