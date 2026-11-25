# Design

## Renewal rides on the touch

The verifier already writes `last_seen_at` when it is a minute old or more. The renewed expiry is written in the
same update, so a busy session costs one write a minute, as before. Within the minute the expiry lags real use
by at most a minute, which is noise against a window of hours.

## Window from the tenant, cap from the server

The window is the effective `ttl` (or `remember_ttl`) read at lookup, so a tenant that shortened its lifetime
renews by its own, shorter, window. The cap is server-wide only, to keep this change free of tenant columns and
a migration; a tenant override can follow if somebody asks. The cap is `created_at + max(limit, window)`, so a
limit below the window renews nothing, and the expiry never moves earlier.

## Cap computed, not stored

Computing the cap from `created_at` avoids a migration and lets an operator lower it for live sessions' next
renewal. A tenant switch stores the old session's `created_at` on the new one, which is also what the sessions
list shows as when the sign-in happened.

## Cookie

Only a remembered session has a persistent cookie, so only it is sent again on renewal, from the API middleware
both surfaces share. `Max-Age` is the new expiry less the time the server checked the request, by the server's
own clock.

## Idle timeout unchanged

`session.idle_timeout` stays a separate, tighter cut-off a tenant can set, and still applies to remembered
sessions.
