# Tasks

## 1. Library

- [x] 1.1 `scripts/vendor/prosemirror`: pinned packages, entry, esbuild build, `just vendor-prosemirror`
- [x] 1.2 `internal/web/assets/prosemirror.min.js`, recorded in `REUSE.toml`; MIT and BSD-2-Clause texts in `LICENSES/`

## 2. Editor

- [x] 2.1 `assets/richtext.js`: schema, parser, serialiser and fidelity check
- [x] 2.2 Mode switch, toolbar, table and link bars, checklist boxes, mentions, mode in `localStorage`
- [x] 2.3 `data-md-editor` on the task screen's and New task form's description; script in `layout.html`
- [x] 2.4 `app.css` and the toolbar icons in `icons.svg`; strings in `jsMessages` and Bulgarian

## 3. Tests and docs

- [x] 3.1 Round-trip corpus checked by `jstest/richtext.test.mjs` and by goldmark in `internal/markdown`, each watched failing
- [x] 3.2 `docs/web-ui.md`, `docs/features.md`
