## ADDED Requirements

### Requirement: Project icon flags

`tix project create` and `tix project edit` SHALL accept `--icon` for a text icon, `--icon-name`
for a built-in icon and `--icon-file` for a path to an image to upload, and SHALL refuse more than
one of them with a non-empty value. `tix project icon KEY` SHALL write the project's uploaded image
to a file named with `--out`, or to standard output when it is not a terminal.

#### Scenario: Built-in icon from the command line

- **WHEN** `tix project edit infra --icon-name rocket` is run
- **THEN** the project carries the built-in rocket icon and `tix project ls` shows its fallback glyph and name

#### Scenario: Image from the command line

- **WHEN** `tix project edit infra --icon-file logo.png` is run and then `tix project icon infra --out copy.png`
- **THEN** `copy.png` holds the same bytes as `logo.png`

#### Scenario: Two kinds are refused

- **WHEN** `tix project create web Web --icon W --icon-name globe` is run
- **THEN** the command fails with exit code 2 and creates nothing
