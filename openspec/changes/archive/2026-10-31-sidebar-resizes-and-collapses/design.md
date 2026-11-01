# Design

## Where the width lives

In `localStorage`, not a cookie. The width is only ever applied by CSS, so the server has no use for
it, and the head script sets it before the body is parsed, so there is no flash for a cookie to
prevent. Two keys, `tix.sidebar.width` (an integer number of pixels) and `tix.sidebar.collapsed`
(`true`), each removed when it holds the default. Neither says anything about anybody's work, which
is the condition `docs/web-ui.md` already sets for what may live in browser storage.

The width is written to `--side-w` on the root element's inline style, which outranks the
breakpoint that widens the default sidebar on a large screen. Collapsing sets a class on the root,
so a boosted navigation that replaces the body undoes neither.

## Clamping and the collapse threshold

180px is the narrowest the expanded sidebar stays readable ("Import and export" wraps but is still a
label); 480px is enough for any project name and still leaves a 900px window most of its width. A
drag released under 120px collapses rather than clamping to 180px, which is the gesture the reader
made. The stored width is kept while collapsed, so expanding returns to it.

## The rail

64px. The labels are visually hidden with the same declarations as `.sr-only`, not removed, so each
link keeps its accessible name; the script adds a `title` to each while collapsed, so a pointer gets
a tooltip, and takes it away on expanding, where the label is on screen already. Group headings
become a hairline, the handle and version line are hidden. Settings and Sign out gain icons that are
shown only in the rail.

## One content inset

The alternative was to keep centring and give every screen the same measure, which would have
narrowed the boards and listings back to the prose width they were widened from. Anchoring the
column to the sidebar keeps every measure and makes the left edge a single number: the gutter. The
cost is that a screen narrower than the window leaves its spare width on the right rather than
split, which is how every sidebar application lays out and what makes switching screens stable.
