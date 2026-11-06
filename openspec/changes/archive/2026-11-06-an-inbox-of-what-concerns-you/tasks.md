# Tasks

## 1. Model and storage

- [x] 1.1 `internal/core/inbox.go`: `InboxItem`, `InboxKind`, `InboxFilter`, `InboxPrefs`, `InboxService`
- [x] 1.2 `internal/store/migrations/0024_inbox.sql` and the store methods on both engines
- [x] 1.3 `internal/service/inbox.go`: the notifier, mention parsing, the seven methods
- [x] 1.4 Call sites: create, update, comment add and edit, transition, release
- [x] 1.5 `internal/authz`: `inbox.read`; `internal/capability/registry.go`: seven entries

## 2. Surfaces

- [x] 2.1 `internal/wire`, `internal/httpapi`, `internal/client`: the routes
- [x] 2.2 `cmd/inbox.go`: `tix inbox ls|read|unread|read-all|count|prefs`
- [x] 2.3 `internal/web`: sidebar entry and badge, inbox page, mute switches in Settings
- [x] 2.4 `internal/web/assets`: live badge, `@` completion, styles, i18n with Bulgarian
- [x] 2.5 `internal/tui`: the Inbox view

## 3. Channels and the live list

- [x] 3.1 `inbox_channels` and `inbox_deliveries` in 0024, store methods on both engines
- [x] 3.2 Service methods, queueing in `deliverInbox`, the dispatcher's channel pass and formats
- [x] 3.3 HTTP routes, client, `tix inbox channel`, registry entries with terminal exemptions
- [x] 3.4 Settings, Notifications and the channel screen; the inbox's live binding and highlight

## 4. Documentation

- [x] 4.1 `docs/api.md`, `docs/web-ui.md`, `docs/tui.md`, `docs/features.md`, `docs/commands.md`,
      `docs/webhooks.md`, README and `skills/tix`
