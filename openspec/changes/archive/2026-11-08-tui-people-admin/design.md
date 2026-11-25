# Design: the people screen in the terminal

## Context

The browser's Users screen lists accounts with their role and state, opens a dialog per account to change
the name, role, disabled state and forced password change, clears a second factor, deletes, and has a create
dialog with two modes: set a password, or send an invite link. Pending invites sit under the list with
Re-issue and Revoke. The CLI covers the same ground under `tix user` and `tix user invite`.

The terminal already has the shapes this needs: forms with typed and cycled fields, confirmations that name
their subject, and per-view action lists gated on the registry.

## Goals / Non-Goals

**Goals:**

- Every operation the Users screen calls, reachable from one terminal view.
- The view and each key offered only to readers whose authority reaches them.
- Changes to other agents' views kept to the shared switches a new view has to appear in.

**Non-Goals:**

- Global administration (`tix tenant user`, global admins): a terminal session is pinned to one tenant.
- Paging past the first page of users. The view reads one page and says when there is more.

## Decisions

- **One view, one cursor over users then invites.** The removal key acts on either, as the tenant screen's
  does over domains and members, so the confirmation always names a subject.
- **The board's keys, not new ones.** `n`, `e`, `X`, `enter`, `R` (renew, here re-issue) and `F` (here the
  second factor) are bound already and moved by every scheme, so the screen needs one new binding, the `@`
  that opens it. `@` is the mark the interface already puts in front of a handle.
- **The gate is `user:admin`, as the service enforces it.** The view is offered when the reader may perform
  one of its reads, which is how every view is gated, and the reads need `user:admin`. Gating on
  `tenant:admin` instead would be a second opinion the service does not hold: a token carrying `user:admin`
  can make every one of these calls from the command line. The admin role holds both.
- **An edit sends only what changed.** The form opens on the current values and the update carries the
  fields the reader changed. Sending the forced password change unconditionally, as a form post does, would
  turn every edit of somebody who also belongs to another tenant into an `account_in_other_tenants` refusal.
- **The refusal is translated, not swallowed.** A refusal whose detail reason is `account_in_other_tenants`
  is reported as "@ada also belongs to another tenant: only they or a global admin can change their
  password, second factor or account", and the rest of the change is not retried behind the reader's back.
- **A password field is masked.** A form field can be marked secret: the summary shows bullets and the input
  echoes bullets while it has the cursor. Nothing else in the form changes.
- **The invite link is held in the view's state, once.** It is drawn at the top of the view until the reader
  leaves the view or performs another action, matching the browser's one-time dialog. A link with no absolute
  URL says to put the address people open tix at in front of the path, as the CLI does.

## Risks / Trade-offs

- A link on screen can be read over a shoulder. The browser shows it the same way; the CLI prints it to a
  terminal too. It is cleared as soon as the reader does anything else.
- Other views are being added in parallel and touch the same switches. The view keeps its state in one field
  and its own file, so a rebase is a handful of one-line conflicts.
