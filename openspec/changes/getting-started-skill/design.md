# Design

## A separate skill, not a section of the main one

The main skill is loaded for every queue an agent works. A first-hour walkthrough in it would cost every one of
those agents context for something they need once, and its description would have to cover two very different
moments. A separate skill with its own description lets an agent pick the right one: getting started when a
person is new, `tix` for the work itself.

## One file, absolute links

The skill ships as a single `SKILL.md` with no reference folder. The site mirrors `skills/tix` with its
`reference/` files under `/reference/`; a second skill with its own `reference/` would collide there. Links out
of the skill are absolute (`https://tix.red/skill.md`, the guides on GitHub), so the file works when installed
on its own or mirrored on tix.red. A guard refuses a relative link to a file the skill does not ship.

## Same guards as the main skill

The skill is followed literally, so the command and flag check, the single `verified-against` marker and the
ancestor check apply to it as well. The skill helpers in `cmd/skill_test.go` take a directory so both skills
share them. The paragraph naming the test file starts with the same sentence as the main skill's, which is what
the site's sync matches to replace it.

## Workflow templates

The `tix-*` workflow templates land in a separate change. The skill names them conditionally and tells the
agent to check the build for a `template` subcommand, so it is correct with or without them.
