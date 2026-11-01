# Specs name every surface

## Why

An audit of the specs against the capability registry and the command tree found text that no longer
matched the code:

- `cli` "Command surface" listed 26 groups; the root has 41 commands.
- `cli` "Documented exit codes" stopped at 6, while the CLI, its help and the docs use 7 for a system
  tix imports from that could not be read.
- `http-api` names its routes only by resource kind, so identity, the directory, the second factor,
  ssh keys, restore, route, tree, reclaim, retention, stats, status and connections were covered by
  no http-api requirement.
- `web-ui` said a listing's columns stay on the listing and off the settings screen, while a later
  requirement and the code put them in both places. It also said the settings screen owns every
  per-browser preference, which the sidebar's colour scheme switch and the layout state kept in
  `localStorage` contradict.

## What Changes

- `cli`: the command list matches the root help; exit code 7 is documented.
- `http-api`: one requirement names the routes outside the resource collections.
- `web-ui`: columns are offered beside the listing and on the settings screen, writing one cookie;
  the colour scheme is the one preference the settings screen does not own; sidebar width and
  collapse and the lanes view's completed toggle are layout state, not preferences.

## Impact

Specs only. No code changes.
