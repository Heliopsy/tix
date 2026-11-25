# Design

## Where the schedule runs

The refresher is one more background worker of `tix serve`, beside the lease sweeper, the retention pruner and
the webhook dispatcher, supervised and restarted the same way. It lists every live tenant from the store, then
for each one calls `ListSSOProviders` and `RefreshSAMLMetadata` as that tenant's system actor on the
`saml-refresh` job, so the read goes through the service's authorisation, egress policy and transaction like
any other change. It looks for due providers at start and then every `min(interval, 15m)`.

## Due-ness lives on the provider

Whether a provider is due is decided from its own `metadata_refresh.at` (and `result`, for the one hour retry
after a failure), not from when the process started. A restart does not reread every provider, and with
several replicas a provider one process just read is not due on another. Two processes can still read the same
provider in the same instant; the write rereads the row in its transaction and merges onto what is stored, so
the second write is a no-op rather than a lost update. No lock is taken, which keeps the worker identical on
SQLite and PostgreSQL. An operator who wants exactly one reader sets the interval to `0` on the other processes,
as with the other workers.

## Merging certificates

```text
published = certificates in the metadata
kept      = published, then each stored certificate not published whose retiring mark has not expired
retiring  = a mark (fingerprint, since, until = since + 7 days) for each stored certificate the metadata
            dropped, set on the first refresh that finds it gone
```

A mark is keyed by the certificate's SHA-256, so it survives a reorder of the metadata. A certificate that
reappears loses its mark. Seven days covers the slowest published rollover schedules (Entra publishes the next
certificate well ahead and keeps the old one listed after the switch) while bounding how long a key an IdP
retired is still trusted. It is a constant rather than a setting: nobody has asked to tune it, and a shorter
value is the kind of change that breaks sign-in quietly.

Metadata that does not parse, names no signing certificate, or would leave the provider invalid is a failed
read: the provider keeps every setting, the outcome records the error, and sign-in carries on.

## Pinned settings

Today a save with metadata fills only what the admin did not give. The row did not record which values those
were, so a later refresh could not honour it. `idp_pinned` records them:

- On create, a given entity id, sign-in URL or certificate list is pinned.
- On edit, a value given that differs from the stored one, or one already pinned, is pinned; a value given
  empty is handed back to the metadata; a value not given keeps its pin unless metadata is read on the same
  save, which fills it.

The web edit dialog sends every field, so a value equal to the stored one does not pin it; otherwise saving the
dialog once would freeze the certificates against every later refresh.

## Recording the outcome

`metadata_refresh` (`at`, `result`, `error`, `added`, `removed`), `idp_certificates_retiring` and `idp_pinned`
live in the provider's settings JSON, so no migration is needed. A read that changes nothing writes only that
JSON through `RecordSSOProviderRefresh` and leaves `updated_at` to admin changes; a read that changes the
certificates or sign-in URL goes through `UpdateSSOProvider` with an audit entry and event in the same
transaction. `next` is computed when a provider is answered, from the configured interval, so it is absent when
the schedule is off or the provider is disabled.

## Surfaces

`RefreshSAMLMetadata(ref)` is one service method with a registry entry: CLI `tix tenant sso refresh`, HTTP
`POST /sso/providers/{id}/refresh`, web `POST /admin/sso/refresh`. The terminal interface is exempt for the
reason the other provider operations are. A manual refresh returns the read's error after recording it, so the
CLI exits 7 for an unreachable IdP and 2 for invalid metadata.
