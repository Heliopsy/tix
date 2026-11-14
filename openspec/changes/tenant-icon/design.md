# Design

## Where the image lives

A project's image bytes live in `project_icons`, a tenant-scoped table. A tenant's cannot: the
switcher of a person in tenant A draws the icon of tenant B, and a global admin updating
tenant B does it from a transaction scoped to their own tenant. A scoped table would need a
second transaction in B's scope for both, splitting the update from its audit entry.

The `tenants` row is already outside tenant scoping, so the image goes on it: `icon_type`,
`icon_data` and `icon_updated_at` beside `icon_image`, the digest. No listing selects
`icon_data`, so it is read only by `GetTenantIcon`.

## Who may read it

`GetTenantIcon` needs only a signed-in actor, like `WhoAmI`. The caller's own tenant always
answers. Another tenant answers when the caller is a global admin or when the caller's account
holds a membership in it; anything else is not found, the same answer as a tenant that does not
exist. A token answers for another tenant only when it carries `account:read`, because a token
without it is not told its owner's other tenants and must not be able to probe them one key at
a time.

## Contract shape

The fields mirror a project's exactly: `icon`, `icon_name` and `icon_image` on what is read,
`icon`, `icon_name` and `icon_file` on the update. `icon_file` reuses `ProjectIconFile`, since
the shape is identical. `Actor` already prefixes its tenant fields (`tenant_key`,
`tenant_name`), so its copy is `tenant_icon`, `tenant_icon_name` and `tenant_icon_image`.

## Monogram

With no icon, surfaces draw `core.Monogram(name)`: the upper-cased first letter of each of the
first two words, splitting on anything that is not a letter or digit. The web switcher, the
sidebar entry, the CLI and the terminal interface use it; the header keeps the tix logo instead,
since the tenant's name is written right beside it.
