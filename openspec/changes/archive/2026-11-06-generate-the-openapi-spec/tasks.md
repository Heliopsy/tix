## 1. Generator

- [x] 1.1 `Contract` on registry operations and `capability.Endpoints` for routes outside the one binding
- [x] 1.2 `internal/openapi`: reflection with swaggest, doc comments and closed enums from the source, error
  responses from `core.Kind`, security from scopes, OpenSpec links
- [x] 1.3 `just openapi` writes `api/openapi.yaml` and the routes table of docs/api.md

## 2. Serving

- [x] 2.1 `GET /api/v1/openapi.json`, public and unscoped, from the embedded document

## 3. Guards

- [x] 3.1 Drift: the committed files equal a fresh generation, in `go test` and `just docs-check`
- [x] 3.2 Coverage: the router's registered patterns equal the document's operations
- [x] 3.3 Contract: the HTTP API fixture validates requests and responses against the document
- [x] 3.4 OpenSpec: every route the http-api spec names is in the document
- [x] 3.5 Public operations agree with the authentication middleware

## 4. Docs

- [x] 4.1 docs/api.md: the endpoint, `just openapi`, the generated routes table
- [x] 4.2 AGENTS.md: registry and document rules, contract overrides, known flakes
