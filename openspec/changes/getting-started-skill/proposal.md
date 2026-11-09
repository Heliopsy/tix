# A getting-started skill for new users

## Why

The `tix` skill assumes tix is already reachable: its first step is `tix whoami`, and it goes straight to
claiming and leases. An agent helping a person who has never used tix has nothing to go on. It does not know
whether to install a desktop app, open a browser, set up the CLI or connect over SSH, and it has no plain
explanation of what an instance, a tenant or a workflow is. It also has no guidance on trying tix without
writing into somebody's real store.

## What changes

- A second skill, `skills/tix-getting-started/SKILL.md`, chosen by its description when a person is new to tix
  or asks how to start or what a tenant is.
- It makes the agent ask how the person wants to use tix and where their tix lives before anything else, explain
  the concepts with one running example, walk the setup for the chosen interface, take them through a first task
  and hand over to the main skill.
- Safety rules: never write to a real store without asking, prefer a scratch `--db`, never print a secret.
- The README, the docs index and the main skill point newcomers at it.
- Guards in `cmd/skill_getting_started_test.go`: front matter on every skill, commands and flags checked against
  the real tree, the version marker an ancestor of HEAD, no relative link out of the skill, and the README
  pointers.

No behaviour of the binary changes.
