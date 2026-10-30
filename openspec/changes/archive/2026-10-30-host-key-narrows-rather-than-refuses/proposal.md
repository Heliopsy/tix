# Host key narrows rather than refuses

## Why

The SSH listener refuses to start when its host key is readable by anyone but
its owner. The intent is right: a readable host key lets somebody else
impersonate the listener. The consequence is not, because the usual way to
reach that state is not an operator being careless.

Kubernetes defaults `fsGroupChangePolicy` to `Always`, which recursively adds
group-rw to every file on the volume at mount time. The 0600 that `tix` writes
the host key with therefore comes back as 0660 on the next mount, and the
listener will not start.

The shape of the failure is the worst part. The install works and the first boot
works, because `tix` creates the key itself at 0600 and nothing has remounted
the volume yet. The pod dies at its first restart, which is usually a rollout,
so it reads as the upgrade having broken a working deployment. It strands anyone
following the documented Kubernetes path with SSH enabled and a persisted host
key, which is the combination the documentation prescribes.

## What changes

- The listener narrows a host key wider than its owner back to 0600 and carries
  on, rather than refusing to start. It logs a warning naming the mode it found.
- It still refuses a key whose mode it cannot fix, because a chmod this process
  cannot perform means the file is not ours, and it reads the mode back after
  the chmod rather than trusting it.
- `docs/deployment.md` records the `fsGroup` interaction and names
  `fsGroupChangePolicy: OnRootMismatch` as the way to avoid the rewrite.

## Impact

Affected specs: `ssh-access`.

The refusal protected nothing the narrowing does not. Whoever could read the key
while it was wide has already read it, and refusing to start neither unreads it
nor rotates it. What the refusal did instead was convert a state the process can
repair into a listener that will not run.
