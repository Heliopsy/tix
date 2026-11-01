# Quick-add syntax

## Why

Adding a task with a priority, a tag, an owner and a date takes the one-line box plus a trip
through Add with details, or a command line with five flags. People already write those
attributes inline when they jot a task down ("Fix login !high #auth @raj due:fri"), so the
one line should be able to say them.

## What Changes

- New package `internal/quickadd`: one parser that turns a line into a title plus the
  attributes it names. Priority (`!high`, `!1`..`!5`), tags (`#auth`), assignee (`@raj`),
  due date (`due:fri`, `due:tomorrow`, `due:+3d`, `due:2026-11-20`), project (`+web`,
  `project:web`), parent (`^infra-3`) and initial status (`status:doing`). Long forms
  `priority:`, `tag:`, `assignee:` and `parent:` mirror the filter language's keys.
- Recognised tokens are taken out of the title. A backslash before a token, or quotes around
  text, keep it literal. A sigil whose value does not fit (`C#`, `#42`, `a@b.c`, `+1`,
  `!important`, a hashtag inside a URL) is ordinary title text.
- Surfaces: the web one-line add box always parses, shows a live preview of what will be set
  under the box and lists the syntax in its (i) tip; `tix task add --parse` opts in (literal
  stays the default so scripts are never rewritten); `POST /v1/tasks` takes `"parse": true`;
  the terminal interface's new-task prompt parses and previews.
- An unknown handle, project or parent refuses the add with the message under the box, and
  the line stays as typed.

## Impact

- New package `internal/quickadd`; no new Service method, no migration, no new API route.
- One new browser route, `GET /tasks/quickadd`, answering the preview fragment.
- `tix task add` gains `--parse`.
