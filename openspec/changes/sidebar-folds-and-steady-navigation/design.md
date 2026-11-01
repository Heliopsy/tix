# Design

## Swap the whole body, but draw it final

Two ways to stop the sidebar moving on a boosted navigation: swap only the main column and leave the
sidebar in place, or keep swapping the body and make the server's sidebar identical to the live one.

Swapping only `main` would have to update, in place, everything outside it that a page changes: the
current entry, the inbox badge, the saved filters after a save, the palette's and shortcuts' data
blocks, the toast, and the task dialog's host, which some screens have and some do not. Every one of
those is a chance for the sidebar to say something the page does not.

So the body is still swapped, and nothing in the sidebar is drawn in a state a script corrects:

- Width, rail and folds come from a cookie the server reads (`tix_sidebar`), not `localStorage`.
- Controls that do nothing without a script are marked `data-needs-js` and shown by a `js` class on
  `<html>`, set by `sidebar.js` from the head before the body is parsed. A swap never replaces
  `<html>`, so the class is there in the first frame of every navigation.
- Recently viewed is the one thing only the browser knows. It carries `hx-preserve`, so the swap
  keeps the drawn list, and `recent.js` then only moves its current mark or adds the task just opened.

On a full reload Recently viewed is still filled by the deferred script, one step after first paint.
Drawing it on the server would mean the list living in a cookie and a lookup of up to ten titles on
every page; that is left for when someone asks.

## One cookie

`w=320&rail=1&fold=configure.data`. Defaults are left out and a cookie of defaults is removed. The
server clamps the width to the same limits as the handle and ignores unknown group names, so a damaged
cookie costs an arrangement and never a page. It is not a Settings preference: it has no route and no
control on the settings screen, and it is set only by the sidebar's own controls.

## A folded group still shows where you are

Folding Configure and then opening Users from the palette would otherwise leave the sidebar with no
current entry. Rather than unfolding the group behind the reader's back, the folded group shows the
current entry alone under its heading. This is CSS: `.navgroup.is-folded` hides every child of the
body that is not, and does not contain, `[aria-current="page"]`.

## The rail

On the rail every group is its icons, folded or not, as before. The Recently viewed heading is its
clock there, and pressing it, or any group heading, expands the sidebar with that group unfolded.
