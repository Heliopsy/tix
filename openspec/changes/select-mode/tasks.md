# Tasks

## 1. Spec

- [x] 1.1 Delta for `web-tasks`: selection as a mode, board and lanes, the bar, keys, no-JS

## 2. Web

- [x] 2.1 `select` parameter: toggle, Done link, kept by pager, filter form, Clear
- [x] 2.2 Boxes on board and lane cards, bar on every view, boxes and bar hidden outside the mode
- [x] 2.3 `select.js`: the mode, x, Shift+arrows, Ctrl/Shift-click on empty area, long press, Esc,
      exit on empty, selection kept across swaps
- [x] 2.4 Shortcuts: `extendUp`, `extendDown`, `complete` (`d`), `shift+` on named keys
- [x] 2.5 CSS: square box, dimmed done circle, selected card tint
- [x] 2.6 Bulgarian strings

## 3. Tests

- [x] 3.1 Rendering: default vs `?select=1`, board and lanes boxes, kept parameters
- [x] 3.2 No-JS bulk flow from `?select=1` back to select mode
- [x] 3.3 jstest: x, Shift+arrows, Esc, long press, shift-click range, exit when empty, swap keeps
      the mode and the selection
- [x] 3.4 Screenshot states for select mode on the list, board and lanes

## 4. Docs

- [x] 4.1 `docs/web-ui.md`: Acting on many tasks, keyboard
