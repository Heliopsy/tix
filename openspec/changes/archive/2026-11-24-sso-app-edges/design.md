# Design

## Handoff after enrolment

The recovery codes page carries a handoff ticket rather than a code. It reuses the second factor ticket's
encoding (user, tenant, surface, expiry, method, app) but is signed under its own HMAC label, so a handoff
does not pass as a second factor ticket or the reverse. It reaches the service as `LoginInput.AppHandoff`,
which is not part of the API's login request. Open the app posts it to `POST /login/sso/app`; the service
checks the person still holds a membership and stores the code then. The exchange still checks everything
else, so a handoff alone gets nobody a session: the code it mints goes to the app's redirect and needs the
app's PKCE verifier.

Fifteen minutes is long enough to keep ten codes and short enough that a forgotten tab is useless. An
expired handoff goes back to the app with `expired`; by then the person is enrolled, so starting again asks
for a code and hands one over at once.

## Browser only

The service tells the browser interface apart by the request source the web handler sets. An app's flow,
ticket or handoff anywhere else is refused as invalid, naming the field, rather than sent to the app's
redirect, since the API caller is not the browser.

## Second factor refusals

A wrong or rate limited code still redraws the form. Anything else refused at that step, an expired ticket
included, goes back to the app. The check compares the refusal with the canonical code errors by kind and
message, so the code path itself is untouched.
