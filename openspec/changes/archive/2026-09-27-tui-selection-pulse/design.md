# Design

## Why a ticker rather than the terminal's blink attribute

SGR 5 is the obvious mechanism and the wrong one. Terminals disagree about it: some ignore it, some render
it as bold, some blink at a rate the application cannot see or choose. A preference that produces three
different screens is worse than no preference. Toggling a style from a ticker puts the rate and the
appearance in one place, and every terminal that can draw a colour draws the same pulse.

## The period: 800ms a phase, 1.6s a cycle

The period is a bandwidth figure as much as a visual one, because `tix ssh` renders server side. Two
constraints meet in the same range: under about half a second a changing row reads as a blink and takes
attention away from the work, and every phase is a repaint on somebody's connection. 800ms is a little over
one frame a second, slow enough to read as a breath rather than a warning light, and a tenth of what a
conventional animation would cost. A calmer pulse is also the cheaper one, so nothing is traded here.

## The idle timeout: 45 seconds

Long enough that the motion does not stop while somebody is still reading: a task with a body and a
comment thread takes longer than half a minute to read, and 45 seconds clears that with room. Short enough
that an abandoned session goes quiet inside a minute, which is what a demo host needs.

Stopping is invisible except for the absence of motion, because the pulse rests on full emphasis. A reader
who looks up after a minute sees the same selected row a reader with motion off sees, so the interface
cannot read as frozen: nothing else on the screen was moving either.

## An idle session sends nothing

The pulse is a chain: each phase schedules exactly the next one. When the phase that arrives finds the
session idle, it schedules nothing, and since nothing else in the interface repaints on a timer, the
session goes silent until a key arrives. A keystroke records the time it arrived and starts the chain again
if the pause had ended it.

The guard is therefore about frames rather than about a timer field: the chain is driven until the model
asks for no further phase, the frame count is held against the timeout, and one more phase is delivered
afterwards to prove it neither redraws nor rearms.

The SSH listener's own idle timeout is unaffected. Its activity tap is fed by key and mouse messages only,
so a pulse cannot keep an abandoned session alive, for the same reason a keepalive cannot.

## No second signal for suppressing motion

There is no settled environment variable for "no motion" the way there is for `NO_COLOR`, and inventing a
second spelling would be a second place to disagree with the configuration file. `tui.motion` already
carries `TIX_TUI_MOTION` through the five layers, which is the signal.

What is honoured is `NO_COLOR`, and everything else that turns colour off, because the pulse needs colour
to exist at all: without it every style in the theme is the same empty style, so motion would cost frames
and show nothing. A terminal that cannot do what the pulse needs therefore degrades to a static selection
rather than to invisible traffic.

## Emphasis, not presence

The quiet phase drops the selection's weight and keeps its colour, and the marker `▸` is drawn by the same
function in both phases. A reader who looks mid-cycle sees a coloured, marked row either way; what changes
is only how loudly it is drawn.

## An enumeration rather than a boolean

`tui.motion` takes `on` or `off` rather than `true` or `false` so that an unset value reads as unset rather
than as off. The hosted SSH sandbox passes no preferences at all, and an empty motion preference there has
to mean the shipped default, which is on.
