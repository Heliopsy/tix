# Tasks

- [ ] Measure per-package contribution properly (symbol sizes by package, not by section) and record it in design.md
- [ ] List operations reaching each candidate package through the capability registry; settle the exclusion list
- [ ] `tix_embedded` build tag: gate the browser interface, PostgreSQL store and terminal rendering registration
- [ ] Decide and gate `openapi.yaml`, MCP, external sync and inbound channels for the embedded build
- [ ] Registry flag for operations a tagged build omits; contract tests run for both builds
- [ ] Dependency guard test for the tagged build
- [ ] `lib*` recipes pass the tag; CI builds both and writes sizes to the job summary
- [ ] Record measured sizes per target; stop and record if the saving is under about 15 percent
- [ ] Docs: `docs/embedding.md` (what the embedded build serves, sizes)
