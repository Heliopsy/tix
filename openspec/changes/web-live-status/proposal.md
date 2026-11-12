# Show the live connection state in the web shell

## Why

The web reconnects its live updates without saying anything, apart from a "Not live" line on some screens.
Nobody could tell a proxy dropping idle WebSockets from one that never forwards the upgrade, or from a server
that was down. The app shows a live, polling, reconnecting or offline indicator next to the context
switcher, and a connection details sheet with the last error, Reconnect now and a report to copy. The web
should show the same.

## What Changes

- A connection indicator next to the tenant's name in the sidebar. Live is a dot, and every other state has a
  word: Reconnecting, Polling or Offline.
- Connection details, a dialog that opens from the indicator. It shows the state, what carries it, when it
  started, the last contact and event, failures in a row, the next try, the last error (close code or HTTP
  status), hints, the recent changes, Reconnect now and Copy details.
- `live.js` reports what each stream goes through. `livestatus.js` combines those reports and draws them.
- No server change.
