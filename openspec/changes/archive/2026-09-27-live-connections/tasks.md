# Tasks

## 1. The registry

- [x] 1.1 `internal/connections/registry.go`: entries, register, unregister, list by tenant, counts, end by id
- [x] 1.2 Concurrency: registering, ending and listing all happen from different goroutines. Tested under `-race`
- [x] 1.3 `internal/connections/registry_test.go`: lifecycle, a closed entry leaving, ending one and only one

## 2. Feeding it

- [x] 2.1 `internal/httpapi`: the hub registers on upgrade and unregisters on close, beside its existing counters
- [x] 2.2 `internal/sshd`: register a session when it opens, unregister when it ends
- [x] 2.3 `internal/sshd`: `Drain` uses those handles to tell a session still open at the deadline why it is
      closing, which is the half of the shutdown requirement currently missing

## 3. Contract

- [x] 3.1 `internal/core`: `Connection`, `ConnectionCounts`, and `ListConnections` / `EndConnection` on the
      right sub-interface. Stdlib only. **Do not call these sessions**: `core.Session` is the login session
- [x] 3.2 The response carries which server answered

## 4. Service

- [x] 4.1 `internal/service/connection.go`: both methods under `authz.ActionTenantAdmin`, tenant-scoped
- [x] 4.2 Ending writes an audit entry and an event, and commits them **before** closing the connection
- [x] 4.3 An identifier from another tenant is reported as not found, disclosing nothing

## 5. Surfaces

- [x] 5.1 `cmd/connection.go`: `tix connection ls`, `tix connection kill ID`
- [x] 5.2 `internal/httpapi`: the routes and handlers
- [x] 5.3 `internal/web`: the screen, its template and its handlers, working without JS
- [x] 5.4 `internal/capability`: registry entries with CLI, HTTP and Web bindings, or the build fails
- [x] 5.5 `internal/client`: the two methods, no validation

## 6. Security

Each verified by mutation: break the check, watch the test fail, restore it.

- [x] 6.1 Another tenant's connections are absent from the list and from the tenant counts
- [x] 6.2 Another tenant's identifier cannot be ended and is reported as not found, not as forbidden
- [x] 6.3 The process total leaks no tenant, actor or address
- [x] 6.4 Ending is refused without `tenant.admin`
- [x] 6.5 The audit entry exists even when the close fails
- [x] 6.6 Ending is not revocation: the holder may reconnect
- [x] 6.7 Leak suite extended across the service, the API routes and the web handlers

## 7. Documentation

- [x] 7.1 `docs/deployment.md`: what the view covers, and that it is one server's
- [x] 7.2 Note the relationship to key revocation: revoking stops the next connection, ending stops this one
