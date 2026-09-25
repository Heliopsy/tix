# Design

## Where a terminal reader's preference lives

The browser stores a display preference in a cookie, one per browser, because the browser has no other
place to put it. A terminal has no cookie, and inventing a store would mean two places holding a timezone.

The four preferences are written to keys the CLI already resolves:

| Row | Key | Shared with |
| --- | --- | --- |
| keys | `tui.keymap` | the `--keys` flag |
| time format | `output.time_format` | every table the CLI prints |
| timezone | `output.timezone` | every table the CLI prints |
| colour | `output.color` | every stream the CLI writes |

Three of the four are therefore not per-surface: choosing US dates in the terminal interface changes what
`tix task ls` prints. That is the intended behaviour and the screen says which key each row writes, because
a reader who can see the key can predict the consequence. The alternative, a `tui.` copy of each, would
mean a reader whose terminal and command line disagree about what two o'clock means.

## Why the write is immediate

A preference screen with an explicit save has two states a reader can leave it in, and the one where the
change is applied but not written is indistinguishable from the one where it is. Cycling a value is already
a deliberate key press, so the press is the commit. The status line names the file that was written.

`config.SaveChanges` is what performs it, not `config.Save`: a `Config` always holds a value for every key,
so saving one would pin the defaults and whatever the environment happened to be supplying. That defect
already cost `tix tenant use` a user's DSN, and the writer here is a guarded test away from repeating it.

## Why the layer is shown

The file is the fourth of five layers. A reader whose `TIX_OUTPUT_TIMEZONE` is set can change the timezone
row, see the frame follow it, watch the file be written, and find the old zone back on the next run with
nothing having failed. The row says which layer supplies it and that the layer still wins after a restart.

## What is deliberately not here

The target, the token, the listen address, the log destination, the retention windows and the webhook
settings are deployment configuration. A session is already connected through the target it resolved, so
changing it on this screen would edit a file and change nothing a reader can see, which is the worst
possible feedback. The screen names `tix config show --sources` instead, which answers the same question
across every key and every layer.

## Colour

`output.color` is the CLI's own three-way mode. `auto` resolves against the probe this run already made,
and the row says what auto currently decides rather than what it decides in general, so a reader looking at
a monochrome screen is told why. `NO_COLOR` keeps working: the configuration layer folds it into
`output.color` at the environment layer, so it arrives here as an ordinary override, warned about like any
other, rather than as a special case this screen has to know about.

## Keeping the screen reachable

The body is taller than a 24-row terminal. The cursor walks the four settings; once it is at an end, or has
been scrolled out of the window, the same key scrolls the body, so the session facts below are reachable
without a second scrolling concept. The keybinding preview is drawn whatever row is selected, because a
body whose height depends on the cursor moves the content out from under it.

The column keys are relabelled in this view: `←/→` step a setting's values, and a footer that promised
"column left" on a screen with no columns was describing the board.
