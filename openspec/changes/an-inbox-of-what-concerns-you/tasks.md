# Tasks

## 1. Model and storage

- [ ] 1.1 `internal/core/inbox.go`: `InboxItem`, `InboxKind`, `InboxFilter`, `InboxPrefs`, `InboxService`
- [ ] 1.2 `internal/store/migrations/0024_inbox.sql` and the store methods on both engines
- [ ] 1.3 `internal/service/inbox.go`: the notifier, mention parsing, the seven methods
- [ ] 1.4 Call sites: create, update, comment add and edit, transition, release
- [ ] 1.5 `internal/authz`: `inbox.read`; `internal/capability/registry.go`: seven entries

## 2. Surfaces

- [ ] 2.1 `internal/wire`, `internal/httpapi`, `internal/client`: the routes
- [ ] 2.2 `cmd/inbox.go`: `tix inbox ls|read|unread|read-all|count|prefs`
- [ ] 2.3 `internal/web`: sidebar entry and badge, inbox page, mute switches in Settings
- [ ] 2.4 `internal/web/assets`: live badge, `@` completion, styles, i18n with Bulgarian
- [ ] 2.5 `internal/tui`: the Inbox view

## 3. Documentation

- [ ] 3.1 `docs/api.md`, `docs/web-ui.md`, `docs/tui.md`, `docs/features.md`, `docs/commands.md`,
      `docs/webhooks.md`, README and `skills/tix`
