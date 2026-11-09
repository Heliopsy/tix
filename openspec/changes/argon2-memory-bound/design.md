# Design

## Parameters

OWASP lists argon2id configurations of equal strength: m=46 MiB t=1, m=19 MiB t=2, m=12 MiB t=3, m=9 MiB
t=4 and m=7 MiB t=5, all p=1. Memory is what a sign-in holds, and passes are what it spends in time, so for a
server sized in memory the useful lever is fewer MiB and more passes. Measured on the development host, one
derivation took 120 ms at m=64 t=3 p=4, 58 ms at m=46 t=1, 100 ms at m=46 t=2 and 37 ms at m=19 t=2.

m=46 MiB t=2 p=1 keeps the latency of the old setting on one core, holds 28% less memory, and is twice the
work of OWASP's 46 MiB option. It is about half the memory-time product of the old setting. Keeping 64 MiB
with only a bound was the other option; on a 256 MiB pod it fits one sign-in at a time with little room for
anything else. Operators who want the old cost back set it.

## Sizing the bound

`debug.SetMemoryLimit(-1)` reads the soft limit in force. It is read when the first hash runs, not at
package load, so it sees the limit `tix serve` derives from the container after the service is built.
A quarter of it goes to hashing: 1 slot on 256 MiB, 2 on 512 MiB, 4 on 1 GiB at 85%. More slots than cores
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

`NeedsRehash` compares the stored parameters with the policy for equality. A sign-in that succeeds rehashes
at the policy and stores it with the session, as it already did for weaker hashes. An account still on the
old hash takes longer to check than an unknown address until then; that window closes per account at its
next sign-in.
