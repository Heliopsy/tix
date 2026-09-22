# Design

## One registry, two feeders

`internal/server` owns a `connections.Registry`. The WebSocket hub registers on upgrade and unregisters on
close, which it already does for its own counters. The SSH listener registers when a session opens and
unregisters when it ends, which is the piece it does not have today: its gate counts sessions but keeps no
handle on them.

Giving the listener handles is worth doing on its own. The shutdown path already wants them. `Drain` can stop
accepting and wait, but a session still open at the deadline is closed silently, because there is nothing to
write to. With a registry there is.

A registry entry holds:

    ID          a fresh identifier, valid for the life of the connection
    Surface     "events" for the WebSocket, "ssh" for a terminal session
    TenantID    the tenant the connection is scoped to
    ActorID     the actor it speaks for
    Remote      the address it came from
    Since       when it opened
    Fingerprint the key, for an SSH connection only

Nothing is written to the database. These are facts about a process, true only while it runs, and persisting
them would mean reconciling rows against reality after every crash.

## What an administrator may see and do

Listing and ending are tenant-scoped through the same path as everything else, under `ActionTenantAdmin`. A
caller sees the connections of their own tenant and no others, and may end those and no others. The isolation
suite covers this like any other surface, because an observability endpoint that leaks across tenants is
still a leak, and it is the kind that gets built in a hurry and reviewed lightly.

The counts an administrator sees are their tenant's. The process-wide total is reported alongside, without
any breakdown, because "this server is holding 240 connections" is capacity information an operator needs and
says nothing about who anybody else is.

## Ending a connection is a mutation with no rows

Every mutation here writes domain rows, an audit entry and an event in one transaction. Ending a connection
writes no domain rows, because there are none: the thing being changed is a socket.

It still writes the audit entry and the event. Cutting somebody off is a privileged act against a named actor
and it must be answerable afterwards, which matters more here than for writes that leave a trace in the data
anyway. The audit entry names the actor ended, the surface and the caller.

The connection is closed after the audit entry commits. A record of a cut that did not happen is a smaller
problem than a cut with no record.

## Per process, and what it would take not to be

A connection is held by one process. A server can end only what it holds, so with several servers against one
database each reports and ends its own. The response says which server answered, so a partial view is visible
as partial rather than read as the whole.

The extension is available and deliberately not taken here: the outbox already reaches every server, and a
`connection.terminate` event carrying an identifier would let whichever process holds it act. That turns a
local call into a broadcast with no acknowledgement, so the caller learns that the request was published
rather than that the connection ended. Worth doing when somebody runs more than one server; misleading to
build before that, because the weaker guarantee would be invisible in the single-server case it was tested in.

## Why not reuse the login session

`core.Session` is an authenticated period backed by a row, with an expiry and a revocation. A live WebSocket
and an SSH terminal are neither rows nor revocable in that sense: they end when the socket ends. One actor
may hold one login session and six connections. Naming both "session" in one service interface would make
`ListSessions` ambiguous at exactly the moment somebody is trying to cut off an intruder.
