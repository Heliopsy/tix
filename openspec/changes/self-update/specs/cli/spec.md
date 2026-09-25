## ADDED Requirements

### Requirement: The binary can replace itself with a published release

The command line SHALL replace the running binary with a release built for its own operating system and
architecture, and SHALL report the version it moved from and to.

It SHALL verify the downloaded archive against the checksum published with that release, and SHALL refuse
to install an archive whose checksum does not match or that the checksum file does not list.

The replacement SHALL be atomic: the new binary SHALL be written beside the target and moved into place,
so that an interrupted or failed update never leaves a partially written file where the binary was.

#### Scenario: Updating to the current release

- **WHEN** a newer release exists for this platform and the binary is one this command may replace
- **THEN** the archive for this platform is downloaded, its checksum verified, and the binary replaced,
  and the output names the version before and after

#### Scenario: A checksum that does not match is refused

- **WHEN** the downloaded archive's checksum differs from the one published for it
- **THEN** nothing is installed, the existing binary is untouched, and the failure says the checksum did
  not match

#### Scenario: An archive the checksum file does not list is refused

- **WHEN** the published checksum file carries no entry for the archive that was downloaded
- **THEN** nothing is installed and the failure says the archive was not listed

#### Scenario: An interrupted download leaves the binary alone

- **WHEN** the download or extraction fails part way through
- **THEN** the binary that was already there still runs and no partial file is left in its place

#### Scenario: Already current

- **WHEN** the running version is the newest release
- **THEN** the command reports that it is current, writes nothing, and succeeds

### Requirement: It refuses what is not its to replace

The command SHALL determine how the running binary was installed, and SHALL refuse rather than overwrite a
binary owned by another tool, naming the command that would do the upgrade instead.

A target that cannot be written SHALL be reported before anything is downloaded.

#### Scenario: A binary installed with the Go toolchain

- **WHEN** the running binary was installed by the Go module toolchain
- **THEN** the command refuses and names the `go install` command that would upgrade it

#### Scenario: A binary a package manager owns

- **WHEN** the running binary resolves inside a path a package manager controls
- **THEN** the command refuses and says the binary is managed elsewhere

#### Scenario: An unwritable destination is reported first

- **WHEN** the directory holding the binary cannot be written by this user
- **THEN** the command fails before downloading anything and says the destination is not writable

### Requirement: Checking does not install

The command SHALL offer a mode that reports whether a newer release exists and exits without downloading
or writing anything, and SHALL report the absence of a newer release as success rather than as an error.

#### Scenario: Checking when a newer release exists

- **WHEN** the check mode runs and a newer release exists
- **THEN** it names that version and no file is created, downloaded or replaced

#### Scenario: Checking when current

- **WHEN** the check mode runs and this binary is the newest release
- **THEN** it says so and succeeds

### Requirement: A specific version may be named

The command SHALL accept a release to install instead of the newest, including one older than the running
binary, and SHALL refuse a version that has no release for this platform.

#### Scenario: Installing a named release

- **WHEN** a version is named and a release exists for it on this platform
- **THEN** that release is installed, whether it is newer or older than the running binary

#### Scenario: A version with no release for this platform

- **WHEN** a version is named that publishes nothing for this operating system and architecture
- **THEN** nothing is installed and the failure names the platform it looked for
