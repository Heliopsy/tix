## ADDED Requirements

### Requirement: Completion installs itself where the shell looks

The command line SHALL install a completion script to the location the named shell loads completions from,
for bash, zsh and fish, and SHALL report the path it wrote.

The shell SHALL be taken from the environment when it is not given explicitly, and an explicit choice SHALL
override it. An unsupported shell SHALL be refused.

Installation SHALL write only under the invoking user's own directories, and SHALL NOT modify shell startup
files. Where a shell needs something further before completion works, the command SHALL say so.

Installing twice SHALL succeed and leave the same result.

#### Scenario: Installing for the running shell

- **WHEN** completion is installed with no shell named and the environment names a supported shell
- **THEN** the script is written to that shell's completion directory and the path is reported

#### Scenario: An explicit shell overrides the environment

- **WHEN** completion is installed with a shell named explicitly
- **THEN** that shell's script is written, whatever the environment says

#### Scenario: A dry run writes nothing

- **WHEN** completion is installed as a dry run
- **THEN** the path that would be written is reported and no file is created or changed

#### Scenario: Installing twice is not an error

- **WHEN** completion is installed and then installed again
- **THEN** the second run succeeds and the file matches the first

#### Scenario: Uninstalling removes what was written

- **WHEN** completion is uninstalled
- **THEN** the file this command writes is removed, and removing it when absent is reported rather than failed

#### Scenario: An unsupported shell is refused

- **WHEN** completion is installed for a shell that is not supported
- **THEN** the command fails with the exit code for unusable input and names the shells that are supported
