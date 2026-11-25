# Tasks

## Groundwork

- [x] `core`: refresh outcome, retiring certificates, pinned settings, fingerprints, due-ness
- [x] Store: refresh state in the provider's settings JSON; `RecordSSOProviderRefresh` on both engines
- [x] `ssotest`: a fake IdP that rotates its key and retires the old certificate

## Service

- [x] `RefreshSAMLMetadata`: reread from the stored URL, merge certificates with a seven day overlap, keep
      pinned settings, record the outcome, audit and event when something changed
- [x] Create and edit record pinned settings and the outcome of a read on save
- [x] `WithSAMLMetadataRefresh` dates the next read

## Schedule

- [x] `internal/samlrefresh`: a worker visiting every tenant's due providers on a fake-clock ticker
- [x] `auth.saml_metadata_refresh` (12h, 0 off) wired through connect, serving and the server workers

## Surfaces

- [x] Registry entry, HTTP route, client, `just openapi`
- [x] `tix tenant sso refresh KEY`
- [x] Web: refresh status on the provider row and a Refresh metadata button
- [x] i18n: bg, de, fr

## Tests

- [x] Rollover with overlap then drop; failed, invalid and certificate-less reads keep the certificates;
      pinned settings; disabled and off providers skipped; interval and off; sso:admin required

## Docs

- [x] docs/sso.md, configuration.md, api.md, deployment.md, ROADMAP, README, skill
