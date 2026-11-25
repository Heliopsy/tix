# Tasks

## 1. Spec

- [x] 1.1 Delta for `web-ui`: build on the body, reload when safe, bar when not

## 2. Web

- [x] 2.1 `Build` on the page view and `data-build` on `<body>`
- [x] 2.2 `live.js` fires `tix:live-reconnected` when a dropped socket comes back
- [x] 2.3 `deploy.js`: version check, `tix.reloadBlocked`, bar, note and scroll after the reload
- [x] 2.4 Stylesheet part `750-deploy-refresh.css`
- [x] 2.5 Bulgarian strings

## 3. Tests

- [x] 3.1 Go: `data-build` on the body and the script loaded after `decide.js`
- [x] 3.2 Go: `/healthz` reports `version.Version`
- [x] 3.3 Node: each blocking rule, reload when quiet, bar then reload when work is sent, the note
- [x] 3.4 Node: the reconnect event fires after a drop and not on the first connection
