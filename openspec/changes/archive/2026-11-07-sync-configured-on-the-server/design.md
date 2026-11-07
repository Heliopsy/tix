# Design

## Sealing

The repository already seals one kind of recoverable secret, TOTP seeds, with AES-GCM under `auth.totp_key`
(`internal/auth/seal.go`). Sync credentials reuse that sealer rather than adding a second key: an operator who
has set the key for second factors gets credential storage for free, and one who has not is told which key to
set. The additional data is `sync-source`, tenant, system and name, so a ciphertext copied onto another source
row, or into another tenant, does not open.

Webhook secrets are stored in the clear and only hidden from responses. That was not good enough here: a sync
credential is usually a personal API key of a person with wide access to the other tracker.

Without a key, storing a credential is refused rather than stored in the clear. The environment variables keep
working, so a deployment with no key loses nothing it had.

## Precedence

One rule for everything: what is stored on the source wins, and the server's environment fills what the
source leaves empty. That was already the rule for settings. Credentials apply it as a set, so a stored token
is never combined with a password from the environment. The mapping has three places: the stored document,
then `mapping_path`, then `TIX_SYNC_<SOURCE>_MAPPING`; `resolved.mapping.from` says which one a run will read.

## Readiness

`resolved` is computed on the server, which is where runs execute, so a mapping path is checked for
readability on the machine that will read it. Credentials are required for jira and openproject. A run is
refused with every missing item listed, instead of the first one an adapter trips over, which used to be an
HTTP 401 from the external system.

## Keeping what an update leaves out

Settings are replaced whole on update, as before. The mapping document and credentials are kept unless sent
or cleared: they are expensive to retype, credentials cannot be read back to resend, and a form that does not
show them must not erase them.
