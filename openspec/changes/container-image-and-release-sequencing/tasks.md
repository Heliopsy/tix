# Tasks

## Containerfile

- [x] Pin the builder stage to `${BUILDPLATFORM}` and cross-compile to `${TARGETARCH}`, so neither architecture
      needs emulation
- [x] Bind a reachable address in the command, because `serve` reads its listen address from its flag and never
      from the resolved configuration
- [x] Add the OCI source, licence, version, revision and created labels, so a registry package links back to the
      repository
- [x] Keep hadolint green, with the one `--platform` exemption carrying its reason

## justfile

- [x] `_image-build PLATFORM TAG`: one build path for every image this project produces
- [x] `image-multiarch`: both architectures, the ELF machine type of each binary, and a manifest list over them
- [x] `image-verify`: run the container and assert `/healthz` with this build's version, `/readyz` by named
      check, the sign-in template, the embedded stylesheet, the running process's uid, and a database on `/data`
      surviving a replacement container
- [x] `image-scan`: Trivy against the saved image rather than the source tree
- [x] `image-tags VERSION` and `image-tags-check`: the tag policy, asserted
- [x] `image-push REPO VERSION`: the only recipe that touches a registry, writing the digest for signing
- [x] `image-gate` aggregating them, wired into `ci`
- [x] Gitignore the image tarball and the digest file

## Workflows

- [x] CI: an image job running `just image-gate`
- [x] Release: an image job that runs the gate, pushes the manifest list, signs it recursively, attests an SBOM
      per architecture and pushes build provenance
- [x] Release: every added action pinned to a full commit SHA with its version in a comment
- [x] Release: hold an already-published release as a draft before rebuilding it, because
      `use_existing_draft` looks at drafts only and would otherwise fail a rebuild with a bare 422
- [x] release-please: create the tag a draft release does not create, from the draft's own target commit
- [x] release-please: rewrite the self-heal to ask for the release the manifest names, handle drafts, repair a
      missing tag, and not duplicate a build already in flight

## Release configuration

- [x] `release-please-config.json`: create the release as a draft
- [x] `.goreleaser.yaml`: `use_existing_draft` so the upload reaches that draft instead of creating a second
      release for the same tag, and `make_latest` stated

## internal/selfupdate

- [x] A sentinel for an asset the host does not have
- [x] Report a missing archive as a fact about the release and the platform, with no URL and no status code
- [x] Report an absent latest release as nothing published to update to
- [x] Tests for both sentences, asserting the absence of the URL and the status separately from the presence of
      the explanation

## Verification

- [x] Both architectures build, and each binary's ELF machine type is checked
- [x] The running container answers `/healthz`, `/readyz`, the web UI and its stylesheet
- [x] The server process runs as uid 65532
- [x] A task written to `/data` survives a replacement container
- [x] Trivy scans the image clean, with the two accepted findings' reachability re-verified against the current
      dependency graph
- [x] Every guard added here mutated, watched to fail with its own message, and restored by checksum

## Left for the repository owner

- [ ] Publish: nothing here has been pushed, tagged or released
- [ ] `docs/` and `README.md`: how to pull and run the image, the tag policy, how to verify the signature and the
      attestations
- [ ] `cmd/serve.go` reads its listen address from the flag and ignores `server.listen`, so the configuration key
      and its environment variable do nothing. Out of this change's scope; the image works around it on the
      command line
