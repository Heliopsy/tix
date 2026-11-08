# Tasks

- [x] `SyncHost` option and per-tenant scope in `internal/service`; shared unless trusted
- [x] Confined reads (`EvalSymlinks`, `filepath.Rel`, `os.Root`, `O_NOFOLLOW`) in `internal/sync`
- [x] Generic adapter takes the file's bytes from the service
- [x] Uniform refusals for a stored path; no stat outside the tenant directory
- [x] `sync.env_sources` bindings; unbound sources see no environment
- [x] Environment credentials only to the environment's url
- [x] `sync.file_root` and `sync.env_sources` config keys, `tix serve` marks its service shared
- [x] Tests: service, confined reads, connect wiring, a real `tix serve`, HTTP transport matrix
- [x] Web dialog text and Bulgarian strings, CLI help, docs, skill
