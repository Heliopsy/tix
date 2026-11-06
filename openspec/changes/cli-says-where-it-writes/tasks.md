# Tasks

## 1. Token

- [x] 1.1 `globals.credential` resolves the configuration itself
- [x] 1.2 A server refusing a tokenless command names `tix ctx add ... --token`
- [x] 1.3 `cmd/whereto_test.go`: every way of giving a token, across commands, asserted at the server

## 2. Target

- [x] 2.1 `target` and `cli.target_notice` keys, documented
- [x] 2.2 `connect.NeedsOptIn`, `Target.Implicit`
- [x] 2.3 The first-run guard in `dialWith` and `tix demo seed`, with the terminal question
- [x] 2.4 The local database notice
- [x] 2.5 `tix init`, `tix whoami`, the identity lines on `tix status`, the `TOKEN` column on `tix ctx ls`
- [x] 2.6 Tests for the refusal, the question, every opt-in and the notice, each watched failing

## 3. Docs and skill

- [x] 3.1 `docs/configuration.md`, `docs/agents.md`, `README.md`, `docs/scaling.md`, `docs/commands.md`
- [x] 3.2 `skills/tix/SKILL.md` and `reference/setup.md`: check `tix whoami` first; version 19
