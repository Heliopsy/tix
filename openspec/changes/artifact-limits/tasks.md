# Tasks

## Service

- [x] `core.ArtifactLimits`, pattern parsing and byte sizes; drop `core.MaxBlobSize`
- [x] `core.Tenant` and `UpdateTenantInput` artifact fields and validation
- [x] Migration and both stores
- [x] `service.WithArtifactLimits`, tighten-only check in `UpdateTenant`, enforcement in `PutArtifact`
- [x] `ArtifactLimits` service method, registry entry, route, client, OpenAPI

## Configuration and serving

- [x] `artifacts.max_bytes` and `artifacts.allowed_types` with validation and classification
- [x] `tix serve --artifact-max-bytes` and `--artifact-allowed-types` join the flag layer when typed
- [x] Artifact routes take a body sized for the limit; SSH standard input too

## Surfaces

- [x] `tix tenant edit --artifact-max-bytes --artifact-types`, `tix artifact limits`, type from extension
- [x] Web: file input with `accept=`, limits stated, attachment-only download, tenant settings fields
- [x] TUI: tenant screen shows and edits both
- [x] Bulgarian strings
- [x] Docs: configuration, deployment, web UI, skill
