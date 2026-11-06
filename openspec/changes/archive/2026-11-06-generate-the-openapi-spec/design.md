# Design

## The registry is the source

Every route already has to be declared once in `internal/capability`, and the parity tests fail when a service
method is missing from it. Generating the document from the registry makes that one declaration describe the
route too, so adding a route and documenting it are the same change.

Alternatives considered:

- **swag annotations.** A second description of every handler in comments, which nothing checks against the code.
  It drifts exactly the way the hand-written routes table did.
- **huma, or an OpenAPI-first framework.** Every handler would be rewritten to the framework's signature. The
  handlers are plain `net/http` and the service contract is already the typed layer.
- **oapi-codegen from a hand-written document.** The document becomes the source and the Go types are generated
  from it, the reverse of how this codebase is built: `internal/core` is the frozen contract.

The registry and reflection need no annotations and no handler change, and produce OpenAPI 3.1, whose schemas are
JSON Schema 2020-12.

## Defaults and contracts

For most operations the service method's signature is the wire contract: the request body is the method's last
struct parameter on a POST, PUT or PATCH, the response its first result, a slice result is answered as a
`wire.Page` of its elements, and an error-only method answers 204. Where a handler departs from that it says so on
the registry entry with `Contract`: `Body`, `Resp`, `Query`, `Content`, `Alt`, `Status`, `Also` and `Public`. A GET
or DELETE whose method takes a struct the route carries in no body fails generation until it declares `Query`, so
a list filter cannot go undocumented by default.

Routes that are not the one binding of a service method are `capability.Endpoints`, carrying the same contract.

Two operations sharing one method and path, told apart by a query discriminator (archiving and deleting a
project), are one OpenAPI operation, since OpenAPI allows one per method and path; the second is named in its
description and its status added to its responses.

## Determinism

Every map is written sorted and the top level in a fixed order, the version is a constant rather than the
build's, and nothing depends on the machine. The document is committed, so a review shows what a change does to
the API.

## Guards

- The committed document and routes table equal a fresh generation (`just docs-check` and `go test`).
- The router's own registrations, walked through a mux that records them, equal the document's operations both
  ways, so a handler with no registry entry is caught as surely as an entry with no handler.
- The HTTP API test fixture validates every request it accepts and every response it gives against the document,
  naming the operation and the field, and fails a query parameter the operation does not declare.
- Every route the http-api spec mentions resolves to an operation of the document.
- The document's public operations are the ones the authentication middleware lets through.

## Follow-up

An MCP server generated from this document, so an agent's tools cannot drift from the API either.
