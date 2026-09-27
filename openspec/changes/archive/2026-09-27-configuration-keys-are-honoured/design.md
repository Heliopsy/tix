# Design

## Where the fallback lives

`tix ssh` already had the shape: `sshOptions.fromConfig` copies the resolved value into any field whose flag
`cobra`'s `Flags().Changed` reports untyped. `tix serve` gains the same method for `server.listen` alone, and
`runServe` calls it once the configuration is resolved and before the options are assembled.

`Changed` is the discriminator rather than "is the flag value empty", because an address is a value a flag
declares a non-empty default for. Emptiness cannot separate "nobody typed it" from "somebody typed the
default", which is the whole trap: a flag whose default equals the configuration default looks correct in
every test that sets the flag.

## The credential

`globals.bearer` stays what it was, the flag layer: `--token`, then `TIX_TOKEN`. `globals.resolve` already
places that value in the flag layer of `server.token`, so the resolved configuration is the value with the
documented precedence applied. A new `globals.credential` reads the resolved value and falls back to `bearer`
for the callers that report a failure before configuration could load.

This is a one-way change: the resolved value can only be `bearer`'s, or something a lower layer supplied
while `bearer` had nothing.

## What the guard reads

A unit assertion over `fromConfig` is not enough, and the mutation testing proved it: with the call removed
from `runServe`, the per-key table still passed, because it called the helper rather than the command. The
call site is what gets forgotten, so each command that has one is covered end to end, by a request answered
on the configured address and a TCP connection accepted on it. The per-key table then covers the other
failure, a key left out of a `fromConfig` that is still being called.

A third test classifies every key `config.Keys()` returns as shadowed or not, with the consumer that reads
it named. It is bookkeeping rather than behaviour, and it says so: its job is to stop a new key shipping
without anybody deciding which of the two tables it belongs in.

## Why serve's SSH flags stay flag-only

`tix serve` documents that `--ssh-listen` alone turns the SSH listener on. Letting `ssh.listen` reach it
would mean an environment variable could open a network listener in a process whose operator asked for an
HTTP server, so the four flags keep reading no key and the documentation keeps saying so.
