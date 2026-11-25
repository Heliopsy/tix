# Design: a slimmer embedded build

## Context

`embedded.Start` and `tix serve` share `internal/serving.Build`, which is why they cannot drift. The cost
is that the library carries every package the server links. Sizes below are from an unstripped
`go build -buildmode=c-shared ./embedded/libtix` on linux/amd64 at b66e8b44, read from ELF section
headers; the stripped file is about 31 MB.

| Section | Size |
| --- | --- |
| `.text` | 14.0 MB |
| `.gopclntab` | 9.9 MB |
| `.rela.dyn` + `.data.rel.ro` | 5.0 MB |
| `.rodata` | 2.4 MB |
| `.noptrdata` | 1.2 MB |

Embedded files: `internal/web/assets` 1.3 MB, `internal/web/templates` 0.6 MB, `api/openapi.yaml` 0.5 MB,
`CHANGELOG.md` 76 KB. Notable packages by code size: `jackc/pgx` (PostgreSQL), `mattn/go-runewidth` and
`jedib0t/go-pretty` (terminal tables), `yuin/goldmark` (Markdown to HTML for the browser).

## Decisions to make

- **Tag, not a second composition.** One `Build` with tag-gated registration keeps serve and embed from
  drifting. Files that register the browser interface or a driver get a `//go:build !tix_embedded`
  counterpart that registers nothing.
- **What stays.** Anything an API operation needs. Before excluding a package, list the operations that
  reach it through the capability registry; an operation the app binds must keep working.
- **`openapi.yaml`.** It is embedded for `/api/v1/openapi.yaml` and MCP. Whether the embedded build serves
  it is open: the app vendors its own copy, so it can probably go.
- **Contract tests.** The registry gains a per-operation flag for "not in embedded builds", and the HTTP
  contract test runs against both builds in CI.
- **Guard.** A test lists the packages a tagged build links (`go list -deps -tags tix_embedded`) and fails
  on any excluded one, the same way `TestTheBinaryReachesNoCgo` guards cgo.

## Risks

- Hidden coupling: a service path may import the web package for a helper. The dependency guard finds it;
  the helper moves.
- The saving is a guess until measured. If it comes in under about 15 percent, record that and stop: the
  added build matrix would not be worth it.
