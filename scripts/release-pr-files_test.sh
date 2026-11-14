#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Guards scripts/release-pr-files.sh, the check that lets CI skip the suite on
# release-please's pull request. A guard that passes everything would let code
# merge untested, so most cases here are ones it must refuse. The last cases
# run the real `git diff` the just recipe runs, in a scratch repository, so a
# rename cannot hide a code path behind a release file's name.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
guard=${1:-$here/release-pr-files.sh}
fails=0

expect() {
  local want=$1 name=$2 input=$3 got=pass
  printf '%b' "$input" | "$guard" >/dev/null 2>&1 || got=fail
  if [ "$got" != "$want" ]; then
    echo "FAIL: $name: want $want, got $got" >&2
    fails=$((fails + 1))
  fi
}

expect pass "a release" 'CHANGELOG.md\n.release-please-manifest.json\n'
expect pass "the manifest alone" '.release-please-manifest.json\n'
expect pass "no trailing newline" 'CHANGELOG.md\n.release-please-manifest.json'
expect fail "nothing changed" ''
expect fail "only blank lines" '\n\n'
expect fail "a Go file" 'CHANGELOG.md\n.release-please-manifest.json\ncmd/root.go\n'
expect fail "a workflow" 'CHANGELOG.md\n.github/workflows/ci.yaml\n'
expect fail "the release-please config" 'CHANGELOG.md\nrelease-please-config.json\n'
expect fail "a changelog in a subdirectory" 'docs/CHANGELOG.md\n'
expect fail "a name with the changelog as a prefix" 'CHANGELOG.md.bak\n'
expect fail "a name with the changelog as a suffix" 'xCHANGELOG.md\n'
expect fail "a different case" 'changelog.md\n'
expect fail "a quoted path" '"CHANGELOG.md\\n.go"\n'
expect fail "the release file on a line with another" 'CHANGELOG.md cmd/root.go\n'

repo=$(mktemp -d)
trap 'rm -rf "$repo"' EXIT
git -C "$repo" init -q
git -C "$repo" config user.name test
git -C "$repo" config user.email test@example.invalid
git -C "$repo" config commit.gpgsign false
printf '# Changelog\n' >"$repo/CHANGELOG.md"
printf '{".": "1.0.0"}\n' >"$repo/.release-please-manifest.json"
printf 'package main\n' >"$repo/main.go"
git -C "$repo" add -A
git -C "$repo" commit -qm base

diffed() {
  local want=$1 name=$2 got=pass
  git -C "$repo" add -A
  git -C "$repo" commit -qm "$name"
  git -C "$repo" diff --no-renames --name-only HEAD^1 HEAD | "$guard" >/dev/null 2>&1 || got=fail
  if [ "$got" != "$want" ]; then
    echo "FAIL: $name: want $want, got $got" >&2
    fails=$((fails + 1))
  fi
}

printf '## 1.0.1\n' >>"$repo/CHANGELOG.md"
printf '{".": "1.0.1"}\n' >"$repo/.release-please-manifest.json"
diffed pass "git: a release commit"

git -C "$repo" mv main.go CHANGELOG.go
diffed fail "git: a code file renamed"

git -C "$repo" rm -q CHANGELOG.md
git -C "$repo" mv CHANGELOG.go CHANGELOG.md
diffed fail "git: code renamed over the changelog"

if [ "$fails" -ne 0 ]; then
  echo "$fails case(s) failed" >&2
  exit 1
fi
echo "release-pr-files: all cases passed"
