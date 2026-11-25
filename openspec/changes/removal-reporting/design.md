# Design

## Deletion counts

`deleted` already named the field everybody read as "how many went", so it takes that meaning, and the trash
count, which only the web confirmation used, gets its own name, `trashed`. A plan states `would_delete` rather
than filling `deleted`, so no output of a dry run can be read as something having been removed. `would_delete`
is omitted outside a plan; `deleted` is always present, so a script reading it after a move gets zero rather
than a missing key.

## Removed people

A removed person's actor stays in the tenant, with no membership, so audit entries and task history still
name them. The listing decides membership the way `tenant user ls` already showed it: an actor with no
membership row is a former member.

Hiding them by default, rather than only labelling them, matches what the listing is for: the people who can
act in the tenant now. An administrator who needs the old rows passes `--include-removed`, and each one comes
back marked `removed`, so nobody mistakes it for a member without a role.

The filter is a `UserFilter` beside the page, the shape `ProjectFilter` has for `include_archived`. The web
and TUI screens offer no toggle: they are for managing current people, and a removed person has nothing left
to manage there. The activity screen asks for removed people too, since it resolves names for history.

`member rm` leaves the same state as `user rm` for a person who belongs elsewhere, so both are covered by the
same membership check.

An invite to somebody with an account in another tenant also creates an actor here with no membership, until
they accept. That person is not a former member, and the Users screen already lists them under pending
invites, so the listing leaves them out whether or not removed people are asked for.

The web fixture seeds its signed-in admin without a membership row, which no real sign-in has. Tests that
count that admin as a user add the membership first.
