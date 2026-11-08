# Web user edit sends only what changed

## Why

The Manage dialog on Admin > Users posted every field on each save, the forced password change included,
and the service refused a forced password change for somebody who also belongs to another tenant even when
the value was the one they already had. A tenant admin who renamed such a person, or changed their role,
got a 403 error page and the edit was lost. The refusal itself came back as a bare error screen.

## What Changes

- The web compares the Manage form with the user as they stand and sends only the fields that differ. A
  save that changes nothing calls nothing.
- The service treats a forced password change equal to the account's current value as no change, so it
  is not refused.
- An `account_in_other_tenants` refusal from the Manage dialog, for the forced password change or for
  clearing the second factor, redraws the dialog open with what was typed and a message naming the person
  beside the control that asked.

## Capabilities

### Modified Capabilities

- `web-ui`: the Manage dialog sends only changed fields and shows the cross-tenant refusal on the form.
- `auth`: an unchanged forced password change is not a change to a two-tenant person's account.

## Impact

`internal/web/admin.go`, `internal/web/totp.go`, `internal/web/createforms.go`,
`internal/web/templates/users.html`, `internal/i18n/bg_invites.go`, `internal/service/user.go`.
