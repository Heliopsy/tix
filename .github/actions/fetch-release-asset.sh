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
# asset endpoint, so attempts alternate between the two, with a backoff, and
# the API is asked with GH_TOKEN when the caller passes one (unauthenticated
# requests share a runner IP's 60-an-hour limit).
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

auth=()
if [ -n "${GH_TOKEN:-}" ]; then
  auth=(-H "Authorization: Bearer ${GH_TOKEN}")
fi

curl_get() {
  curl --fail --silent --show-error --location \
    --connect-timeout 20 --max-time 300 "$@"
}

from_download() {
  curl_get -o "$1" "https://github.com/${repo}/releases/download/${tag}/${asset}"
}

from_api() {
  local id
  id=$(curl_get "${auth[@]}" -H 'Accept: application/vnd.github+json' \
         "https://api.github.com/repos/${repo}/releases/tags/${tag}" \
       | jq -r --arg n "$asset" '.assets[] | select(.name == $n) | .id')
  if [ -z "$id" ]; then
    echo "release ${tag} of ${repo} lists no asset named ${asset}" >&2
    return 1
  fi
  curl_get "${auth[@]}" -H 'Accept: application/octet-stream' -o "$1" \
    "https://api.github.com/repos/${repo}/releases/assets/${id}"
}

mkdir -p "$(dirname "$dest")"
tmp="${dest}.part"
delay=2
for ((i = 1; i <= attempts; i++)); do
  if ((i % 2)); then source=from_download; else source=from_api; fi
  rm -f "$tmp"
  if "$source" "$tmp"; then
    got=$(sha256 "$tmp")
    if [ "$got" = "$want" ]; then
      mv "$tmp" "$dest"
      echo "fetched ${asset} ${tag} (${source#from_}, attempt ${i})"
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
