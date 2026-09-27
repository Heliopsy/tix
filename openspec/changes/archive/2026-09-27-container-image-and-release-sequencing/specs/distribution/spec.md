## ADDED Requirements

### Requirement: A published container image for both server architectures

The project SHALL publish a container image to a registry for `linux/amd64` and `linux/arm64`, addressed by a
single manifest list so that one reference serves both. The binary inside each architecture's image SHALL be
built for that architecture, which SHALL be established from the binary itself rather than from the image's
declared platform.

#### Scenario: One reference serves both architectures

- **WHEN** the manifest list for a release is inspected
- **THEN** it carries exactly one entry for `linux/amd64` and one for `linux/arm64`

#### Scenario: The binary matches the architecture it is labelled with

- **WHEN** the binary is extracted from the image built for an architecture
- **THEN** its machine type is that architecture's, and a build that lost its cross-compile is refused

#### Scenario: Building both architectures needs no emulation and no registry

- **WHEN** both architectures are built on one host
- **THEN** the compiler runs on the host's architecture throughout, and the manifest list is assembled from
  local storage

### Requirement: The published tags carry the compatibility the version scheme promises

A release SHALL publish its exact version, its minor series, and `latest`. While the project is below 1.0 it
SHALL NOT publish a bare major tag, because a minor bump may break compatibility and a major tag would move a
consumer across that break without their having named a version.

A reference that is not a three-part numeric version SHALL be refused rather than turned into tags.

#### Scenario: A release publishes three tags

- **WHEN** the tags for version 1.4.2 are resolved
- **THEN** they are exactly `1.4.2`, `1.4` and `latest`

#### Scenario: No bare major tag is published

- **WHEN** the tags for a release are resolved
- **THEN** no tag consisting of the major version alone is among them

#### Scenario: A development version produces no tags

- **WHEN** a version describing an untagged commit, such as `v0.5.0-2-g6fd7de6`, is offered
- **THEN** resolving tags fails rather than publishing that string alongside `latest`

### Requirement: The published image is signed and carries its provenance

The image SHALL be no weaker than the release archives. It SHALL carry a signature covering both the manifest
list and each architecture manifest, an SBOM attestation for each architecture attested to that architecture's
own digest, and build provenance recording what produced the digest.

#### Scenario: A consumer on either architecture can verify what they pulled

- **WHEN** the image is verified on `linux/arm64`
- **THEN** the signature covers the architecture manifest that was pulled, not only the list above it

#### Scenario: An SBOM describes the image it is attached to

- **WHEN** the SBOM attestation for an architecture is read
- **THEN** it was produced from that architecture's image, not from the other one

### Requirement: The image is verified by running it

The image SHALL be gated by starting a container from it and observing what it serves, not by the build
succeeding. The gate SHALL run identically from the project's task runner and in continuous integration.

#### Scenario: The server is reachable on the port the image declares

- **WHEN** a container is started from the image and its declared port is published
- **THEN** a request to the health endpoint on the published port is answered

#### Scenario: The health endpoint reports this build

- **WHEN** the health endpoint is read
- **THEN** it reports a healthy status and the version this build stamped into the binary

#### Scenario: Readiness names what it checked

- **WHEN** the readiness endpoint is read
- **THEN** it reports the database and the migrations as individually ready

#### Scenario: The browser interface and its assets are served

- **WHEN** the root path is requested
- **THEN** the sign-in page renders and its stylesheet is served from the binary's embedded assets

#### Scenario: The server does not run as root

- **WHEN** the uid of the running server process is read
- **THEN** it is the image's non-root user, established from the process rather than from the image's declared
  user

#### Scenario: A database on the data volume outlives its container

- **WHEN** a task is created through the image's binary, the container is destroyed, and a new container is
  started over the same volume
- **THEN** that task is still there

### Requirement: The image is scanned as an image

Vulnerability scanning SHALL cover the built image's layers, which is what a consumer receives, in addition to
the source tree. An accepted finding SHALL state why it is accepted and how to re-check that claim.

#### Scenario: The scan reads the layers

- **WHEN** the image is scanned
- **THEN** the base image's own packages and the modules carried by the shipped binary are both examined

#### Scenario: A finding accepted for the source tree is re-argued for the image

- **WHEN** a finding is suppressed for the image
- **THEN** the reachability claim behind it is one that can be re-verified, and the command to re-verify it is
  recorded

### Requirement: A release is not resolvable until its binaries are attached

A release SHALL NOT be the release that the latest-release lookup resolves until its archives are attached to
it. Becoming resolvable and gaining its archives SHALL be one transition, so there is no interval in which the
published install command resolves a release it cannot download from.

#### Scenario: A release under construction is not offered

- **WHEN** a release has been created but its build has not uploaded its archives
- **THEN** the latest-release lookup resolves the previous release, which still has its archives

#### Scenario: A release becomes resolvable with its archives already present

- **WHEN** the build finishes uploading
- **THEN** the release is published, and it is resolvable and complete from that moment

#### Scenario: A failure part-way leaves nothing resolvable but broken

- **WHEN** the release process fails after creating the release and before attaching its archives
- **THEN** the release is not what the latest-release lookup resolves, and the install command keeps working
  against the previous release

### Requirement: A release left unfinished is detected and finished

The release process SHALL detect a release that exists without its archives and ask for its build again, without
relying on the step that should have asked. Detection SHALL identify the release the project's own manifest
names rather than asking which release is latest, because an unfinished release is deliberately not the latest
one.

#### Scenario: A release whose build was never requested is picked up

- **WHEN** a release exists unpublished with no archives and no build is in flight
- **THEN** its build is requested

#### Scenario: A release missing the tag its build needs is repaired

- **WHEN** an unpublished release has no tag, because publication is what would have created one
- **THEN** the tag is created at the commit the release names, before the build is requested

#### Scenario: A build already running is not duplicated

- **WHEN** a release is unfinished and a release build is already in flight
- **THEN** no second build is requested

#### Scenario: A finished release is left alone

- **WHEN** the release the manifest names is published with its archives
- **THEN** nothing is requested

#### Scenario: A release whose assets need replacing can be rebuilt

- **WHEN** the build is requested again for a tag whose release is already published
- **THEN** the upload reaches that release rather than a second one created beside it, and the release is
  published again when the upload finishes

### Requirement: The update command explains a release it cannot download from

When no archive is published for the running platform, the update command SHALL report that as a fact about the
release and the platform. It SHALL NOT report the download URL or the HTTP status, which describe the request
rather than the situation and read as a fault on the machine that ran the command.

#### Scenario: A release with no archive attached yet

- **WHEN** an update is attempted against a release that publishes no archive for this platform
- **THEN** the message names the release and the platform, and says both that the archives may still be
  uploading and that the platform may not be one the release builds

#### Scenario: No URL or status code reaches the reader

- **WHEN** that message is shown
- **THEN** it contains no download URL and no HTTP status code

#### Scenario: Nothing published to update to

- **WHEN** an update is attempted and no release is published at all
- **THEN** the message says there is no published release to update to, rather than reporting a lookup failure
