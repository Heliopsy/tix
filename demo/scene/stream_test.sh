#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Guards the redaction in stream.jq. A lint gate cannot see this one: the
# filter parses fine while it prints a credential, which is how the narration
# path went unredacted from the day the filter was written.
#
# Takes the filter to test as an argument so the guard can be pointed at an
# older revision and watched to fail:
#
#   git show <rev>:demo/scene/stream.jq > /tmp/old.jq
#   demo/scene/stream_test.sh /tmp/old.jq
set -euo pipefail

FILTER="${1:-$(dirname "$0")/stream.jq}"
SECRET="lt_s3cr3tvalue"
fails=0

delta() { printf '{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":%s}}}' "$(printf '%s' "$1" | jq -Rs .)"; }
tool_use() { printf '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":%s}}]}}' "$(printf '%s' "$1" | jq -Rs .)"; }

render() { jq -j -f "$FILTER"; }

# want_redacted NAME EVENT: the secret must not reach the pane.
want_redacted() {
  local name="$1" got
  got=$(printf '%s\n' "$2" | render)
  if printf '%s' "$got" | grep -qF "$SECRET"; then
    printf 'FAIL %s: secret in the output: %s\n' "$name" "$got" >&2
    fails=$((fails + 1))
  elif ! printf '%s' "$got" | grep -qF '********'; then
    printf 'FAIL %s: no redaction marker: %s\n' "$name" "$got" >&2
    fails=$((fails + 1))
  else
    printf 'ok   %s\n' "$name"
  fi
}

want_redacted "narration, --lease-token" "$(delta "claiming it with --lease-token $SECRET now")"
want_redacted "narration, --token" "$(delta "releasing with --token $SECRET and moving on")"
want_redacted "narration, --lease-token=" "$(delta "ran --lease-token=$SECRET there")"
want_redacted "narration, token at end of line" "$(delta "ran --token $SECRET
and carried on")"
want_redacted "tool call, --lease-token" "$(tool_use "tix task mv T-1 doing --lease-token $SECRET")"
want_redacted "tool call, --token" "$(tool_use "tix claim release T-1 --token $SECRET --status done")"

# Narration is a delta stream: the pane is fed a few characters at a time and
# truncating one cuts a word in half. Only the tool call arm is shortened.
prose="I picked the obvious reading and wrote a small in-memory store with a tiny API, which is more than ninety six characters of perfectly ordinary narration."
got=$(printf '%s\n' "$(delta "$prose")" | render)
if [ "$got" = "$prose" ]; then
  printf 'ok   narration passes through whole\n'
else
  printf 'FAIL narration was altered: %s\n' "$got" >&2
  fails=$((fails + 1))
fi

got=$(printf '%s\n' "$(tool_use "tix task add $(printf 'x%.0s' $(seq 1 120))")" | render)
if printf '%s' "$got" | grep -qF ' ...'; then
  printf 'ok   tool call is truncated\n'
else
  printf 'FAIL tool call was not truncated: %s\n' "$got" >&2
  fails=$((fails + 1))
fi

if [ "$fails" -ne 0 ]; then
  printf '\n%s check(s) failed against %s\n' "$fails" "$FILTER" >&2
  exit 1
fi
printf '\nall checks passed against %s\n' "$FILTER"
