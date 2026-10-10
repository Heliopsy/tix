# Design

## The screen is the one that already existed

The interface has a `tenant` view. Adding an `admin` view beside it would have given a reader two screens
about one subject and two keys to remember which, and the registry would then bind `identity.whoami` to one
and `tenant.show` to the other while both answer "which tenant is this". The view is extended instead.

That keeps a property the gating depends on. `capability.TUIAccess` offers a view when the reader may perform
at least one of the reads bound to it, and `identity.whoami` needs no scope, so the tenant view stays offered
to everybody. A reader without `tenant:admin` still reaches the screen and still switches tenant; the parts
they may not read say so in place. Had this been a new view bound only to tenant-admin reads, a reader without
that scope would have lost the switch as well, which is not what the refusal was about.

## What each part of the screen is for, and what it costs

One read per section, issued together when the view opens:

| Section | Read | On refusal |
| --- | --- | --- |
| attributes | `GetTenant` | the section says why, the rest of the screen stands |
| visible tenants | `ListTenants` | the section says why |
| domains | `ListDomains` | the section says why |
| members | `ListMembers` | the section says why, and handles go unresolved |

`ListActors` is not read here. It is read on the keystroke that opens the member form, for the reason the
assignee picker reads it on the keystroke: who a tenant has changes while a screen is open, and a directory
held for the session offers somebody who has left.

The member rows show handles. `ListActors` resolves them when the reader may list actors; when they may not,
a row shows the identifier it has rather than a blank, because a blank reads as a row that failed to draw.

## Why the cursor is over two kinds of row at once

A domain and a member are different things, and a separate cursor for each would need a key to move between
the lists. One cursor over both, with each row carrying what it is, means `X` always has a subject and the
confirmation always knows which verb it is asking about. This is the same shape the board's cursor has over
columns: one selection, rows that know their own kind.

A screen with no removable rows has no selection, and `X` then says the tenant has no domains or members
rather than opening a confirmation over nothing.

## Three forms, and why the split falls where it does

- **Edit.** Attribute first, the way the project edit form does it. `theme` is answered from the palettes the
  build carries plus the word for having none, so clearing a theme is a choice on the same list rather than an
  attribute with no way back. `name` is free text, so it hands to the prompt seeded with the current name.
- **Add.** `n` asks domain or member, because two additions behind one key need the key to ask which. A domain
  then needs a hostname, which no fixed list holds, so it hands to the prompt and nothing else. A member needs
  an actor and a role, both of which are fixed lists, so it is one form and never touches the prompt.
- **Remove.** Not a form. The row under the cursor is already the subject, so `X` goes straight to the
  confirmation.

`AddDomainInput` also carries a certificate mode with a certificate path and a key path. The paths are
filesystem paths on the server rather than on the reader's machine, and a mode offered without them leads to a
refusal, so the terminal adds a hostname that carries no certificate of its own and the operation records a
limitation saying where the rest is set. That is the same shape `artifact.put` already uses: the binding
exists and says in the same breath how far it reaches.

## Why `tenant.create` and `tenant.delete` stay gaps

The browser exempts both, and the reasons transfer: a session pinned to one tenant is not where another is
made, and deleting the tenant the session belongs to destroys the operator's own access mid-session. They are
left recorded as gaps rather than converted to exemptions, because the browser's reasoning is about a browser
session and the terminal's would be a second opinion written by somebody who did not try. Recording them as
gaps keeps them visible; converting them would close the question by assertion.

## What the tests read

Every assertion about this screen reads one section of it, through a helper that takes a heading and returns
the lines from it to the blank line that ends it. The sections share words with each other and with the footer:
`key` is a row of the attributes and `default` is both a tenant key and a value elsewhere on the page. An
assertion against the whole frame would pass on whichever copy matched, which is how a guard passes while the
thing it names is gone. The forms go through `formBlock` and the confirmation through `confirmLine` for the
same reason.
