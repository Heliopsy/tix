# Configurable artifact limits

## Why

An artifact blob is stored inline in the database, and the only bound on it is `core.MaxBlobSize`, a 1 MiB
constant. An operator cannot raise it for a deployment that keeps screenshots or reports on tasks, cannot lower it
for one that should not, and cannot say which media types are welcome. Nothing stops an HTML or SVG file being
attached, and once the browser can upload and download files that matters.

## What Changes

- Server keys `artifacts.max_bytes` (1 MiB) and `artifacts.allowed_types` (common images, PDF, text, CSV, JSON,
  YAML, archives and `application/octet-stream`), each with its `TIX_` variable and a `tix serve` flag.
- A tenant administrator can tighten both for the tenant (`artifact_max_bytes`, `artifact_types` on the tenant),
  from Tenant settings, `tix tenant edit`, the terminal tenant screen and the API. A value past the server's is
  refused naming the server's. Unset inherits.
- The service enforces the effective limits on `PutArtifact`, so every transport gets them, with messages that
  state the limit and the accepted types. Existing artifacts are not judged again.
- `ArtifactLimits` (`GET /v1/artifact-limits`, `tix artifact limits`) reports the limits in force.
- The artifact upload routes take a body sized for `artifacts.max_bytes`, so `--max-body-bytes` never refuses an
  upload under it.
- The browser's artifact form takes a file, states the limits up front and sets `accept=`. A stored file is
  offered as a download only, never inline.
- Migration adding `tenants.artifact_max_bytes` and `tenants.artifact_types`.
