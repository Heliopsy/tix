# Design

## One parser, applied at the edge

`quickadd.Parse(line, now)` is pure: it returns a `Result` holding the title, the attributes
and the tokens it recognised, in order. It does not resolve anything. Handles, project keys,
parent references and status names are resolved by `Service.CreateTask` exactly as they are
for the explicit fields, so an unknown one is refused with the same not-found error and field
the explicit form gets. `Result.Apply` folds the result into a `core.CreateTaskInput` and
refuses a line that contradicts a field set explicitly (`--priority low` with `!high`).

The parser runs on the surface rather than in the service because relative dates are a
question about the reader's calendar. The command line and the terminal interface resolve
`due:fri` in the local zone, the browser in the zone chosen under Settings, and the HTTP API,
which knows no zone, in UTC. Each surface states its present; the parser never reads a clock.

A date without a time is stored as midnight UTC of that calendar day, the instant `--due
2026-11-20` and the date control already give.

## Syntax

| Token | Sets | Notes |
| --- | --- | --- |
| `!highest` `!high` `!normal` `!low` `!lowest`, `!1`..`!5`, `priority:X` | priority | 1 is highest, the existing scale |
| `#tag`, `tag:X` | tag, repeatable | `#` needs a letter after it, so `#42` stays text |
| `@handle`, `assignee:X` | assignee | |
| `due:X` | due date | `today`, `tomorrow`, a weekday (`fri`, `friday`: the next one, never today), `+3d`, `+2w`, `YYYY-MM-DD`, RFC 3339 |
| `+key`, `project:key` | project | overrides the box's project picker |
| `^ref`, `parent:ref` | parent | |
| `status:X` | initial status | any state of the project's workflow |

A token is a whole whitespace-separated word. A sigil only counts at the start of a word and
only when what follows is a valid value, so `C#`, `a@b.c`, `https://x/#frag`, `+1` and
`!important` are title text. A keyed form (`due:`, `project:`, `status:` and the long forms)
whose value is wrong is refused instead, because the key says the writer meant it.

Escapes: `\#tag` is the literal `#tag`; a run in quotes (`"…"`, `„…“`, `“…”`, `«…»`) is kept
as written, quotes included, so `Rename "#main" to "#trunk"` keeps both names. A backslash
before anything that is not a token is left alone, so `C:\temp` survives.

A field named twice with different values (`!high !low`) is refused rather than resolved by
position.

## Why `--parse` is opt-in on the command line

Scripts pass titles they did not write: issue titles, commit subjects, file names. Parsing
by default would quietly turn "Bump #42 @types/node" into a different task. The web box and
the terminal prompt are typed by a person who can see the preview, so they parse always; the
HTTP API and the CLI parse when asked.

## Preview

The browser asks `GET /tasks/quickadd?title=…` after a short pause in typing and swaps the
answer under the box: one chip per recognised token, and a refusal line when the line cannot
be parsed or names a project or handle this tenant does not have. The terminal interface
draws the same summary as a row under its prompt. Neither is required: submitting parses
again and the service has the last word.
