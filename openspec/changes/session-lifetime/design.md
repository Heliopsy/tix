# Design

## Modelled on the artifact limits

Server keys with a `tix serve` flag that joins the flag layer only when typed, tenant columns where empty
inherits, and a tighten-only check in `UpdateTenant` that names the server's value when it refuses. The
effective policy is computed by `core.EffectiveSessionPolicy`, which clamps a tenant value the server has since
tightened past rather than trusting it.

## Keep me signed in stored as one column

`tenants.session_remember_ttl` holds `''` (inherit), `off`, or a duration. Three states in one text column avoid
a second boolean that could contradict the duration. The API input takes the same text.

## Remember travels on the context

`LoginInput.Remember` is put on the context by `Login` and read where the session is minted, because the
password, second factor, recovery code and password change paths all reach `mintSessionIf` with different
arguments. A tenant switch carries the presented session's remembered flag to the new one. A remember lifetime
no longer than the ordinary one is not offered, since ticking it would grant nothing.

## Idle timeout checked at lookup

The credential lookup that resolves a session already runs inside the request's tenant, so it reads the tenant
and the session's last use there and the verifier refuses an idle session. This is why a new idle timeout
applies to live sessions at once. A touch is a separate short write, issued only when the stored time is a
minute old or more, so a busy session costs one write a minute. `NULL` last use reads as the sign-in time.

The idle timeout applies to remembered sessions too. Exempting them would let one tick of a box defeat a policy
an administrator set for the whole tenant; an operator who wants Keep me signed in to survive a weekend sets the
idle timeout longer or leaves it off.

## Cookie lifetime

A remembered session's cookie carries `Expires` and `Max-Age` equal to the lifetime it was minted with
(`ExpiresAt - CreatedAt`), so it agrees with the server's expiry regardless of the clock of the process setting
it. Every other session gets a browser-session cookie; the server still refuses it after its expiry.

## Minimum of a minute

Each setting is at least a minute when set, because use is recorded at that granularity.
