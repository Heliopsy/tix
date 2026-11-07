#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Splits the Go packages into the shards CI runs side by side.
#
#   test-shards.sh names          the shard names, one per line
#   test-shards.sh list SHARD     the packages SHARD runs, one per line
#   test-shards.sh check [FILE]   fails unless every package is in exactly one
#                                 shard, and FILE (the CI workflow) runs every
#                                 shard by name
#
# A shard is a list of package patterns, anchored at the module path. The last
# shard, rest, takes whatever no named shard claims, so a package added next
# month runs somewhere without anybody editing this file. The check still
# counts every package against every shard, because two named shards claiming
# one package run it twice and a rest that misses one runs it nowhere.
set -euo pipefail

module="github.com/heliopsy/tix"

shards=(service cmd web api store rest)

# The four heaviest packages get a runner each; the store shard holds the
# PostgreSQL engine and everything that dials it.
patterns() {
	case "$1" in
	service) echo "^$module/internal/service\$" ;;
	cmd) echo "^$module/cmd\$" ;;
	web) echo "^$module/internal/web\$" ;;
	api) echo "^$module/internal/(httpapi|integration|demo)\$" ;;
	store) echo "^$module/internal/(store(/.*)?|connect|bench)\$" ;;
	rest) echo "" ;;
	*)
		echo "unknown shard: $1 (have: ${shards[*]})" >&2
		return 1
		;;
	esac
}

packages() { go list ./...; }

# claimed prints the packages a named shard's patterns match.
claimed() {
	local re
	re=$(patterns "$1")
	grep -E "$re" <<<"$2" || true
}

list() {
	local shard=$1 all named=""
	all=$(packages)
	patterns "$shard" >/dev/null
	if [ "$shard" != rest ]; then
		claimed "$shard" "$all"
		return
	fi
	for s in "${shards[@]}"; do
		[ "$s" = rest ] && continue
		named+=$(claimed "$s" "$all")$'\n'
	done
	grep -vxF -f <(grep -v '^$' <<<"$named") <<<"$all" || true
}

check() {
	local workflow=${1:-} all assigned s bad=0
	all=$(packages)
	assigned=""
	for s in "${shards[@]}"; do
		local got
		got=$(list "$s")
		if [ -z "$got" ]; then
			echo "shard $s runs no package" >&2
			bad=1
		fi
		assigned+="$got"$'\n'
	done
	while read -r pkg; do
		[ -n "$pkg" ] || continue
		local n
		n=$(grep -cxF "$pkg" <<<"$assigned" || true)
		if [ "$n" -ne 1 ]; then
			echo "$pkg is in $n shards, want exactly 1" >&2
			bad=1
		fi
	done <<<"$all"
	if [ -n "$workflow" ]; then
		# The matrix is the one line that says `shard: [...]`.
		local matrix
		matrix=$(sed -n 's/^ *shard: *\[\(.*\)\] *$/\1/p' "$workflow" | tr -d ' ' | tr ',' '\n' | sort)
		if [ -z "$matrix" ]; then
			echo "$workflow has no 'shard: [...]' matrix line" >&2
			bad=1
		elif [ "$matrix" != "$(printf '%s\n' "${shards[@]}" | sort)" ]; then
			echo "$workflow runs shards [$(echo "$matrix" | paste -sd, -)], want [$(printf '%s\n' "${shards[@]}" | sort | paste -sd, -)]" >&2
			bad=1
		fi
	fi
	[ "$bad" -eq 0 ] || exit 1
	echo "every one of $(grep -c . <<<"$all") packages is in exactly one of ${#shards[@]} shards"
}

case "${1:-}" in
names) printf '%s\n' "${shards[@]}" ;;
list) list "${2:?usage: test-shards.sh list SHARD}" ;;
check) check "${2:-}" ;;
*)
	echo "usage: test-shards.sh names | list SHARD | check [WORKFLOW]" >&2
	exit 2
	;;
esac
