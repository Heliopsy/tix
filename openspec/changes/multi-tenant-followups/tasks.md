# Tasks

Carried over from `one-account-many-tenants` (tasks 2.6 and 6.1 there). Sizes: S under a day, M one to
three days.

## 1. Accounts left with no actor (S)

- [ ] 1.1 Tenant deletion deletes accounts whose every actor was in the deleted tenant, in the same
  transaction, keeping global admins and accounts still a member elsewhere; account log row per deletion
- [ ] 1.2 `tix doctor` lists accounts with no actor (count, ID, email), deleting nothing
- [ ] 1.3 Tests on both engines, the guard watched failing; `docs/tenancy.md`

## 2. Cross-host switch (M)

- [ ] 2.1 Switch ticket migration (`just migration switch_tickets`), unscoped allow-list entry and
  isolation guard, both engines
- [ ] 2.2 Ticket mint in the switch for a tenant at its own domain; `POST /auth/switch/redeem` with the
  confirmation page; the redeem route redacted from the request log
- [ ] 2.3 The switcher offers Switch for a tenant at its own domain; Bulgarian strings; screenshots
- [ ] 2.4 Registry, `just openapi`, `docs/api.md`, `docs/web-ui.md`, the skill
