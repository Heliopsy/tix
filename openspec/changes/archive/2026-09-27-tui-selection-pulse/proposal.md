# A selected row that pulses, and stops when nobody is there

## Why

The terminal interface draws the selected row in one colour and leaves it there. On a board wide enough to
fill a terminal that is a quiet cue, and a reader coming back to a screen they left ten minutes ago has to
find it again. A little motion answers "where am I" before the eye has to search.

Motion in a terminal is usually reached for through the blink attribute, which many terminals ignore and
some render as bold, so the same preference produces three different screens. It is also the wrong shape
for `tix ssh`: that listener renders server side and sends each frame down the wire, so an animation is
network traffic per frame per connected session. A public demo host with twenty idle sessions repainting
twice a second, for ever, is the failure worth designing against rather than the animation itself.

## What Changes

- **The selected row pulses**, by alternating its emphasis between two styles on a ticker, so every
  terminal draws the same thing. Nothing else moves: no transitions, no spinners, no flash on a state
  change.
- **The pulse stops after a period with no keystroke, and resumes on the next one.** An idle session
  produces no frames at all. This is the requirement, not an optimisation.
- **It rests at full emphasis**, so a session that has gone quiet draws exactly what a reader with motion
  off draws, and a stopped pulse cannot be mistaken for an interface that died.
- **The selection stays unambiguous at both phases.** The pulse varies emphasis; the marker and the
  selection's colour are drawn in every frame.
- **A terminal getting no colour never pulses at all.** Without colour every style in the theme is the
  same empty style, so a pulse there would repaint identical frames: `NO_COLOR`, `TIX_NO_COLOR`,
  `output.color = never` and a destination that is not a terminal all degrade to a static selection.
- **The settings screen carries it as a fifth display preference**, `tui.motion`, on by default, written
  to the configuration file at once and showing which configuration layer supplies the value, exactly like
  the four rows already there.

## Impact

- `internal/tui`: a new `motion.go` holding the pulse's rules as pure functions, a second selection style
  on the theme, a settings row, and the model's tick.
- `internal/config`: one new key, `tui.motion`, validated against the two values it takes.
- `cmd/tui.go`: the key resolved into the preferences the interface opens with, and written back by the
  existing preference writer.
- No new dependency, no new store, no change to any other surface: the browser has its own motion story
  and is untouched.
