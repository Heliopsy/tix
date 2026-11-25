# Tasks

## 1. Configuration

- [x] 1.1 `server.public_url` key, `TIX_SERVER_PUBLIC_URL`, validation and tests
- [x] 1.2 Classify the key in `cmd/config_shadow_test.go`

## 2. Service and API

- [x] 2.1 `core.Task.WebURL`, `service.WithPublicURL`, set on every task the service returns
- [x] 2.2 `tasklink.Rehost` and the tenant-domain rule
- [x] 2.3 `just openapi`; `web_url` present and absent in HTTP and MCP tests

## 3. Links drawn from a request

- [x] 3.1 `httpapi.Origin`; the Share menu, hand-off, calendar, invites and Connect screen use it
- [x] 3.2 `data-share-origin` for `assets/share.js`
- [x] 3.3 Notification channel messages link under the public address

## 4. SSH

- [x] 4.1 `Operation.Formats`, `tix task link` hosted with `task.show`
- [x] 4.2 `task link` uses `web_url`, refuses `--base` over SSH; in-process SSH tests

## 5. Terminal interface

- [x] 5.1 Share key, picker, OSC 52 copy and the reveal panel, in help and the palette
- [x] 5.2 Web base for a remote context; model tests

## 6. Docs

- [x] 6.1 configuration, deployment, tui, web-ui, tenancy, the skill; drop the limits from `task-share`
