# Design

## The flag is a challenge, not a session state

A password sign-in on a flagged account is answered with a challenge of kind `password_change`, the
same shape a second factor uses, and no session is minted. The caller sends the same email and
password again with `new_password`. That puts the rule in `Login`, the one place every password
sign-in passes through (browser, `POST /api/v1/auth/login`, `tix login`, local and remote), so no
surface can let a flagged account in by forgetting a check. A session-level "must change" state
would need every handler and every surface to honour it, and a forgotten one would be a way in.

The browser's "Choose a new password" page therefore has no session behind it: it asks for the
current password again beside the new one, and its only other way out is back to sign-in, which is
signing out. Nothing else is reachable because nothing is signed in.

The challenge carries no ticket. A ticket is signed with the sealing key, which a deployment without
a second factor need not configure; resending the current password proves the same thing and needs
no key.

`tix login` on a terminal asks for the new password twice without echo and finishes the sign-in;
without a terminal it stops and says where to change it. SSH sign-in takes keys only and is not a
password path. API tokens are not passwords and keep working.

The change and the flag clearing are one write with its audit entry `user.password_change`. When
nothing else stands in the way it is the same transaction that mints the session.

## Invites create the account up front

An invite creates the user, its actor and its membership when it is issued, with no password. The
person is assignable at once, shows on the Users screen, and accepting only sets a password. The
alternative, creating the account on acceptance, would leave invited people invisible to everything
that names actors until they act.

A user row is bound to one tenant through its actor, so an address that belongs to a user of
another tenant is refused as a conflict, as tenant creation refuses it. An address of this tenant's
user who holds no membership is granted the membership; if that account has a password, accepting
asks for it and leaves it as it is.

## Tokens

`tix_inv_` plus 32 random bytes from `crypto/rand`, stored as its SHA-256 hash in `user_invites` and
compared in constant time after the lookup. One unaccepted, unrevoked invite per user, enforced by a
partial unique index; re-issuing revokes the old row and writes a new one in one transaction. The
audit entry and the event carry the invite's id, email, role and expiry, never the token or the
hash. The token appears in a URL path, so the request log redacts `/invite/` and
`/api/v1/auth/invites/` paths the way it already redacts calendar feed paths, and the pages send
`Referrer-Policy: no-referrer`.

Every failure to resolve a token, whether malformed, unknown, expired, used or revoked, is the same
not found with the same message, so a link tells its holder nothing about other links. The token
carries 256 bits, so guessing is not a practical attack; acceptance goes through the same bounded
password hashing gate as sign-in.

## The link's address

Calendar feed addresses are built from the request's own scheme and host, honouring a trusted
proxy's forwarded scheme. Invites use the same: the HTTP API and the browser fill in the absolute
`url`; a command run against a database directly prints the path and says to prefix it with the
server's address.

## Email

tix has no mail transport and this change does not add one. The administrator copies the link.
