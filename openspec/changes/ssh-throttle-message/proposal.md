# Say why a throttled ssh connection is refused

## Why

The ssh listener's connection rate limit (a burst of 5 per source address, refilled at 60 an hour) refuses a
connection inside the public key callback. The client then prints `Permission denied (publickey)`, the same line
an unknown key gets, so a few quick logins in a row on prod looked like a broken key. Separately, a global admin
whose key was "not enrolled here" in a tenant with no members had nothing in the docs telling them that the
global admin flag does not make them a member.

## What Changes

- A throttled connection is let through authentication and refused at the session with
  `tix: too many connections from this address; try again in N seconds` and exit status 1. Nothing is looked up
  for it and no session slot is taken.
- The rate limit is charged once per connection, however often the public key callback runs for it.
- docs/deployment.md describes the limit, the wait and the message. docs/tenancy.md says a global admin is not a
  member of any tenant, how to join one and enrol a key there, and what a tenant with no members looks like.
