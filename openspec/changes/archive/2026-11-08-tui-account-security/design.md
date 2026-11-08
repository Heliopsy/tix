# Design

## One view, the person's own

The browser splits this over three screens because a page has room for one table. A terminal session is one
person in one tenant, and what they come to do is short: issue a token for an agent, enrol a key, turn on the
second factor, check what changed. One view holds all of it, in sections, the way the tenant screen holds the
tenant, its domains and its members.

The tokens and keys listed are the reader's own. The browser's Tokens screen widens to the tenant's tokens for
a reader holding `tenant:admin`, and the command line takes any actor; reaching somebody else's credentials
belongs with user administration, not with "my account". Both listings record that as a terminal limitation
rather than claiming the whole operation.

## Selection and keys

One cursor runs over the second-factor row, then the tokens, then the keys, so `X` always knows which of the
three it is removing. The keys are the board's own (`n`, `X`, `enter`, `esc`, and in the scope picker the
inbox's `space` and `a`), so every keybinding scheme drives the view without new bindings. `$` opens it: every
letter worth having is spoken for, `@` went to the people screen, and `$` is the shell's prompt for an ordinary
user, the one that means "you".

`n` opens a one-field form asking what to add, offering only what the reader may do and what makes sense
now: `second factor` while none is enrolled, `recovery codes` once one is.

## Tokens

The form gathers the name, the expiry and a preset. The expiry choices are the browser's, 7, 30 and 90 days,
a year or never, proposing 90 days. The presets are `onboard.Presets` filtered to the ones the reader holds,
plus `custom`.

The scope picker then takes the body of the screen, because it is twenty-odd rows and the form panel is drawn
under the body. It is the browser's picker: the same groups in the same order, each scope with the same note,
`*` offered first and only to a reader who holds it, and a scope the reader does not hold drawn and marked
rather than left out. `space` ticks one, `a` ticks or clears the group under the cursor, `enter` issues.
The groups, notes and expiry choices move to `internal/onboard` so the two surfaces read one list; the
browser keeps unexported aliases so its templates and tests are unchanged.

The issued value is held in the model only until the reader dismisses it with `esc` or leaves and reopens the
view. It is never put in the status bar, where the next message would replace it before it was copied.

## Second factor

`BeginTOTPEnrolment` hands back the secret once. The view draws it with `qr.Matrix.Terminal`, the same half
block renderer `tix login` uses, under two conditions: the theme has colour, since the renderer states its
own colours and a terminal stripping escapes would receive a block of identical characters, and the symbol
fits the body whole, since half a symbol does not scan. Otherwise it says which condition failed. The key,
grouped in fours as the browser groups it, and the URI are always drawn, so enrolment never depends on the
code.

`enter` opens a one-field form for the code. A refused code leaves the enrolment on screen with the reason in
the status bar. `esc` abandons it, which the service already treats as a pending enrolment granting nothing.

Removing the factor is offered whenever one is enrolled. The service refuses it on the surface the session
came in on when the tenant requires a factor there, and that refusal is the message the reader sees: the
terminal cannot tell from inside the model whether it is a local session or an SSH one, and a second copy of
the rule here would be a second opinion about it.

## Account log

`account.log` was exempted because "an account spans every tenant ... and a terminal session is pinned to
one". The browser reads it at `/account` from any tenant session, and the authority to read it is the
service's (`CanReadAccount`): a session, or a token carrying `account:read`. An SSH session is a session, so
the person reads their own log there as they do in the browser. An agent or a token without the scope is
refused by the service and the section says so. The view reads the newest page, as `tix account log` and the
browser do, and names the command for older entries.

## Alternatives

- **Three views.** Rejected: three more letters out of a keymap that has none left, and three screens each
  too thin to justify itself.
- **A giant form for the token.** Rejected: twenty-five switches in the panel under the body do not fit a
  24-row terminal, and a form has no headings to group them under.
- **Listing the tenant's tokens for an administrator.** Deferred to user administration, which is where
  another person's credentials are reached.
