# Tasks

## 1. A tenant administrator sees and revokes another actor's token in the browser

- [x] 1.1 `internal/store/store.go`: `Tx.GetToken`, so a caller about to change a token can record whose it
      was
- [x] 1.2 `internal/store/sqlite/auth.go`, `internal/store/postgres/auth.go`: both engines
- [x] 1.3 `internal/service/token.go`: `RevokeToken` reads the token, and records `token.revoke_other` when
      the holder is not the caller
- [x] 1.4 `internal/web/admin.go`: `tokenRow`, `tokenListing` gated on `tenant:admin`, `labelTokens`
      ordering by owner
- [x] 1.5 `internal/web/admin.go`: the token route declares `ListActors`, which it now calls
- [x] 1.6 `internal/web/templates/tokens.html`: the owner column, the note above the table, and the
      confirmation naming whose token it ends
- [x] 1.7 `internal/capability/registry.go`: `token.list`'s `Limitation` rewritten to the narrowing that
      remains
- [x] 1.8 `internal/web/tokenowner_test.go`: a non-admin sees only their own and is offered no control for
      another's; an admin sees others' with the owner named and the control naming them; the revocation
      works
- [x] 1.9 `internal/service/tokenrevoke_test.go`: both revocations, so "the other one is recorded" is
      distinguished from "everything is recorded that way"
- [x] 1.10 `internal/web/tokens_test.go`: `tokenCellUnder` reads a cell by its column heading, because the
      positional helper silently skipped any cell carrying an attribute
- [x] 1.11 `openspec/changes/archive/2026-09-27-scope-aware-tui-navigation`: the scenario and the design
      note recording the old narrowing as deliberate, amended in place

## 2. `tix token create` defaults to a 90-day expiry

- [x] 2.1 `cmd/parse.go`: `DefaultTokenExpiry`, `ExpiryNever` and `parseExpiry` — duration, date,
      timestamp, or `never`
- [x] 2.2 `cmd/auth.go`: the flag carries the literal default, and the long description names the breaking
      change
- [x] 2.3 `cmd/tokenexpiry_test.go`: the default, `never`, an absolute date, the flag's own help line, and
      the parser's table
- [x] 2.4 `docs/agents.md`: the breaking change, the grammar, and the note that the HTTP API keeps no
      default of its own
- [x] 2.5 `skills/tix/SKILL.md`: the stated default, which `cmd/skill_test.go` checks against the flag

## 3. `webhooks.html` renders the generated signing secret it promises

- [x] 3.1 `internal/web/secret.go`: `oneTimeSecret`, the two constructors, and the one-time cookie helpers
      the token screen's own code now uses
- [x] 3.2 `internal/web/templates/partials.html`: the `secret` partial, extracted from `tokens.html`
- [x] 3.3 `internal/web/templates/tokens.html`: renders through the partial
- [x] 3.4 `internal/web/webhooks.go`: `putWebhook` carries a generated secret to the screen in a one-time
      cookie; `showWebhooks` reads and clears it
- [x] 3.5 `internal/web/templates/webhooks.html`: renders the partial, and the help text and the meta line
      are true for a generated secret, a supplied one, and a blank save of an existing endpoint
- [x] 3.6 `internal/web/webhooksecret_test.go`: shown once, absent afterwards, never for a supplied secret,
      and the help text says both
- [x] 3.7 `docs/api.md`: what a generated secret does and where it is shown

## 4. Verification

- [x] 4.1 Watch every guard fail: break the behaviour, quote the failure, restore against git, confirm
      `git diff` is empty
- [x] 4.2 `just check`
- [x] 4.3 `just test-postgres`
- [x] 4.4 Drive all three by hand against a scratch database under this agent's own directory, never the
      zero-config store
