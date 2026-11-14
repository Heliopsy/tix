## ADDED Requirements

### Requirement: A tenant carries at most one icon

A tenant SHALL carry at most one of `icon` (one emoji or a one or two letter monogram),
`icon_name` (a built-in icon from the set projects use) and `icon_image` (the digest of an
uploaded image, checked as a project icon image is). Setting one kind SHALL replace the others,
and setting any of them empty SHALL clear the icon.

#### Scenario: A built-in icon replaces an image

- **WHEN** a tenant with an uploaded icon image is updated with `icon_name` set to `rocket`
- **THEN** the tenant carries `icon_name` `rocket`, no `icon` and no `icon_image`, and its image is no longer stored

#### Scenario: Two kinds at once are refused

- **WHEN** a tenant update names both `icon` and `icon_name`
- **THEN** it is refused as invalid on the `icon` field and the tenant is unchanged

### Requirement: A tenant without an icon is drawn with a monogram

A surface drawing a tenant that carries no icon SHALL draw its monogram: the upper-cased first
letter of each of the first two words of its name.

#### Scenario: Monogram of a two-word name

- **WHEN** a tenant named `Acme staging` carries no icon
- **THEN** its monogram is `AS`
