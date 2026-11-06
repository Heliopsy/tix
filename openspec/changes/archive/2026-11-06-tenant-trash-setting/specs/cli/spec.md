## ADDED Requirements

### Requirement: The command line sets and reports the trash

`tix tenant edit` SHALL accept `--trash on|off`, refusing any other value as a usage error. `tix task
trash` SHALL report whether the trash is on and how many tasks it holds. With the trash off, `tix task
rm` SHALL keep its flags and SHALL NOT prompt, and SHALL say on stderr that the delete is permanent.

#### Scenario: Turning the trash off from the shell

- **WHEN** `tix tenant edit default --trash off` runs and then `tix task rm default-1`
- **THEN** stderr says the delete is permanent, and `tix task ls --trashed` exits 6
