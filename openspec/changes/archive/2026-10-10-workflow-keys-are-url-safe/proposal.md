# A workflow key holds the shape a key addressable from a URL has to hold

## Why

A workflow key was validated for one thing: that it was not empty. Every other key in the system that is
spent as a URL path segment — a project key, a tenant key — goes through `ValidateProjectKey`, which
narrows it to letters, digits, hyphens and underscores. A workflow key did not, although it is spent the
same way: `/workflows/{key}` on the browser surface, and `RouteWorkflows + "/" + key` is what the editor
redirects to after a save.

That combination produced a redirect that left its own route. `http.Redirect` runs `path.Clean` over a
relative location, so a key of `../admin` answered with `Location: /admin`. It is not an open redirect —
nothing reachable through it carries a scheme or a host, and a leading `//` does not survive the clean —
but the route prefix the handler chose does not survive either. A key carrying `?` or `#` instead swallowed
the flash message into a query or a fragment: `/workflows/a?next=b?flash=workflow+saved`.

The annotation on the redirect helper asserted the opposite. It said the path "is a route constant chosen
by the handler … neither is a caller-supplied destination", which was false for every call that appends a
segment, and the next reader would have trusted it. An untrue security annotation is worse than none.

Validating the key is the fix rather than rewriting the annotation alone, because the stored key is also
what the route reads back: a workflow saved under `a/b` was addressable at no URL the router serves.

## What Changes

- **`WorkflowInput.Validate` narrows the key** to the shape a project key already holds: starts with a
  letter, ends with a letter or digit, contains only letters, digits, hyphens and underscores, at most 64
  characters. The refusal is an invalid-input error naming the workflow key.
- **A new `core.ValidateWorkflowKey`**, sharing the one predicate with `ValidateProjectKey` so the two
  cannot drift, and naming which kind of key it rejected.
- **The `#nosec G710` justification on `redirect` becomes true**: it states that the path is a route
  constant followed by at most one already-validated segment, and says what would break if a segment
  carried a separator.
- **Every surface inherits it.** The key is validated in the contract, so the command line, the HTTP API,
  the browser editor and an import through `ImportFrom` all refuse the same keys for the same reason.

## Impact

- Affected specs: `workflows`
- Affected code: `internal/core/ref.go`, `internal/core/input.go`, `internal/web/render.go`,
  `internal/core/ref_test.go`, `internal/web/workfloweditor_test.go`
- No schema change. Existing stored keys are untouched; the check runs on a write.
- Behaviour change: `PutWorkflow` refuses keys it used to accept, on every surface, including a snapshot
  import carrying one. A workflow already stored under such a key cannot be saved again without renaming
  it. This is intended: a key the router cannot address was never usable.
