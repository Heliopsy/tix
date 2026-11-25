# Copy diagnostics on the release screen

## Why

The app's About screen copies the facts a bug report needs in one press. In the browser a reporter had to
find the version in the sidebar footer and describe their browser by hand, and the connection state, which
explains most "nothing updates" reports, was not shown anywhere.

## What Changes

- `/release#diagnostics`, "Something wrong?", lists the running and newest published versions, and with
  scripting the browser, languages, time zone, screen, scheme, Reduce motion and whether live updates are
  connected, with **Copy diagnostics**. The server address is added only when asked for.
- The live connection marks `<html data-live="on|off">` so the report can say whether it is connected.
