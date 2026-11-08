#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# .github/actions/fetch-release-asset.sh REPO TAG ASSET SHA256 DEST
#
# Downloads one GitHub release asset to DEST and refuses it unless its sha256
# is the pinned one. CI installs its tools with this rather than a bare curl.
#
# github.com/OWNER/REPO/releases/download answered 504 for hours on end while
# the status page stayed green, and setup-just and the toolbox gave up after
# two or three tries. The same bytes stayed reachable through the REST API's
# asset endpoint, so attempts alternate between the two, with a backoff. The
# API is asked with GH_TOKEN when the caller passes one, since anonymous
# requests share a runner IP's 60-an-hour limit, and anonymously after that.
#
# The checksum is what makes the second source safe to fall back on: whichever
# one answers, the file is the pinned bytes or nothing.
set -euo pipefail

if [ $# -ne 5 ]; then
  echo "usage: $0 REPO TAG ASSET SHA256 DEST" >&2
  exit 2
fi
repo=$1 tag=$2 asset=$3 want=$4 dest=$5
attempts=${FETCH_ATTEMPTS:-8}

sha256() {
  if command -v sha256sum >/dev/null; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

curl_get() {
  curl --fail --silent --show-error --location \
    --connect-timeout 20 --max-time 300 "$@"
}

from_download() {
  curl_get -o "$1" "https://github.com/${repo}/releases/download/${tag}/${asset}"
}

# api_get ACCEPT OUT URL, with GH_TOKEN first and then without. A job's
# GITHUB_TOKEN was refused with a 403 for aquasecurity/trivy while it read
# casey/just fine, and the anonymous request is allowed where it is not.
api_get() {
  local accept=$1 out=$2 url=$3
  if [ -n "${GH_TOKEN:-}" ] &&
     curl_get -H "Authorization: Bearer ${GH_TOKEN}" -H "Accept: ${accept}" -o "$out" "$url"; then
    return 0
  fi
  curl_get -H "Accept: ${accept}" -o "$out" "$url"
}

from_api() {
  local id release="${1}.release.json"
  api_get application/vnd.github+json "$release" \
    "https://api.github.com/repos/${repo}/releases/tags/${tag}" || return 1
  id=$(jq -r --arg n "$asset" '.assets[] | select(.name == $n) | .id' "$release")
  rm -f "$release"
  if [ -z "$id" ]; then
    echo "release ${tag} of ${repo} lists no asset named ${asset}" >&2
    return 1
  fi
  api_get application/octet-stream "$1" \
    "https://api.github.com/repos/${repo}/releases/assets/${id}"
}

mkdir -p "$(dirname "$dest")"
tmp="${dest}.part"
delay=2
for ((i = 1; i <= attempts; i++)); do
  rm -f "$tmp"
  if ((i % 2)); then
    source=download
    from_download "$tmp" && ok=1 || ok=0
  else
    source=api
    from_api "$tmp" && ok=1 || ok=0
  fi
  if ((ok)); then
    got=$(sha256 "$tmp")
    if [ "$got" = "$want" ]; then
      mv "$tmp" "$dest"
      echo "fetched ${asset} ${tag} (${source}, attempt ${i})"
      exit 0
    fi
    echo "${asset}: sha256 ${got}, expected ${want}" >&2
  fi
  if ((i < attempts)); then
    echo "attempt ${i} of ${attempts} for ${asset} failed; next in ${delay}s" >&2
    sleep "$delay"
    delay=$((delay * 2 > 60 ? 60 : delay * 2))
  fi
done
rm -f "$tmp"
echo "could not fetch ${asset} ${tag} from ${repo} in ${attempts} attempts" >&2
exit 1
