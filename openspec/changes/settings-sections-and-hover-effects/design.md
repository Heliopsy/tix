# Design

## Where Hover effects is kept

In a cookie, `tix_hover`, beside `tix_drag_move` and `tix_open_after_add`, which it copies: absent
means on, and `0` is the only value that turns it off. Every display preference in this interface is
a cookie on the browser rather than a row on the account, and the docs say why (two people sharing a
tenant choose differently, and the setting needs no write path through the service). A per-account
column would be the first preference stored differently from the other eleven, so no migration.

The attribute goes on `<html>`, like `data-theme`, so it is in place before the body paints and a
boosted navigation that swaps the body does not lose it. That is also why the form opts out of
`hx-boost`, as every other settings form does.

## What "off" removes

The effects are written once, under `:root:not([data-hover="off"])` and inside
`@media (hover: hover)`, so a touch screen never gets a sticky highlight and the off switch is one
attribute selector rather than a second set of rules. The few hover backgrounds that predate this
change (table rows, task rows, project rows, people) are neutralised by a short block under
`:root[data-hover="off"]`. Focus rings are not hover effects and stay: they are how a keyboard reader
finds their place.

Transitions are declared only inside `@media (prefers-reduced-motion: no-preference)`, so a reader
who asked the system for less motion gets the colour change and no movement whatever this preference
says.

## Section navigation without script

Plain anchors to each section's id, with `scroll-margin-top` so a heading is not tucked under the
phone bar. The navigation is laid out by a container query on the settings block rather than by the
viewport, because the sidebar is resizable and collapsible: the room beside it is what decides
whether a side column fits, not the window.

## Save feedback

The settings forms post with `next` set to `/settings?flash=<what>+saved#<section>`. The layout
already renders `flash` as a toast and `here()` already strips it, so a later form on the same page
does not carry the old message forward. Built by a template helper that URL-encodes the words, so the
`Location` header never carries a raw space.
