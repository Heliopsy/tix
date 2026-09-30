# Tasks

## 1. Contract

- [x] 1.1 `internal/core/ref.go`: `ValidateWorkflowKey`, and `ValidateProjectKey` rebuilt on a shared
      `validateKey` that names the kind of key it rejects
- [x] 1.2 `internal/core/input.go`: `WorkflowInput.Validate` runs the key through it

## 2. Browser surface

- [x] 2.1 `internal/web/render.go`: the `#nosec G710` justification on `redirect` states what the path
      actually is — a route constant plus at most one validated segment — and what `path.Clean` would do to a
      segment carrying a separator

## 3. Guards

- [x] 3.1 `internal/core/ref_test.go`: the accepted and refused key sets, the error naming the workflow key,
      and `WorkflowInput.Validate` refusing an escaping key
- [x] 3.2 `internal/web/workfloweditor_test.go`: the editor refuses each dangerous key with 400 and no
      `Location`, and still redirects a usable key under the workflows prefix
