# A tenant can carry an icon

## Why

A person in several tenants picks between them by name alone, in the browser's switcher and in
the apps. Names like "Acme" and "Acme staging" read alike at a glance. A project already carries
an emoji, a built-in symbol or an uploaded image; a tenant should be able to carry the same, so
the switcher is clear at a glance.

## What Changes

- A tenant carries at most one icon, of the three kinds a project's can be: `icon` (an emoji or a
  one or two letter monogram), `icon_name` (a built-in icon from the same catalogue as projects)
  or `icon_image` (the digest of an uploaded PNG, JPEG, WebP or script-free SVG of at most
  64 KiB, checked exactly as a project icon is).
- `UpdateTenant` (`PATCH /api/v1/tenants/{ref}`, `tix tenant edit`, the tenant settings screen)
  takes `icon`, `icon_name` and `icon_file` the way `UpdateProject` does. Whoever may change the
  tenant's settings may set it. Setting one kind replaces the others; an empty value clears it.
- Every place that lists tenants to switch between carries the icon: `Tenant` objects,
  `whoami`'s `memberships` (`icon`, `icon_name`, `icon_image`) and `whoami`'s own tenant
  (`tenant_icon`, `tenant_icon_name`, `tenant_icon_image`).
- New service method `GetTenantIcon`, served at `GET /api/v1/tenants/{ref}/icon`
  (`tenant.icon`), at `GET /tenants/{key}/icon` in the browser and as `tix tenant icon REF`.
  Any member of the tenant reads it, even from a session or token in another of their tenants,
  because a switcher draws them all. A token reaches another tenant's icon only when it carries
  `account:read`, the scope under which it lists memberships at all.
- A tenant with no icon is drawn with a monogram of its name: the first letter of its first two
  words.
- Web: the project icon picker on the tenant settings screen; the icon on each row of the tenant
  switcher and on the sidebar entry that opens it; a tenant's own icon in place of the logo
  beside its name in the header.
- CLI: `--icon`, `--icon-name` and `--icon-file` on `tix tenant edit`, an ICON column in the
  tenant table and a MARK column in `tix tenant ls` for a member. The terminal interface lists
  each membership behind a text stand-in.

## Impact

- One migration adds `icon`, `icon_name`, `icon_image`, `icon_type`, `icon_data` and
  `icon_updated_at` to `tenants`. The bytes sit on the tenant row, which belongs to no tenant,
  so a switcher reads the icon of any tenant it lists and a global admin sets the icon of a
  tenant they stand outside, in the same transaction as the rest of the update.
- `core.Tenant`, `core.AccountMembership` and `core.Actor` gain icon fields; `UpdateTenantInput`
  gains `Icon`, `IconName` and `IconFile` (a `ProjectIconFile`).
- One new `Service` method with a registry entry on every surface, the terminal interface
  exempted because it draws no image.
