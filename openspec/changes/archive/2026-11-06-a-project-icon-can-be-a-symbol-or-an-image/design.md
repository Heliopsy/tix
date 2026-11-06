# Design

## Three columns and a side table, not one encoded string

The icon could have stayed one `icon` column with a prefix (`builtin:rocket`, `image:...`).
That would have kept the schema still, and broken every reader that already treats `icon` as
the text to draw: the CLI table, the TUI, exported snapshots, API clients. So `icon` keeps its
meaning, and two columns are added beside it:

- `icon_name`: the built-in symbol's name, validated against a fixed list in `core`;
- `icon_image`: the uploaded image's digest, empty when there is none.

The service keeps the invariant that at most one of the three is set. A reader that only knows
`icon` sees an empty icon for the new kinds, which is what an older client would draw anyway.

## Bytes in their own table

Image bytes live in `project_icons`, keyed by project, with the tenant column and on PostgreSQL
the row-level isolation policy every tenant-owned table has. Listing projects never reads them:
the project row carries only the digest, which is enough to build a versioned URL. The portable
schema's `BLOB` already maps to `BYTEA` on PostgreSQL, so one migration file serves both engines.
The row cascades with its project and its tenant.

The image is written inside `CreateProject` and `UpdateProject`, so the row, the project update,
its audit entry and its event are one transaction. There is no separate upload method.

## Serving

`GetProjectIcon` authorizes a project read and returns content type, digest and bytes. Both the
API route and the browser route answer with the bytes, not JSON, so an `<img>` and `curl -o` both
work. Headers: the sniffed content type, `X-Content-Type-Options: nosniff`,
`Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; sandbox`, a strong ETag
from the digest, and `Cache-Control: private, max-age=31536000, immutable` on the browser route,
whose URL carries the digest as `?v=`. A matching `If-None-Match` answers 304.

## SVG

SVG is accepted because a logo is usually one. It is refused when it carries anything that can
run or fetch: `script`, `foreignObject`, any `on*` attribute, a `javascript:` URL, or an
`href` that is not a fragment or a `data:` image. It is then only ever drawn through `<img>`,
where browsers run no script and load nothing external, and served with a sandboxing CSP in case
it is opened directly. Either defence alone would do; both is cheap.

## Types are sniffed, not trusted

The declared content type of an upload is ignored. PNG, JPEG and WebP are recognised by their
magic bytes, SVG by parsing to an `svg` root element. Anything else is refused.

## Built-in symbols

The set lives in `core` as names with a text fallback glyph, so the TUI and CLI can draw
something. The sprite carries one `<symbol id="pi-NAME">` per name, in the sprite's existing
style (24 unit grid, 2 unit stroke, round caps, `currentColor`). A test holds the two lists
together.

## Transfer

Snapshots and bundles carry `icon` and `icon_name`. An uploaded image does not travel: it is a
binary in a line-oriented document and the snapshot format has no place for one. An import that
updates an existing project keeps that project's image when the incoming record names no text or
built-in icon, and drops it when it does.
