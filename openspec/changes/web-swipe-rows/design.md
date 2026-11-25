# Design

- **No new endpoint.** The swipe builds a hidden form inside the row posting to the row's `data-swipe-*` route
  with the token and `next` its tick form already carries, and submits it. The row is `data-inplace`, so
  `assets/feel.js` swaps the row and brings `#toast` with it, exactly as for the row's other forms; without
  htmx the form posts and redirects.
- **Undo.** The toast's Undo is the existing `flashundo` mechanism. Complete writes the route back
  (`undoRoute`, as a move does). Snooze writes `unsnooze:` or `snooze:<old wake time>`; the prefix table in
  `quickundo.go` (`undoKinds`) maps it to the unsnooze or snooze route. The table and the snooze change are
  the same code as the open quick actions branch (`feat/web-quick-actions`), so whichever lands second keeps
  one copy.
- **Decisions in decide.js.** `swipesWith`, `swipeAxis` (a 10px slop, a diagonal is a scroll), `swipeSide`,
  `swipeAction` (30% of the width), `swipeOffset` and `swipeFields` are pure and covered by the node suite;
  `assets/swipe.js` is the pointer wiring, bound at the document.
- **Gesture.** A swipe may start anywhere on the row except a text field, as the app's whole row is the
  gesture; the click a swipe ends with is swallowed so a title link under the finger does not open. The
  browser keeps vertical panning and cancels the pointer when it scrolls, which ends the swipe.
- **No setting.** The app has none for swipes (only haptics), so neither has the web.
