# Every screen shares one heading rhythm, one width and one control height

## Why

After the content column was anchored to the sidebar, its left edge matched on every screen, but the
pages still did not line up. Measured with a browser on a seeded store, on all 23 signed-in screens
at 1440px and 1920px:

- The distance from a page heading to the first thing under it was 14, 16, 18, 20, 23, 24, 34 or
  42px depending on the screen. A lede was pulled up by `-8px` and pushed down by `18px`; inside the
  header strip its margins stopped collapsing, so the summary overlapped the heading by 4px and the
  strip ended 34px above the content; a header with a button was 7px deeper than one without; an
  h2 straight under the heading collapsed its 24px margin with the heading's 16px.
- The right edge was 16px on a listing and 52px on a screen of forms at 1440px, and 32px against
  520px at 1920px, because screens of forms got a narrower measure. The controls at the end of a
  header jumped sideways between screens.
- On the task screen the quick-add title box was 49px tall beside a 36px select and button, and
  "Add with details" was a disclosure on a strip of its own under the row. About 315px stood
  between the top of the page and the first task.

## What Changes

- Two tokens, `--head-gap` and `--head-sub`, decide the space under any page heading and between a
  heading and its standfirst. The heading block owns the gap: what follows it has no top margin of
  its own.
- The header strip's controls are centred on the heading's line and do not add to its depth.
- One page measure for every screen. Prose, fields, forms and charts keep their own measures.
- Every input, select, button and link-button in the quick-add and filter rows is `--control-h`
  tall. The pager's step is `--control-h-sm` (44px on touch layouts).
- "Add with details" is a secondary button at the end of the quick-add row, at the row's height,
  and opens the fuller form as a popover anchored under it, like the View menu. On a phone it is
  the icon alone, still named.
- The task list rows, board cards and lane cards are set tighter; the toolbar rows sit closer.

## Impact

- `internal/web`: `assets/app.css`, `assets/icons.svg`, `templates/tasks.html`.
- `internal/web/rhythm_test.go`.
- No route, service method or capability changes.
