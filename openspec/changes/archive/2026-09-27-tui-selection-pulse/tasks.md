# Tasks

## The pulse

- [x] `tui/motion.go`: the period, the idle timeout, and the rules as pure functions
- [x] `tui/theme.go`: a second selection style for the quiet phase, plain without colour
- [x] `tui/model.go`: the tick, the idle pause, and the restart on the next keystroke
- [x] `tui/view.go`, `activity.go`, `history.go`: draw the selected row at this frame's phase

## The preference

- [x] `config`: the `tui.motion` key, its shipped value and its validation
- [x] `tui/settings.go`: the fifth row, its values and what each one means on this terminal
- [x] `cmd/tui.go`: resolve the key into the preferences, and write it back with the others
- [x] `cmd`: classify the new key, with a probe proving the resolved value reaches the interface

## Tests

- [x] an idle session produces no further frames, and a stray phase neither redraws nor rearms
- [x] the selection is marked and distinguishable from an unselected row at both phases
- [x] a session with no colour never pulses and renders no escapes
- [x] a session given no preferences at all animates, which is the SSH sandbox
- [x] stepping the row applies the change to the open frame and writes it down
