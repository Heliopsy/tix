# Design

## The two properties, separated

Two statements were conflated in the old ordering:

1. An observer cannot tell from a refusal whether a key is enrolled.
2. An observer cannot fill the listener without being enrolled.

The old code obtained (1) by deciding capacity before any lookup, and lost (2) because that decision spent
capacity. They separate cleanly once the decision and the spend are separate operations: `reserve` decides,
at the instant the old code decided, and `acquire` spends, once there is an actor to spend it for.

A reservation counts against the per-key cap and never against the listener's. That asymmetry is the whole
fix. Any listener-wide hold taken before the lookup is, by construction, a hold a stranger can take, so a
pre-lookup pool sized against the listener would only make the same denial cheaper to mount.

## What an observer can distinguish

The message an unenrolled key receives is decided at the same point in the exchange as before, from the same
counters, so the sets of answers are unchanged:

- Listener full: `this demo is serving its limit of N sessions`, for every key, before any lookup. Both keys
  have done identical work at that point, so the refusal is neither faster nor slower for one of them.
- Listener not full, key unenrolled: `this key is not enrolled here`, after the lookup.
- Listener not full, key enrolled: the interface.

An enrolled key can now be refused for capacity at either step, but with one message, which is the message a
full listener already gave. The only new outcome is the race in which a key passes `reserve`, resolves, and
finds the last slot gone; it is then told what every other key at that moment is told.

Timing carries what it carried before. An enrolled lookup does more work than an unenrolled one, and that
difference was already observable on a listener with room; the fix neither widens it nor adds a fast path
that refuses an unenrolled key before the lookup. Critically, the fullness of the listener is not itself
timeable against enrolment, because when the listener is full neither key reaches the lookup at all.

## Alternatives weighed

- **Refuse in `PublicKeyHandler`.** SSH auth already expects a yes or no, and no slot would be taken. It
  moves the refusal into the handshake, where it becomes `Permission denied (publickey)` with no message, and
  on a full listener an enrolled key would still authenticate and then be refused for capacity while an
  unenrolled one was denied at the handshake: a clean enrolment oracle, and the loss of the refusal text the
  boundary test pins.
- **Move the slot after the lookup and check nothing before it.** Simple, and it closes the capacity leak,
  but the refusals then diverge: a full listener tells a stranger it is not enrolled and a member that the
  listener is full. That is the oracle the old comment existed to prevent.
- **A separate pre-lookup pool sized against the listener.** Bounds stranger work, but the pool is itself
  listener-wide capacity a stranger can fill, and filling it refuses enrolled keys. The denial moves rather
  than going away.
- **Reserve, then convert (chosen).** Keeps the decision point, spends nothing until there is an identity,
  and bounds one fingerprint's concurrent resolutions by the per-key cap.
