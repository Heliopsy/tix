# Design

## Templates are files, not rows

A template is a YAML document embedded with `go:embed` and parsed once. Nothing about it is tenant data
until it is applied, so there is no table, no migration and nothing to seed. A test parses every file with
known-fields decoding, runs the workflow validator over each definition and the field validator over each
field, and checks every state has a route to a final one.

## Fields live beside the definition

Custom fields belong to projects (`PutFieldDef` takes a project), and a workflow is shared by any number of
projects, so a field cannot be part of a workflow definition. A template therefore carries a `fields` section
next to `definition`. Applying a template on its own creates only the workflow; creating a project with
`workflow_template` creates or reuses the workflow and puts the fields on the new project, all in one
transaction with the project's audit entry and event. That keeps one write per operation and makes the
browser's New project dialog a single submission.

## The reserved prefix

Template keys start with `tix-`. `PutWorkflow` refuses a new workflow under that prefix, so a `tix-` key in a
tenant always came from a template; an existing one can still be redefined by key, which is how a team
adjusts a template it applied. Applying a template is allowed under any key, including another `tix-` one
(rename picks `tix-bugs-2`), because it is still made from a template. Bundle and snapshot imports write
workflows directly and are not refused: a `tix-` workflow from another installation came from a template
there. Display names carry no prefix; pickers draw a Built-in label instead.

## Collisions

`ApplyWorkflowTemplate` reuses the bundle vocabulary: no policy refuses with a conflict, `skip` returns the
existing workflow, `rename` takes the first free `KEY-N`, `replace` redefines the existing one through the
same removed-state check `PutWorkflow` runs. Replacing the builtin `default` is refused.

## The approve scope

Approving agent work or a request is a decision the worker must not be able to make. `task:transition` is
held by every member and every agent token, so it cannot gate that. `task:approve` is new, carried by no role
but admin (`*`), and given to a reviewer's token by name. It sits in `AllScopes`, so token creation accepts
it and the OpenAPI scope enum lists it.

## Adaptations to the requested shapes

- GTD `waiting` and support `waiting on customer` use the `waiting` category, which the vocabulary defines as
  stalled on something outside the board, rather than `blocked`, which means stalled on something inside it.
- Support has an `urgency` enum rather than a `priority` field: every task already carries a priority, and two
  fields named Priority on one task would be read as one.
- Support `resolved` is in the `done` category but not terminal, so a reply can reopen it while boards and
  statistics already count it as answered.
- QA sending work back in `tix-dev-review` needs a comment, like review does; a failed verification and a
  regression in `tix-bugs` do too.

## Translation

Names, purposes and state labels are catalog entries in their own context (`workflow template`), so a label
such as Open or Done can be translated for a template without disturbing the same word elsewhere. The web
completeness test derives the keys it expects from the templates themselves.
