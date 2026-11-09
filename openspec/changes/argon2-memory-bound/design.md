# Design

## Parameters: the cost was kept

OWASP lists argon2id configurations of equal strength: m=46 MiB t=1, m=19 MiB t=2, m=12 MiB t=3, m=9 MiB
t=4 and m=7 MiB t=5, all p=1. The default, m=64 MiB t=3 p=4, is about four times the memory-time product of
the first. Timed on the development host, one derivation took 120 ms at the default, 100 ms at m=46 t=2 and
58 ms at m=46 t=1.

A cheaper default (m=46 MiB t=2 p=1) was considered and rejected. The kills came from how many derivations
ran at once, which the bound fixes; a cheaper hash would have halved an attacker's per-guess cost for every
account to save memory the bound already saves. The cost stays, and memory is sized as 64 MiB times the
number of slots. Operators on very small servers can lower it for new hashes through the `auth.argon2_*` keys,
down to an OWASP floor.

## Sizing the bound

`debug.SetMemoryLimit(-1)` reads the soft limit in force. It is read when the first hash runs, not at
package load, so it sees the limit `tix serve` derives from the container after the service is built.
A quarter of it goes to hashing: at 64 MiB per hash and 85% of the container, 1 slot on 256 MiB and 512 MiB,
3 on 1 GiB, 6 on 2 GiB. More slots than cores
add memory without throughput at p=1, so the core count caps it. Without a limit there is nothing to size
from, and 2 keeps two people from queueing on each other without promising memory nobody measured.

## The busy refusal

The error kinds are the frozen contract in `internal/core`, and none of them is a 503 or a 429. Adding one
changes the HTTP mapping, the exit-code table and every client. The refusal reuses `precondition_failed`
(HTTP 422, its own exit code) with a `reason` detail and a message that says to retry, which the web form
shows beside the sign-in fields. It is the same for every account, so it reveals nothing about whether one
exists, and the dummy check goes through the same bound.

## Returning memory after a burst

Not added. With a soft limit the scavenger returns freed pages as the heap nears it, so memory held after a
burst does not take the process past the limit, and the smoke run showed RSS dropping from 255 to 148 MiB
within a second of the load stopping without help. `debug.FreeOSMemory` would lower the steady figure, not
the peak the kernel kills on, and costs a forced collection. The argon2 package does not take a caller's
buffer, so reusing the work area would mean forking it.

## Migration

`NeedsRehash` stays upgrade-only. A hash weaker than the setting is rehashed at a successful sign-in and
stored with the session, as before. A stronger one is kept: lowering the setting to fit a small server changes
new hashes and never weakens one already stored. An account on a costlier hash takes longer to check than an
unknown address; that is the same window any change of setting has always opened.
