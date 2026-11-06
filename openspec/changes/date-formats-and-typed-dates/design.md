# Design

## Where the order comes from

`output.TimeStyle` already decides how an instant is written, so it also answers how a typed date is read:
`DateOrder()` is `DateDayFirst`, `DateMonthFirst` or `DateISO`, and `Date(t)` writes a calendar date the way
that reader would type it. The web gets the style from the `tix_time_format` cookie, the TUI and the CLI
from `output.time_format`. `short` and `long` write the month as a word, so they say nothing about the
order of digits and read digits as ISO only.

## Parsing

`quickadd.ParseDueIn(value, now, order)` is the one reader. It takes the calendar words and offsets
`ParseDue` already took, then YYYY-MM-DD and RFC 3339, then a month-name date, then the digits layouts the
order names: `2.1.2006` and `2/1/2006` day first, `1/2/2006` month first. A dotted date is never read
month first. Two-digit years are refused. `ParseDue` and `Parse` keep their meaning, ISO only, which is
what the HTTP API calls.

## Examples

`quickadd.DueExamples(date)` and `core.RecurrenceExamples` are the lists every hint draws from. The date
example is `quickadd.ExampleDay` (20 November 2026, after the 12th so the order is visible) written with
`TimeStyle.Date`. A quick-add example falls back to YYYY-MM-DD when the reader's date has spaces in it,
because a quick-add line splits on spaces.

## Trade-off

The due field used to be `<input type="date">`, which gave a browser date picker. A picker cannot take
`tomorrow` or `+3d` and submits ISO whatever the reader typed, so the field is now text. The snooze menu
keeps its `datetime-local` picker, which is not free text.
