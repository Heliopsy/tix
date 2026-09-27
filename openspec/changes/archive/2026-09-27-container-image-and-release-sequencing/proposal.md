# A container image people can pull, and a release that is never empty

## Why

Two halves of the same problem: what a release publishes, and when it says it has published it.

**There is no image.** `Containerfile` builds a good runtime image, `just image` builds it locally, and nothing
has ever been pushed anywhere. Anyone who wants to run tix under Kubernetes, podman or Docker has to build it
themselves. The project's own plan listed "container image build + Trivy image scan" as a CI gate and it was
never implemented, so the image was also never a gate: nothing ran it.

Which mattered, because the image did not work. `tix serve` takes its listen address from its flag and
defaults to `127.0.0.1:8080`, and a loopback bind inside a container's network namespace reaches nobody. The
image declared `EXPOSE 8080`, started, logged `tix listening on 127.0.0.1:8080`, and reset every connection to
the published port. Nothing in the repository would have noticed: the build succeeded, and a successful build
was the whole test.

**A release exists before its binaries do.** release-please creates the GitHub release; the binary build runs
afterwards and uploads to it. Between those two moments the release is the one `/releases/latest` resolves, with
zero assets, so `curl https://tix.red/install.sh | sh` and `tix update` both fail on a 404 for an archive URL.
This has bitten twice. Once for ten hours, when release-please created v0.4.0 and then died posting a comment
to a locked pull request, so the build was never asked for; a self-heal step now catches that, and it worked
for v0.5.0. But the ordinary window remained, and v0.5.0 was uninstallable for about four minutes.

The two are one change because the fix to the second is a sequencing property of the release, and the image
becomes part of what that release publishes.

## What Changes

- **A multi-architecture image is published to GHCR**, `linux/amd64` and `linux/arm64`, tagged with the exact
  version, the minor series, and `latest`. No bare major tag: at 0.x a minor bump may break, so `:0` would walk
  somebody across exactly the break the version scheme exists to announce.
- **The image is signed and attested, not merely built.** A cosign signature covering the manifest list and both
  architecture manifests, an SPDX SBOM attestation per architecture, and SLSA build provenance, which the
  release archives do not have.
- **The image is a gate, and the gate runs the container.** It builds both architectures, starts the native one
  and asks it for `/healthz`, `/readyz`, the web UI and its stylesheet, reads the uid of the running server, and
  proves a database on `/data` outlives the container that wrote it. Then Trivy scans the layers rather than the
  source tree. All through `just`, so `just ci` and CI cannot drift.
- **The image actually serves.** The listen address moves onto the command line, because `serve` reads it from
  its flag and never from the resolved configuration, so `TIX_SERVER_LISTEN` sets a value nothing consults.
- **A release is a draft until its binaries are attached.** release-please creates the release as a draft, the
  workflow creates the tag a draft does not create, and GoReleaser uploads into that draft and publishes it as
  its own last step. The release becomes `latest` and gains its archives in one move.
- **The self-heal step still works, against drafts.** It asked GitHub for the latest release, which is exactly
  what hides a draft, so it would have reported everything fine while a new release sat unbuilt. It now asks for
  the release the manifest names, and covers the missing tag as well as the missing build.
- **`tix update` says what is wrong.** A missing archive was reported as the URL that answered 404, which reads
  as a broken machine. It now names the release and both reasons it happens.

## Impact

- `Containerfile`: cross-compiling builder stage, OCI source labels, a command line that binds a reachable
  address.
- `justfile`: `image-multiarch`, `image-verify`, `image-scan`, `image-tags`, `image-tags-check`, `image-push`,
  and `image-gate` wiring them into `ci`.
- `.github/workflows/`: an image job in CI, an image job in Release, and a rewritten self-heal.
- `.goreleaser.yaml`: `use_existing_draft`, so the upload reaches release-please's draft instead of creating a
  second release for the same tag.
- `release-please-config.json`: `draft: true`.
- `internal/selfupdate`: a sentinel for a missing asset, and two sentences that use it.
- No change to the service layer, the store, or any access path.
