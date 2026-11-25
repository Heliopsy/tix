# Design

- **One state from three streams.** `live.js` keeps up to three streams open: the feed, the screen and the
  inbox badge. Each one reports binding, connecting, open, event, closed (with its code and whether it had
  opened), retrying (with the attempt and the wait), fetched and fetch-failed. The model in
  `livestatus.js` works out a state for each stream and shows the worst one. It is pure and takes the
  time as an argument, so it is covered under `internal/web/jstest`.
- **States.**
  - A stream on an open socket is **live**.
  - It is **offline** when `navigator.onLine` is false or a fetch got no answer.
  - It is **connecting** before its first try fails.
  - It is **polling** once the 15 second fallback poll gets through with the socket still down.
  - It is **reconnecting** in every other case.
  - A browser does not say why a socket closed, apart from the close code. So a close before open with
    code 1006 reads as "could not reach the server", and the polling state carries the hint that WebSocket
    upgrades may not be forwarded. These are the same rules the app uses.
- **Navigation is not news.** Every boosted navigation rebinds the page's streams. When the state goes from
  live to connecting it stays live, so the dot does not flicker and Recent changes is not filled with page
  loads.
- **Words.** The layout renders the indicator's texts in the page's language as JSON (`livestatus.go`). This
  keeps them out of the shared `jsMessages` list. The report on the clipboard is English, for whoever is
  asked to read it.
- **Close reports.** The close is reported even when an error event came first. The error event carries
  nothing, and the close code is the only detail the browser gives.
