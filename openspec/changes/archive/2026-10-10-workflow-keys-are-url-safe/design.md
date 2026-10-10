# Design

## Where the check lives

In `core`, on `WorkflowInput.Validate`, beside the project and tenant key checks that already live there.
`internal/core` is the frozen contract and the one place every surface passes through, so the command line,
the HTTP API, the browser editor and `ImportFrom` inherit one rule and one message. Validating in
`internal/web` instead would have left the same key reachable through the API and through an import, and
the stored key is what the browser route reads back.

The predicate is `validProjectKey`, unchanged and now reached through a shared `validateKey(what, s)` that
names the kind of key in the error. Two copies of the same character set would drift.

## Why not fix only the annotation

The redirect is not an open redirect, so the annotation could have been narrowed to say "same origin,
therefore fine". That leaves two live defects: a save that answers with a location outside the route the
handler named, and a stored workflow addressable at no URL the router serves. Both come from the key, so the
key is where it is fixed. What `path.Clean` does to a relative location is then a property the annotation can
state rather than a hazard it has to excuse.

## Measured, not assumed

`http.Redirect` was driven directly over the values in question before any of this was written:

| key | Location |
| --- | --- |
| `lean` | `/workflows/lean?flash=workflow+saved` |
| `../admin` | `/admin?flash=workflow+saved` |
| `../../evil.com` | `/evil.com?flash=workflow+saved` |
| `a?next=b` | `/workflows/a?next=b?flash=workflow+saved` |
| `a#frag` | `/workflows/a#frag?flash=workflow+saved` |
| `/evil.com` | `/workflows/evil.com?flash=workflow+saved` |
| `http://evil.com` | `/workflows/http:/evil.com?flash=workflow+saved` |

Nothing reached another origin: `path.Clean` collapses a leading `//`, and a location carrying no scheme and
no host is not rewritten into one. What did happen is the route prefix disappearing, and the flash landing
inside a query or a fragment. That is the set the guard covers.

## What is deliberately not changed

`redirect` does not gain the `?`-separator handling its sibling `redirectTo` carries. With the key validated,
no caller can hand it a path containing a query; adding the branch would be dead code guarding a case the
contract now refuses, and a second place for the two helpers to disagree.
