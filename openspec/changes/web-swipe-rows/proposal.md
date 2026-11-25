# Swipe a task list row to complete or snooze it

## Why

On a phone the tix-mobile app completes a task by swiping its row towards the end of the line and snoozes it
by swiping the other way, with Undo in the toast. The web list on a touch screen offered the tick and nothing
else: snoozing meant opening the task, and completing gave no way back but finding the row again.

## What Changes

- On a touch screen (a finger or a pen, never a mouse) a task list row follows a sideways swipe over the
  action's colour: the ok green and a tick for Complete towards the line's end, the warning amber and a clock
  for Snooze towards its start, the app's mapping. Past 30% of the row's width, the app's threshold, letting go
  posts the action; short of it the row slides back. Vertical scrolling is left to the browser
  (`touch-action: pan-y`), and a reader who asks for reduced motion gets no slide animation.
- Snooze wakes the task tomorrow morning, the existing `tomorrow` preset. The app asks when; the web keeps the
  choice to the task's own menu and makes the swipe one motion.
- The row carries its targets as `data-swipe-complete` and `data-swipe-snooze`, drawn only on a row with a tick
  and only for a reader with `task:transition` and `task:write` respectively. A finished row is not offered
  Complete.
- Completing from the list now offers Undo, the route back to the state it left, and a snooze offers Undo
  that wakes the task (or puts back the wake time it had). Both are the existing transition and snooze routes.
- No service, store or API change.
