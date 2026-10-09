# Tasks

## 1. A refusal says which field it is about

- [x] 1.1 `internal/core/errors.go`: `DetailField` and `FieldOf`, so the attribution travels on the error
      rather than being reconstructed from message text
- [x] 1.2 `internal/core/input.go`: `CreateTokenInput.Validate` names `name` and `scopes`
- [x] 1.3 `internal/service/tokenname_test.go`: each refusal names its field

## 2. A refused form re-renders in place

- [x] 2.1 `internal/web/form.go`: `fieldErrors`, `refusal` and `correctable` — the reusable half, with the
      note saying why a kind the reader cannot act on still goes to the error screen
- [x] 2.2 `internal/web/templates/partials.html`: the `field-error` partial
- [x] 2.3 `internal/web/assets/app.css`: `.field-error` and the invalid-control border
- [x] 2.4 `internal/web/admin.go`: `tokenForm`, `renderTokens`, `refuseToken`; `createToken` answers a
      correctable refusal with the form
- [x] 2.5 `internal/web/templates/tokens.html`: values preserved, messages placed, `required` on the scope
      control
- [x] 2.6 `internal/web/tokens_test.go`: an empty scope selection keeps the name and does not reach the
      error screen

## 3. A live token's name is unique within its tenant

- [x] 3.1 `internal/store/store.go`: `Tx.TokenNameInUse`
- [x] 3.2 `internal/store/sqlite/auth.go`, `internal/store/postgres/auth.go`: both engines
- [x] 3.3 `internal/store/migrations/0010_token_name_unique.sql`: rename existing duplicates, then the
      partial unique index
- [x] 3.4 `internal/service/token.go`: `checkTokenName`, inside the same transaction as the insert
- [x] 3.5 `internal/store/sqlite/seq_test.go`: the hand-rewind list learns about the new index
- [x] 3.6 `internal/store/sqlite/tokenname_test.go`: the upgrade path with duplicates already present, the
      tenant scoping of the check, and that a revoked name does not block the index
- [x] 3.7 `internal/service/tokenname_test.go`: refused as a conflict naming the field, freed by revoking,
      and per tenant rather than globally

## 4. The listing says what it has

- [x] 4.1 `internal/web/columns.go`: the `created` column, on by default like its neighbours
- [x] 4.2 `internal/web/render.go`: `tokenState`, answering from `core.APIToken.Active` so the badge cannot
      disagree with whether the credential works
- [x] 4.3 `internal/web/templates/tokens.html`: Created and State columns, "Never" for no expiry, no revoke
      control on a revoked token, revocation behind a disclosure that names what it ends
- [x] 4.4 `internal/web/assets/app.css`: the token state badges
- [x] 4.5 `internal/web/tokens_test.go`: the created cell is the token's own creation time, a revoked token
      is marked and offers nothing, and revocation is confirmed

## 5. The form can set an expiry

- [x] 5.1 `internal/web/admin.go`: `tokenExpiry`, `tokenExpiryChoices`, `defaultTokenExpiry`,
      `tokenForm.expiresAt`
- [x] 5.2 `internal/web/templates/tokens.html`: the control, with the note saying what an expiry is for
- [x] 5.3 `internal/web/tokens_test.go`: the chosen expiry reaches the stored token, "never" renders as
      such, the proposed default is an expiry, and a key the list does not offer is refused

## 6. The issued value reads as a secret

- [x] 6.1 `internal/web/templates/tokens.html`: its own region, in a `.copyline`, with the generic copy
      button
- [x] 6.2 `internal/web/assets/app.css`: `.secret`, and why it is not a `.flash`
- [x] 6.3 `internal/web/forms_test.go`: the once-only guard reads the new element
- [x] 6.4 `internal/web/tokens_test.go`: the value is in the secret region, not in a flash or a `pre`

## 7. Documentation

- [x] 7.1 `docs/agents.md`: names and expiry under Tokens and scopes, and `token ls` / `token rm`, which
      were documented nowhere
- [x] 7.2 `skills/tix/SKILL.md`: the uniqueness rule, its exit code, and `--expires`
