# Account and security in the terminal

## Why

Eleven terminal gaps sit in one place: API tokens, SSH keys and the second factor of the person who is
signed in. The browser manages all three on its Tokens, SSH keys and Settings screens and the command line
under `tix token`, `tix user key` and `tix user totp`, while a terminal session had no way to issue a token
for an agent, enrol a second laptop's key or turn on a second factor. Over SSH that is the session a person
actually has open.

The account log was exempted on the grounds that an account spans every tenant and a terminal session is
pinned to one. The browser shows the same log at `/account` from whichever tenant the session is in, and the
service decides who may read it, so the tenant a session is pinned to was never the reason to hide it.

## What Changes

- A new terminal view, **account**, opened with `$` and from the command palette. It states who the session
  is signed in as, the second factor, the reader's own API tokens, the reader's own SSH keys and the newest
  page of the account log. Each section is read on its own and a refused one says why without costing the
  rest of the screen.
- `n` issues a token, enrols an SSH key, starts second-factor enrolment, or replaces the recovery codes,
  offering only what the reader's authority and the factor's state allow.
- A token is issued from a form (name, expiry from the browser's choices, a preset to start from) and then
  the browser's scope picker, drawn as the same groups with the same notes. A scope the reader does not hold
  is shown and cannot be ticked. The value is shown once, at the top of the screen, until dismissed.
- An SSH key is enrolled from a form taking the `authorized_keys` line and a label.
- Enrolment draws the otpauth URI as a scannable QR code where the terminal has colour and room for it, and
  always the grouped key and the URI as text. `enter` asks for the code; the recovery codes are shown once.
- `X` revokes the selected token or key, or removes the second factor, behind a confirmation naming it.
- The scope groups, their notes and the token expiry choices move from `internal/web` to `internal/onboard`,
  so the browser and the terminal draw one list.
- The registry binds `token.create`, `token.list`, `token.revoke`, `sshkey.enrol`, `sshkey.list`,
  `sshkey.revoke`, `totp.status`, `totp.enrol`, `totp.confirm`, `totp.codes`, `totp.remove` and `account.log`
  to the account view. The terminal gap count falls from 44 to 33. `token.list` and `sshkey.list` record a
  terminal limitation: the view lists the reader's own, where the command line takes another actor.

## Capabilities

### Modified Capabilities

- `tui`: the account view and its actions.
- `capability`: twelve operations gain a terminal binding, two of them with a limitation.

## Impact

`internal/tui` (new `account.go`, hooks in the view, key, form and confirmation switches),
`internal/onboard`, `internal/web/scopepick.go` and `admin.go`, `internal/capability/registry.go` and its
tests, `docs/tui.md`, `docs/second-factor.md`, `skills/tix/reference/ssh.md`. No HTTP change, so
`api/openapi.yaml` is untouched.
