# Tasks

## 1. Spec

- [ ] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Storage

- [ ] 2.1 Migration adding `require_claim` to `api_tokens` (both engines)
- [ ] 2.2 SQLite and PostgreSQL read and write the flag

## 3. Contract and service

- [ ] 3.1 `core.APIToken.RequireClaim` and `core.CreateTokenInput.RequireClaim`
- [ ] 3.2 Minting and rotation carry the flag
- [ ] 3.3 The gate on transition, route, edit, checklist, tags, project move and delete, judged per task in bulk

## 4. Surfaces

- [ ] 4.1 CLI `--require-claim` on `tix token create`, CLAIM column in the token table
- [ ] 4.2 HTTP field on token create and listings, regenerated OpenAPI
- [ ] 4.3 Web checkbox on the issue form and a mark on the listing, i18n keys in every catalog
- [ ] 4.4 TUI field on the token form

## 5. Docs and tests

- [ ] 5.1 Docs for tokens, claims and running AI agents; the agent skill
- [ ] 5.2 Store, service, CLI, HTTP, web and TUI tests
