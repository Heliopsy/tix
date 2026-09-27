# Design

## How the multi-architecture image is built

Three options were on the table.

**ko** was rejected. It builds an image from Go source and ignores the Containerfile entirely, so the distroless
nonroot base, the numeric `USER`, `EXPOSE`, the `/data` volume and the default DSN would all have to be
re-expressed in ko configuration, and `just image` would stop producing the image that ships.

**GoReleaser `dockers_v2`** was rejected, reluctantly, because it fits the existing toolchain best. It builds
each image from a temporary context containing only the artifacts GoReleaser produced, so a Containerfile with
`COPY . .` cannot see the repository. Using it would mean a second Containerfile that copies a prebuilt binary,
and two Containerfiles for one image is exactly the drift this project keeps paying for. It also needs buildx,
and a multi-platform buildx build cannot be loaded into a local engine, so the image could not be run and tested
without a registry. The requirement is that the gate runs locally.

**Engine-native per-platform builds plus a manifest list** is what was implemented. One Containerfile, and the
same commands locally and in CI:

    podman build --platform linux/amd64 -t localhost/tix:latest-amd64 .
    podman build --platform linux/arm64 -t localhost/tix:latest-arm64 .
    podman manifest create localhost/tix:multiarch
    podman manifest add    localhost/tix:multiarch containers-storage:localhost/tix:latest-amd64
    podman manifest add    localhost/tix:multiarch containers-storage:localhost/tix:latest-arm64

No QEMU, because the builder stage is pinned to the build host with `FROM --platform=${BUILDPLATFORM}` and Go
cross-compiles to `TARGETARCH`. The binary is already `CGO_ENABLED=0`, so this is what the toolchain does
anyway; compiling inside an emulated arm64 image would turn forty seconds into ten minutes for an identical
result. No registry either: the manifest list is assembled from local storage, so the whole thing is verifiable
on a laptop.

The cost is podman. `docker manifest create` resolves its arguments in a registry and cannot see a locally built
image, so the manifest recipes refuse under `CONTAINER_ENGINE=docker` with a message saying so. podman is
already this project's default engine, and the Release and CI jobs set it explicitly.

GitHub's `ubuntu-24.04` runner image ships Podman 4.9.3, which is the version this was verified against, so the
runner and a developer's machine are running the same engine rather than two that happen to agree.

### Why the architecture is read from the ELF header

`podman manifest inspect` reports each entry's platform, but that metadata comes straight from `--platform`. An
image built with `GOARCH` lost carries an amd64 binary and still labels itself arm64, and a manifest list over
two such images still has two entries with the right platforms. So the gate extracts the binary from each image
and reads `e_machine` out of the ELF header: 62 for x86-64, 183 for aarch64. The manifest check stays as well,
because it catches the different failure of a list that is missing an entry.

## Tag policy

Published: the exact version (`0.5.0`), the minor series (`0.5`), and `latest`.

Not published: a bare major (`0`). release-please is configured `bump-minor-pre-major`, which means 0.5 to 0.6 is
where a breaking change lands. A `:0` tag would carry somebody across that without their having typed a version,
which is the one thing SemVer's 0.x clause exists to prevent. When the project reaches 1.0 the argument
reverses and `1` becomes the right series tag; `image-tags-check` is where that decision will have to be
changed, and it will fail until it is.

`latest` is published because it is what somebody trying the project types, and it is the one tag everybody
already understands to move under them.

`image-tags` refuses anything that is not a three-part numeric version. `git describe` on an untagged commit
yields `0.5.0-2-gabc1234`, and publishing that plus `latest` from a working tree is worse than failing.

## Signing and provenance

The archives carry cosign bundles over `checksums.txt` and syft SBOMs. The image should not be weaker, so:

- **cosign sign --recursive** over the manifest list. Recursive because somebody pulling on arm64 receives the
  child manifest, and a signature over the list alone is not one they can check against what they are running.
- **One SPDX SBOM per architecture**, each attested to that architecture's own digest. A single SBOM attached to
  the list would describe one of the two images while being signed as though it described both.
- **SLSA build provenance** via `actions/attest-build-provenance`, pushed to the registry. The archives do not
  have this. It records the workflow, the commit and the runner that produced the digest, so a consumer can ask
  where the bytes came from rather than only who signed them.

## Release sequencing

GoReleaser already does the right thing internally. Reading v2.18.2's GitHub client: `CreateRelease` sends
`Draft: true` with the comment "Always start with a draft release while uploading artifacts. PublishRelease will
undraft it." So a GoReleaser-owned release is never public without its assets.

The problem is that release-please gets there first and publishes. And GoReleaser does not override that:
`createOrUpdateRelease` sets `data.Draft = new(release.Draft)` when it finds an existing release, preserving
whatever state it is in. So today's release stays public and empty for the whole build.

Setting `release.draft: true` in `.goreleaser.yaml` is the wrong lever. `PublishRelease` returns early when it
is set, so every release would stay a draft forever. The correct change is on the other side:

1. **release-please creates a draft** (`"draft": true`). A draft is invisible to `/releases/latest`, which is
   what `install.sh` and `tix update` resolve, so nothing points at it.
2. **The workflow creates the tag.** GitHub only writes the tag ref when a release is published, so a draft has
   no tag and there would be nothing to check out or to `git describe`. The commit comes from the draft's own
   `targetCommitish` rather than from the pushing head.
3. **`use_existing_draft: true`** in `.goreleaser.yaml`. Without it, `findRelease` calls
   `GET /releases/tags/{tag}`, which returns 404 for a draft, so GoReleaser would create a *second* release for
   the same tag, upload into that, and leave release-please's draft empty forever. With it, `findDraftRelease`
   matches the draft by release name: release-please names the release `v0.5.0` and GoReleaser's default
   `name_template` is `{{.Tag}}`, so they agree.
4. **GoReleaser publishes it** as its own last step, with `make_latest: "true"` stated rather than assumed.

`replace_existing_draft` is deliberately left off: it deletes the draft it finds, which would throw away the
notes release-please wrote.

The failure mode improves as well as the happy path. Under the old arrangement, anything that went wrong after
release-please left a public release with nothing to download. Now it leaves a draft, which nobody resolves, and
the self-heal fixes it on the next push.

### The self-heal

It asked for the latest release and dispatched a build if that had no assets. Under drafts that stops working
entirely: the latest release would be the *previous* one, which has its archives, so the step would report
everything fine while the new release sat unbuilt. Rewritten, it asks for the release `.release-please-manifest.json`
names, and then:

- not a draft: published with its assets, nothing to do.
- a release run in flight: leave it, a second dispatch would race the first to the same asset names.
- no tag: create it from `targetCommitish`, because the tag step may be what died.
- otherwise: dispatch.

## What the verification gate asserts, and why each assertion is narrow

Every one of these was chosen so that it names one thing and fails when that thing breaks:

- `/healthz` must report `status: ok` **and the version this build stamped**. The version is what makes the check
  unfakeable by a stale image or by something else already listening on the port.
- `/readyz` must report its **named** `database` and `migrations` checks as true, not only the top-level flag.
- The web UI must render the sign-in template **by title**. A 200 carrying an error page satisfies a check that
  only looks for an HTML tag.
- `/assets/app.css` must be over a kilobyte. The assets are `go:embed`ed, so losing them is a container-specific
  failure nothing else here would see.
- The uid is read from the **running process**, selecting the `tix` row. Reading the image's declared `USER`
  would pass on an image whose entrypoint dropped back to root, and scanning the whole `podman top` output would
  find the 0 belonging to podman's own `ps`.
- Persistence destroys and **replaces** the container rather than restarting it. A restart keeps the writable
  layer, so a row would survive it wherever the database sat, and the check would pass on an image whose `/data`
  mount did nothing.

## Trade-offs accepted

- **`just ci` gets slower.** Two Go builds plus a container run and a Trivy scan. It stays out of `just check`,
  the fast pre-push set.
- **The image ships with the bind-safety opt-out in its command.** `serve` refuses a non-loopback address
  without a certificate and that opt-out is a flag, not a configuration key. Terminating TLS in front of a
  container is the ingress's job. The alternative is an image that cannot serve, which is what there was.
- **The image job in Release runs after the release is published.** A registry refusing pushes should not hold
  back the archives, which is the failure people actually feel. The same gate has already run in CI on the
  commit.
