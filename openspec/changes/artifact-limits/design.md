# Design

- **Columns on tenants, not a table.** The tenant's limits sit beside `lease_max`, which already follows the same
  rule: zero inherits the deployment's, and a tenant may only tighten it. `artifact_types` is a comma separated
  list in one text column; empty inherits.
- **Refuse at the setting, clamp at use.** Setting a tenant value above the server's is refused while the person
  is looking, naming the server's limit. If the operator later lowers the server's limit, the effective limit is
  the lower of the two, and the effective types are those both lists accept, so nothing is ever loosened by a
  stale tenant row.
- **Patterns.** `type/subtype`, `type/*` and `*/*`. A tenant pattern must be covered by a server pattern. A blob
  with no content type is judged as `application/octet-stream`, which the default list accepts so existing agents
  that send bare blobs keep working; the CLI fills the type from the file's extension.
- **Only blobs are judged.** An artifact with no blob carries nothing the limits describe.
- **Body limit.** The blob travels base64 in JSON, or as a multipart part from the browser. The artifact routes
  take `max(--max-body-bytes, ceil(max_bytes/3)*4 + 1 MiB)`, mirroring how `server.max_import_bytes` widens the
  import routes. SSH standard input takes the larger of the import and artifact limits.
- **Serving.** Blobs were only ever returned inside JSON. The new browser download always answers
  `Content-Disposition: attachment`, `nosniff` and a sandboxing CSP, whatever the stored type says.
- **One new service method.** `ArtifactLimits` is a read with a registry entry; the tenant settings ride on
  `UpdateTenant`, which already writes the row, its audit entry and its event in one transaction.
