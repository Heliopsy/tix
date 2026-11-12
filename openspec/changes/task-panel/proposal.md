# Open a task in a panel beside the list

## Why

On a wide window the tix-mobile app shows the task beside the list it was opened from, with a divider the
reader drags to share the room. The web offers a task as its own page, which loses the list, or in a modal
dialog, which covers it. Going down a list task by task means opening and closing one after another.

## What Changes

- **Task opens in** gains a third position, **Panel**, stored as `tix_task_open=pane`. Page stays the default
  and Dialog is unchanged.
- With Panel, a task followed from a listing on a window at least 1100px wide opens in a panel on the right of
  the page. The listing stays usable beside it, its row marked `aria-current`, and another task followed from
  it replaces the one in the panel.
- The panel is the task dialog drawn without a modal: the same fragment, the same history entry, the same
  answers to forms. Escape, Close and Back close it.
- A divider between the listing and the panel is a `role="separator"` that drags with the pointer and moves
  with the arrow keys, Home, End and Enter, as the sidebar's edge does. Its width is clamped and kept in a
  `tix_pane` cookie, per browser.
- On a narrower window, Panel opens the task as a dialog. Without scripting a task is a link to its page.
