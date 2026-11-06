# What a phone was still offered

## Why

A second phone review after `settings-read-plainly` found the same kind of defect on other screens.
The (i) notes in a toolbar, the view switch and the task screen's header, still floated over the
listing on a phone, and the lanes bar's ran off the right edge on a wide screen. A board still
offered drag to move on a touch screen, where only the Move menu works, and Settings still offered the
setting for it. The command palette offered the Keyboard section and the shortcuts overlay that a
phone does not draw. The (i) beside a large page heading sat below the heading's middle.

The screenshot tool hid some of this: Playwright's full-page capture drops touch emulation while it
draws, so a phone shot showed what a phone hides.

## What Changes

- On a narrow screen or a touch-only device a toolbar's (i) note opens in the flow under the toolbar,
  as the notes beside headings and labels already do. A wide screen with a mouse keeps it floating,
  and the lanes bar's opens leftwards like the view switch's.
- On a touch-only device a board card is not draggable and does not show the grab cursor, and Settings
  does not draw Drag to move. The Move menu on every card is unchanged.
- The command palette does not offer the Keyboard section or Keyboard shortcuts on a touch-only device.
- The (i) beside a heading stands on the heading's baseline.
- The screenshot tool captures a touch viewport by growing the viewport to the page, so the shot keeps
  touch emulation.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: toolbar notes, drag and the palette on touch-only devices.

## Impact

`internal/web/templates/settings.html`, `palette.go`, `assets/palette.js`, `app.css`,
`docs/web-ui.md`, `docs/testing.md` and `scripts/screenshots/shoot.mjs`. No service method, route,
cookie, string or migration changes.
