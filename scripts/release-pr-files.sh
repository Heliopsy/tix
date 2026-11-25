#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Reads the paths a pull request changes, one per line, and fails unless every
# one is a file release-please writes. CI skips the test suite on
# release-please's branch on the strength of this check, so a code change
# pushed to that branch has to fail here or it would merge untested.
#
# The release type is "go", which writes the changelog and the manifest and
# nothing else (#234, #242, #253). A path git had to quote, an empty list and
# anything not named below all fail.
set -euo pipefail

allowed='^(CHANGELOG\.md|\.release-please-manifest\.json)$'

n=0
bad=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  n=$((n + 1))
  if ! grep -qE "$allowed" <<<"$f"; then
    echo "not a release file: $f" >&2
    bad=1
  fi
done

if [ "$n" -eq 0 ]; then
  echo "no changed files read; a release pull request changes at least the manifest" >&2
  exit 1
fi
if [ "$bad" -ne 0 ]; then
  echo "a release-please pull request may change only CHANGELOG.md and .release-please-manifest.json," >&2
  echo "because CI skips the test suite for it; land other changes through an ordinary pull request" >&2
  exit 1
fi
echo "release files only ($n changed)"
