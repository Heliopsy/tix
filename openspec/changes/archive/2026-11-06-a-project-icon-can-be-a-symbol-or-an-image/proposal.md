# A project icon can be a symbol or an image

## Why

A project's icon is one or two characters today, which covers an emoji and a monogram and
nothing else. People asked for two more choices: a clean line icon that matches the rest of the
interface instead of whatever emoji font the reader's system ships, and their own logo.

## What Changes

- A project's icon is exactly one of three kinds, or none:
  - **text**: an emoji or a one or two letter monogram, as today (`icon`);
  - **built-in**: a named symbol from the interface's own sprite (`icon_name`), drawn from a
    fixed, curated set of project symbols such as folder, server, database, rocket or shield;
  - **image**: an uploaded PNG, JPEG, WebP or SVG of at most 64 KiB (`icon_image`, which holds
    the image's content digest; the bytes live in their own table).
- Setting one kind replaces whichever kind the project had. Clearing any of them clears the icon.
- An uploaded image is sniffed rather than trusted for its type, is size capped, and an SVG that
  carries script, event handlers, `foreignObject` or external references is refused. Images are
  served with their own content type, `nosniff`, a sandboxing CSP and a strong ETag, and the
  browser only ever draws them through `<img>`, where no script runs.
- Image bytes are stored in the database, so SQLite and PostgreSQL both hold them, scoped to the
  tenant and read through the same project read authorization as the project itself.
- New service method `GetProjectIcon`, served at `GET /api/v1/projects/{ref}/icon`, at
  `GET /projects/{key}/icon` in the browser and as `tix project icon KEY` on the CLI.
- Create and update accept `icon_name` and `icon_file` (content type and base64 bytes) beside
  `icon`. The CLI gains `--icon-name` and `--icon-file` on `tix project create` and
  `tix project edit`.
- The project forms get a picker: Emoji, Icon and Upload segments, a grid of the built-in
  symbols, a preview, and a way to remove the icon.
- The terminal interface and the CLI table, which cannot draw an image or a sprite, show a text
  stand-in: each built-in symbol has a fallback glyph, and an uploaded image shows a fixed mark.
- Snapshots and bundles carry text and built-in icons. They do not carry uploaded images; an
  import that updates a project keeps the image it already has unless the record names a text or
  built-in icon.

## Impact

- Migration 0016 adds `projects.icon_name`, `projects.icon_image` and the `project_icons` table
  on both engines; existing icons stay text icons and need no rewrite.
- `core.Project` gains `IconName` and `IconImage`; inputs gain `IconName` and `IconFile`.
- One new `Service` method with a registry entry on every surface (the terminal one exempted,
  since a terminal draws no image).
