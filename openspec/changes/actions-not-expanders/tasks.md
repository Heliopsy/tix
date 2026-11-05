# Tasks

## 1. Mechanism

- [x] 1.1 `internal/web/actiondlg.go`: dialog state, `refuseIn`, opener and shell helpers
- [x] 1.2 `templates/partials.html`: `dlg-open`, `dlg-start`, `dlg-end`, `dlg-cancel`
- [x] 1.3 `assets/forms.js`: open, close, focus return, promote a dialog drawn open
- [x] 1.4 `assets/app.css`: `a.btn`, the dialog, section heads; the add disclosure's styles removed

## 2. Screens

- [x] 2.1 Tokens, users, domains, SSH keys, webhooks, tenant members, notification channels, fields, sync
  sources, saved filters: header button and dialog, refusals come back open
- [x] 2.2 `GET /projects/{key}/settings`; the board's header links to it; redirects return there
- [x] 2.3 Token revoke, user delete, template delete, channel remove, cleanup, archive: confirm dialogs
- [x] 2.4 Transfer, bundles, sync runs: explanations as text; sign-in help and scheme comparison as dialogs

## 3. Guards and docs

- [x] 3.1 `TestNoPageActionOrSectionSitsInAnExpander` and the dialog tests, each watched failing
- [x] 3.2 Existing tests moved to the new structure
- [x] 3.3 Bulgarian strings
- [x] 3.4 `docs/web-ui.md`, `AGENTS.md`
