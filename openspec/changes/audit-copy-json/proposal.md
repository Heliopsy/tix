# Copy an audit entry as JSON from its row

## Why

Getting one audit entry out of the web UI as JSON took opening its Raw dialog and pressing Copy all as JSON
at the foot. Pasting an entry into a bug report or a script is the common reason to look at it raw, and the
row itself had no way to do it.

## What Changes

- Every activity row, actor feed row and task History row has a Copy JSON button beside Raw. It copies the
  row's audit entry as the audit API lists it, indented, with the fields in the API's order; a row of several
  steps copies them as an array. The label briefly reads Copied, through the site's existing copy handler.
- The button needs scripting and is not drawn without it; Raw still works without.
- In the Raw dialog of a row with several steps, each step has its own Copy JSON button.
- Copy all as JSON now copies the same document as the row's button, so its fields keep the API's order
  rather than an alphabetical one.
- Nothing is copied that the page did not already hold: the entries come from the same `ListAudit` call, under
  the same authorization, that the API answers.
