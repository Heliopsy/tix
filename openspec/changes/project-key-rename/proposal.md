# A project's key can be changed

## Why

A reader reported there is no way to change a project's key. The key is chosen once, at creation,
and it is spent everywhere afterwards: in the project's URL (`/projects/infra`), in filters
(`project:infra`), and in every task reference (`infra-8`). A typo, a rename of the team or a
merge of two areas leaves the project stuck with a key that no longer says what it is. Name and
description were already editable on every surface; the key was not editable on any.

## What Changes

- A project update accepts a new key. The change is one transaction that rewrites the project row,
  records the old key as an alias of the project, and writes the audit entry and the
  `project.updated` event, whose payload names the previous key.
- The new key follows the existing key rules and is unique within the tenant. A key that is the
  current key or a kept alias of another project is refused with a conflict.
- Old keys are kept as permanent aliases. A project, a task reference and a filter that name an old
  key resolve to the renamed project, and every response carries the canonical new key and ref. A
  project may take back one of its own old keys. Aliases go away only with the project.
- In the web UI, `GET /projects/<old>` and `GET /tasks/<old>-8` (and the screens under them)
  redirect permanently to the new location.
- Web: the project settings form gains a key field with an explanation, and a changed key is
  confirmed on a second step that shows example references before and after.
- CLI: `tix project edit KEY --key NEW`. HTTP API: `PATCH /api/v1/projects/{ref}` with `key`. TUI: the
  project edit form offers the key attribute.
- A snapshot import that names a project by an old key updates the renamed project and keeps its
  current key.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `data-model`: project keys can change; old keys are aliases.
- `task-filtering`: a project term may name an old key.
- `web-ui`: key field with confirmation, redirects from old project and task URLs.
- `cli`: `--key` on `tix project edit`.
- `http-api`: `key` on the project PATCH body; lookups by old key.
- `tui`: the project edit form offers the key.
