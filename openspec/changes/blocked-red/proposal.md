# Draw blocked in red in the browser

## Why

The terminal draws the `blocked` category in red, the colour that already means urgent, and the theming docs
say blocked is red whatever a tenant themes itself. The web interface drew it in the amber warning colour
instead: the status pill, the lane pills, the statistics breakdown and its key, the status tint on rows and
cards, and the dependency "blocked" pill. One state was two colours on two surfaces.

## What Changes

- The `blocked` status pill, the `cat-blocked` category rule, the `tint-status-blocked` stripe and the
  `badge.blocked` badge use the `--danger` token instead of `--warn`, in every colour scheme.
- The pills that reuse the blocked class (a failed webhook or channel delivery, a server that stopped
  heartbeating) turn red with it, which fits a failure.
- The stylesheet comment that described a solid amber blocked pill is corrected.
