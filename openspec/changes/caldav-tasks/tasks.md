# Tasks

Phase 1 ships alone. Phase 2 starts only once phase 1 is released and has been tried against the four clients.

## Phase 1: read-only

### 1. Credentials

- [ ] 1.1 `internal/core/identity.go`: `ScopeCaldav` (`caldav`), named-only; `CreateTokenInput.Validate` refuses it
      with anything else, a project or a claim requirement; test both
- [ ] 1.2 `internal/auth`: refusal reasons `caldav_only`, `dav_password_required`, `dav_user_mismatch`; the
      verifier refuses a `caldav` token off `/dav/` (as `scim_only`); table test per reason and route
- [ ] 1.3 `internal/auth`: `BasicDAVAuthenticator` that accepts only a `caldav` token as the password, checks the
      username against the owner's email or handle, and never calls the password hasher (assert with the hash
      gate's test hook)
- [ ] 1.4 `internal/httpapi/middleware.go`: `/dav/` uses the Basic authenticator alone, ignores cookies and bearer,
      answers `401` with the Basic challenge; `credentialHash` reads a Basic password under `/dav/` only; test a
      cookie, a bearer, an account password and an ordinary token are each refused
- [ ] 1.5 `docs/deployment.md`: the three refusal reasons in the refusal table

### 2. App passwords

- [ ] 2.1 `just migration dav_passwords`: `dav_passwords` (tenant, token, actor, name, collections, read_only),
      unique live name per actor, cascades; store methods on both engines with a test each
- [ ] 2.2 `internal/core`: `DAVInfo`, `DAVPassword`, `IssuedDAVPassword`, `CreateDAVPasswordInput`
- [ ] 2.3 `internal/authz`: action `dav.password` (task read, not confinable); authz test row
- [ ] 2.4 `internal/service/davpassword.go`: `GetDAVInfo`, `ListDAVPasswords`, `CreateDAVPassword`,
      `RevokeDAVPassword`; person callers only; token row name `dav-HANDLE-NAME`; `token.*` audit and events;
      `CreateToken` refuses `caldav`; tests for a member without token admin, an SSO-only person, an agent, a
      duplicate name, two people with one name, revocation
- [ ] 2.5 `internal/service`: the DAV reader built from the owner's current role and the ceiling, shared with the
      feed's `feedReader`; test demotion, removal, disabled account, read-only

### 3. Rendering

- [ ] 3.1 `internal/ical/todo.go`: `RenderTodo` with the mapping table in design.md; golden tests for each state
      category, a bare and a timed due date, a snooze before and after the due date, every priority, tags,
      a parent, a long multi-byte title (fold and escape round-trip)
- [ ] 3.2 `internal/ical`: the ETag function, computed without rendering; test that it changes with the version,
      the project key, the workflow's category map and the origin, and with nothing else

### 4. Collections and sync

- [ ] 4.1 `internal/core`: `TaskCollections` interface, `DAVCollection`, `DAVMember`, `DAVSnapshot`
- [ ] 4.2 `internal/service/davcollections.go`: membership (trash, cancelled, completed window, `max_objects` order)
      over keyset pages through the tenant-scoped builder; tests for each exclusion and the bound
- [ ] 4.3 `just migration dav_sync_states` and store methods on both engines: write only on change, share identical
      sets, prune to three per password and collection plus a day; tests on both engines
- [ ] 4.4 `internal/service`: snapshot diff, the event-sequence short cut, an unknown token, the window ageing a
      task out, a workflow edit changing a category, a `limit`; all with `clock.FakeClock`
- [ ] 4.5 PostgreSQL test: two writes committing out of sequence order are both reported by the next sync

### 5. Protocol

- [x] 5.1 `internal/caldav/xml.go`: the bounded decoder (64 KiB, depth 32, 2000 elements, no DOCTYPE or entities)
      and the multistatus encoder; fuzz test for the decoder
- [x] 5.2 `internal/caldav/paths.go`: path parsing, tenant key and actor checks, every mismatch `404`; test with two
      tenants sharing project keys, handles and one account
- [x] 5.3 `internal/caldav/propfind.go`: root, principal, home, collection and object properties, depth 0 and 1,
      `Depth: infinity` refused, unknown properties in a `404` propstat, `current-user-privilege-set` read-only
- [x] 5.4 `internal/caldav/report.go`: `calendar-query` filter evaluation, `calendar-multiget` with its 1000 bound,
      `sync-collection` with `valid-sync-token` and `507`; table tests per filter kind
- [x] 5.5 `internal/caldav/handler.go`: `OPTIONS`, `GET`/`HEAD` with `If-None-Match`, `403` for every write method,
      `207` per-property `403` for `PROPPATCH`
- [ ] 5.6 `internal/httpapi/router.go`: `Config.CalDAV` mounted under `/dav/`, `/.well-known/caldav` redirect,
      both off when `caldav.enabled` is false; `internal/server/assemble.go` wiring; route test
- [x] 5.7 Client fixtures in `internal/caldav/testdata/`: request sequences captured from Apple Reminders (iOS and
      macOS), Thunderbird, DAVx5 and Evolution, replayed from discovery through a full and an incremental sync

### 6. Configuration and surfaces

- [ ] 6.1 `internal/config`: `caldav.enabled`, `caldav.completed_window`, `caldav.max_objects` with range checks;
      `docs/configuration.md`
- [ ] 6.2 `internal/wire`, `internal/httpapi`, `internal/client`: `GET /api/v1/dav`, `GET|POST
      /api/v1/dav/passwords`, `DELETE /api/v1/dav/passwords/{ID}`; proxy-aware `url`
- [ ] 6.3 `internal/capability/registry.go`: `dav.info`, `dav.password.list`, `dav.password.create`,
      `dav.password.revoke` with CLI, HTTP and web bindings and a TUI exemption; `just openapi`
- [ ] 6.4 `cmd/dav.go`: `tix dav show`, `tix dav password list|create|revoke`, generated command docs
- [ ] 6.5 `internal/web`: Tasks in other apps section on Settings, create dialog, one-time password screen with Copy,
      revoke confirm dialog, CSS part, i18n with Bulgarian, German and French; screenshot of Settings only
- [ ] 6.6 `internal/mcp/overlay.yaml`: hide `dav.password.create` from models if review agrees a model has no use
      for it, with the reason

### 7. Documentation

- [ ] 7.1 `docs/caldav.md` with `#app-passwords`, `#apple-reminders`, `#davx5-and-tasksorg`, `#thunderbird`,
      `#evolution`, `#troubleshooting`, and what each field maps to; link it from `docs/README.md`
- [ ] 7.2 `docs/calendar.md`: point "Why events and not to-dos" at CalDAV
- [ ] 7.3 `docs/api.md`, `docs/features.md`, `docs/web-ui.md`, README feature line, `skills/tix`
- [ ] 7.4 ROADMAP: move the entry from v2 to Shipped since v1 with what came out different

## Phase 2: write-back

### 8. Reading iCalendar

- [ ] 8.1 `internal/ical/read.go`: unfold, unescape, parameters, one `VCALENDAR` with one `VTODO`, 256 KiB bound;
      fuzz test; round-trip test against `RenderTodo`
- [ ] 8.2 `internal/ical`: date reading (date, UTC, `TZID` with tzdata, `VTIMEZONE` fallback, floating in the
      person's zone); table test per client's date forms

### 9. Writes

- [ ] 9.1 `internal/service`: the phase 2 ceiling and `write` in `GetDAVInfo`; privileges in `PROPFIND`
- [ ] 9.2 `internal/service/davwrite.go`: `PutDAVObject` as a diff against the rendered task, through `UpdateTask`
      with the ETag's version; absent-means-unchanged and absent `DUE` clears; tests per property
- [ ] 9.3 Status through the multi-hop move; tests for a two-hop done, a guarded edge, an ambiguous route,
      `NEEDS-ACTION` from done
- [ ] 9.4 `internal/caldav`: `PUT` with `If-Match` (`428`, `412`), no `ETag` on success; test a browser edit racing a
      phone edit
- [ ] 9.5 `just migration dav_objects`; create through `CreateTask` in a project or the default project, client UID
      and name kept, `no-uid-conflict`; Evolution's create without `If-None-Match`
- [ ] 9.6 `DELETE` as cancel on open tasks, `204` no-op on done, `403` with no cancelled state; never trashed
- [ ] 9.7 Kept properties per person, 8 KiB bound, recurrence dropped, included in the ETag; test another person
      never sees them
- [ ] 9.8 Replay the client fixtures with an edit, a completion, a create and a delete from each client

### 10. Documentation

- [ ] 10.1 `docs/caldav.md`: writing, what `DELETE` does, the completed-reminders caveat, what is kept and dropped
- [ ] 10.2 `docs/api.md`, `skills/tix`, ROADMAP
