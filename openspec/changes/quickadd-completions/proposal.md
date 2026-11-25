# Completions in the quick-add box

## Why

The filter box beside quick add suggests tags, people and projects as they are typed, and the
tix app's quick add sheet completes `#tag`, `@handle` and `+project`. The web quick-add box offered
nothing, so a reader had to remember the exact tag or handle, and a typo only showed up as a red chip
in the preview or a refused add.

## What Changes

- The quick-add box completes the word under the caret when it opens with `#` (the tenant's tags),
  `@` (the people) or `+` (the projects, each noted with its name). A bare word gets nothing, since
  it is the title.
- It reuses the filter box's suggester (`filterbar.js`): the box carries `data-suggest` and
  `data-suggest-mode="quick"`, and the mode picks the quick-add decisions in `decide.js`.
- Taking a suggestion keeps the sigil and adds a space. A value the sigil form cannot carry is
  written in its keyed form (`tag:c++`), and one holding a space is not offered, because the quick-add
  parser has no way to express it.
- Enter with no suggestion reached still adds the task.
- On a project page the box is still offered every project, because `+key` adds to another project
  there; the filter box keeps leaving projects out.

## Capabilities

### Modified Capabilities

- `web-tasks`: completions in the quick-add box.

## Impact

`internal/web/assets/decide.js`, `internal/web/assets/filterbar.js`, `internal/web/filterbar.go`,
`internal/web/templates/tasks.html`, `internal/web/i18n.go` and a Bulgarian string,
`internal/web/jstest`, `docs/web-ui.md`.
