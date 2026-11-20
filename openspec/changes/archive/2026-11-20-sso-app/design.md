# Design

## The browser does the sign-in, the app gets a code

The app cannot receive a provider's redirect, so the provider keeps redirecting to the server's own callback or
ACS. The app's values ride in the sealed flow cookie (`auth.SSOFlow.App`), and in the signed second factor
ticket when a code is asked for (`auth.Ticket.App`), so nothing new is trusted from a URL after the start.

One choke point turns a session into a code: `mintSessionIf` hands off when the context carries an app sign-in.
Every way a provider sign-in ends (no factor, a trusted provider factor, a TOTP code, a recovery code, an
enrolment) passes there, so none can mint a browser session for an app flow by accident, and whatever the step
records (the TOTP attempt, the enrolment, the recovery codes) is written in the same transaction as the code.

The session itself is minted at the exchange, from the app's own request, so its user agent and its cookie are
the app's. It is always remembered and carries `sso:KEY` and whether a second factor was passed.

## Second factor

The browser asks for the factor, as on the web. The ticket's surface is web, because the web's form completes
it; the requirement is judged on web and on cli, since the app's session is the one its password sign-in gets
on the cli surface. Judging on web alone would let an app sign-in walk past a cli-only requirement.

## Single use

`TakeSSOHandoff` selects the row by hash and deletes it by id in one transaction, and only the take whose delete
removed a row wins. Under PostgreSQL's read committed the second delete waits for the first and then finds
nothing; SQLite serialises writers. A wrong verifier spends the code: the take commits before the compare.

## Refusals

Before `app_redirect` is checked, a refusal is the web error screen. After, it is the app's redirect with a
reason: `access_denied`, `not_linked`, `expired`, `provider_error`, `server_error`. The service attaches the
target as the error detail `app_redirect`, and the web layer sends the browser there. An expired flow still
opens (`PeekSSOFlow`), and its seal vouches for the app's address, so it can say `expired`. A refused code at
the second factor stays on the form, as on the web.

## Custom schemes after a POST

A browser may refuse a redirect to an app's scheme that no gesture started, which is what an IdP's
auto-submitted SAML form produces. After a POST the server answers with a page that navigates itself (meta
refresh) and links the address; after the provider's GET redirect it answers `302`. The page also shows the
recovery codes of an enrolment, without navigating away, before the reader goes on.

## Rate limiting

`POST /auth/login` has no request rate limiter of its own (the hash concurrency cap and the TOTP lockout are
what bound it); the exchange runs under the same public route middleware and refusal log. A 256 bit,
two-minute, single-use code bound to a verifier leaves nothing to guess.
