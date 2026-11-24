# App sign-in edges

## Why

A review of native app sign-in (`sso-app`) found six edges:

- A first enrolment minted the app's two minute code before the recovery codes page, which waits for the
  reader. Anyone who took longer than two minutes to keep the codes handed the app a dead code.
- Under an enforced second factor policy an unenrolled person got an app session the policy gate refuses
  everywhere, since the web skips enrolment inside the sign-in and the app has no screen to enrol on.
- Some refusals after the app's redirect was validated (a begin failing past that check, a second factor
  step that expired or failed on the server) reached the error screen instead of the app.
- `auth.sso_app_redirects` accepted `javascript:`, `data:`, `vbscript:` and `file:` entries, which then
  reach a link and a meta refresh.
- The JSON API took an app's flow or second factor ticket and wrote a code nothing would ever carry to the app.
- Expired codes were pruned only on the next sign-in in the tenant.

## What changes

- An enrolment that completes an app sign-in shows the recovery codes with Open the app as a CSRF protected
  form carrying a signed handoff (15 minutes). Posting it mints the code and answers 302 to the app.
- An app sign-in under an enforced policy enrols inside the sign-in, as the command line does.
- Begin failures past the app check, and second factor steps refused for anything but the code itself, go
  back to the app with a reason.
- Those four schemes are refused in `auth.sso_app_redirects`, ignoring case.
- An app's flow, ticket or handoff is refused outside the browser interface.
- Taking a code also prunes the tenant's expired codes.
