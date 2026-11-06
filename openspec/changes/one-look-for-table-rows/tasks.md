# Tasks

## 1. The sticky mark

- [x] 1.1 `users.html`: the Manage dialog is `manage-user-ID`, so the row keeps `user-ID` for the activity feed's links
- [x] 1.2 `forms.js`: closing a dialog drops the fragment that named it with its parameter
- [x] 1.3 `copy.js` and `app.css`: `.is-target` and `.is-flashed` are a flash that clears itself

## 2. Row states

- [x] 2.1 `app.css`: `--row-hover`, `--row-selected`, `--row-target` and the one set of row rules
- [x] 2.2 Every `<table>` and every row list carries `rows`; the one-off hovers are deleted
- [x] 2.3 Guards: `rowstates_test.go` (stylesheet, templates, dialog ids) and `browser_rows_test.go` (in Chrome)
