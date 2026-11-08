# Upgrading

```sh
tix update            # replace this binary with the newest release
tix update --check    # is there a newer one? write nothing
tix update --version v0.2.0   # install a named release, including an older one
```

## What it does

Looks up the release, downloads the archive built for this operating system and architecture, checks it
against the checksum published beside it, and replaces this binary with the one inside.

The replacement is written next to the binary and then moved over it. A rename within one directory is
atomic, so an interrupted download or a full disk cannot leave a truncated file where tix used to be. That
failure mode, a machine with a `tix` on `PATH` that no longer runs, is the reason it is not written in
place.

## What the checksum proves, and what it does not

**It proves** the bytes that arrived are the bytes the release published. That catches a truncated
download, a corrupted one, and a mirror or proxy serving something else.

**It does not prove** the release is the one the maintainers built. The archive and `checksums.txt` come
from the same host, so anyone able to change one could change the other. The real trust anchor is TLS to
that host.

Releases are also signed with cosign, and those signatures are the stronger guarantee. `tix update` does
not check them, because verifying a sigstore bundle needs a verifier this binary does not carry. If you
need that assurance, verify the signature yourself:

```sh
cosign verify-blob --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/Heliopsy/tix/.github/workflows/release.yaml@refs/heads/main' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

The signing is keyless, so there is no key to fetch: the identity flags are what you are checking against, and
cosign refuses to verify without them rather than telling you a signature is fine when you never said whose you
expected. The container image is signed by the same workflow, and
[deployment.md](deployment.md#verifying-the-image) explains each part of that identity, including why the ref is
a branch and not the tag.

So when the output says "checksum verified", it means the download matches the release and nothing more.

## What it refuses, and why

`tix update` replaces only a binary it installed or that came from a release archive.

| Situation | What happens |
| --- | --- |
| Installed with `install.sh`, or unpacked by hand | Replaced in place |
| Installed with `go install` | Refused; it names the `go install` command to re-run |
| Under a package manager's prefix (nix, Homebrew Cellar, snap, flatpak, `/usr/bin`) | Refused; upgrade it the way you installed it |
| The directory cannot be written | Refused **before** downloading anything |

Overwriting a file another tool owns leaves that tool's database describing something that is no longer
there, and the next upgrade from that tool silently reverts you.

The unwritable case is checked first on purpose. Finding out after the download wastes it, and reports a
permission error at the exact moment the binary is being replaced, which reads like the replacement half
happened.

## Going back to an older release

`tix update --version` installs an older release as readily as a newer one, but a database that a newer
release migrated is not migrated back: no migration here has a down step. Most of the time the older binary
simply ignores the columns it does not know. One case needs care.

**After one person belongs to several tenants.** The release that let one account join several tenants
linked every actor to its account (`actors.user_id`), and existing rows kept their identifiers, so for every
person who existed before it the actor and the account share one. Rolling back is safe while that is still
true of everybody. Once a global admin adds somebody to a second tenant, or an invite to an existing account
is accepted, that person has an actor whose identifier differs from their account's. An older binary finds a
person only through the actor sharing their account's identifier, so after a rollback they can sign in only
to the tenant they were in first, and the other memberships, with their tokens and SSH keys, stop working
until the newer release is back. Nothing is deleted. Before rolling back, check with
`tix global-admin account-log EMAIL` or `GET /api/v1/whoami` whether anybody holds more than one membership.

## Restarting a server

`tix update` does not restart anything. If a `tix serve` is running from the binary you just replaced, it
keeps running the old code until you restart it yourself.

This is deliberate. A tracker that restarts itself because it noticed an open file handle is worse than one
that leaves the decision to whoever knows whether anybody is mid-transition.

## If you would rather not

Nothing here is required. `install.sh` re-run over an existing install does the same job, and every release
is downloadable by hand:

```sh
curl -fsSL https://tix.red/install.sh | sh
```

## See also

- [deployment.md](deployment.md) for running a server
- [statistics.md](statistics.md) and [theming.md](theming.md) for what the newer releases added
