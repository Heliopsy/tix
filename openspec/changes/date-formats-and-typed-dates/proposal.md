# Date formats, and dates typed the way they are read

## Why

The due date, repeat rule and quick-add fields only showed example syntax in the refusal after a bad
entry, so the first thing a person learned about the syntax was that they had got it wrong. The date
format setting offered ISO, RFC 3339, short, US and relative, with nothing for the day-first dates
most of Europe writes (06.11.2026, 06/11/2026), and a date typed into the web's due field had to be
YYYY-MM-DD however the reader's dates were shown to them.

## What Changes

- The (i) tips of the due date fields (Add with details, the task screen's Due row), the repeat fields
  (Add with details, the task screen, a template's edit form) and the quick-add box list values to type
  before anything is typed. Each example comes from the parser's own list and a test runs every one shown
  through the parser.
- `output.time_format` gains `long` (21 Sep 2026 14:05), `dotted` (21.09.2026 14:05) and `dayfirst`
  (21/09/2026 14:05), offered in the web settings, the TUI settings and the configuration file alike.
- A date a person types is read in the order their format writes dates: day first under `dotted` and
  `dayfirst`, month first under `us`. `YYYY-MM-DD` and a month written as a word are read under every
  format. Digits in an order the format does not name are refused, never guessed.
- The web's due date fields become text fields that take a word (`tomorrow`, `fri`), an offset (`+3d`),
  or a date, and show the saved day back in the reader's format.
- The quick-add line follows the same rule in the browser, the TUI and `tix task add --parse`.
- `--due`, the HTTP API (including `"parse": true`) and the MCP tools keep reading `YYYY-MM-DD` and
  RFC 3339 only. A machine interface must not depend on a browser cookie or a person's preference.
