# Reread SAML IdP metadata on a schedule

## Why

Entra ID, Okta and Google Workspace rotate the certificate they sign SAML responses with. tix reads an IdP's
metadata only when an admin saves the provider, so after a rotation sign-in breaks until somebody saves it
again. The provider already stores the metadata URL; nothing reads it afterwards.

## What Changes

- `tix serve` reads the metadata of every enabled SAML provider that has a metadata URL every
  `auth.saml_metadata_refresh` (default `12h`, `0` off), across every tenant, from the URL stored on the
  provider. A failed read is retried after an hour.
- Certificates are merged: one the metadata adds is trusted at once, one it stops naming keeps verifying for
  seven days and is then dropped. A read that fails, or metadata that is invalid or names no signing
  certificate, changes nothing.
- The IdP settings an admin set explicitly (entity id, sign-in URL, certificates) are recorded as pinned and a
  refresh leaves them alone, as a save with metadata already does.
- Each provider records the last read's time, outcome and error, the certificates retiring and when, and the
  pinned settings. `tix tenant sso show`, `GET /api/v1/sso/providers/KEY` and the Single sign-on screen show
  them, with the next scheduled read.
- `tix tenant sso refresh KEY`, `POST /api/v1/sso/providers/KEY/refresh` and a Refresh metadata button read it
  now; each needs `sso:admin`.
- A refresh that changes the certificates or sign-in URL writes an `sso.provider.refresh` audit entry and an
  `sso.provider.updated` event. A failed one is logged and recorded on the provider.
- Docs: docs/sso.md certificate rollover, configuration.md, api.md, deployment.md, ROADMAP, README, skill.

Not in this change: following a change of the IdP's entity id (it selects the IdP within the metadata, so it
stays an edit), a configurable overlap window, and refreshing OIDC discovery documents, which are already read
on demand.
