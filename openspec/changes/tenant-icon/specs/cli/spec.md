## ADDED Requirements

### Requirement: Tenant icon flags and command

`tix tenant edit` SHALL take `--icon`, `--icon-name` and `--icon-file`, each applied only when
given, an empty value clearing the icon. `tix tenant icon REF` SHALL write the tenant's uploaded
image to `--out` or to standard output when that is not a terminal. The tenant table SHALL show
an ICON column and a member's `tix tenant ls` a MARK column with the icon or monogram as text.

#### Scenario: Set a built-in icon from the command line

- **WHEN** an admin runs `tix tenant edit acme --icon-name rocket`
- **THEN** the tenant carries `icon_name` `rocket` and the ICON cell of `tix tenant show acme` names rocket
