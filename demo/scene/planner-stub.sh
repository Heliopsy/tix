#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# A stand-in for the planner, so the layout can be rehearsed without paying
# for a Claude run. It writes the same shape of backlog the real planner does:
# subtasks under the epic, each with a priority and a tag, and the epic moved
# along at the end.
#
# This is a debugging tool and is never what gets filmed for publication. The
# whole point of the recording is that the agent is real. It exists because
# framing, pane widths and colours take several passes to get right, and none
# of those passes need an agent to be the thing writing the rows.

set -uo pipefail

AGENT=${TIX_AGENT:?an agent name is required}
PROJECT=${SCENE_PROJECT:-links}
EPIC=${SCENE_EPIC:?the ref of the task to break down is required}
TASKS=${SCENE_TASKS:-6}

export TIX_SERVER_URL="http://127.0.0.1:8080"
TIX_TOKEN=$(jq -r '.secret // .token // empty' "/home/demo/.$AGENT.token.json" 2>/dev/null)
export TIX_TOKEN
export TIX_OUTPUT_COLOR=always

clear
printf '  %s  breaking the work down  (rehearsal stand-in)\n\n' "$AGENT"

titles=(
  "Slug generation and link storage layer"
  "POST /links handler to create a short link"
  "GET /{slug} redirect handler"
  "Actionable JSON errors for an unknown slug"
  "GET /health check endpoint"
  "API documentation for callers"
  "Expiry sweep for stale links"
)
prios=(high high high normal normal low low)
tags=(storage api api errors health docs storage)

for i in $(seq 0 $((TASKS - 1))); do
  [ "$i" -lt "${#titles[@]}" ] || break
  sleep 2
  printf '  %s\n' "${titles[$i]}"
  tix task add "${titles[$i]}" -p "$PROJECT" --parent "$EPIC" \
    --priority "${prios[$i]}" --tag "${tags[$i]}" >/dev/null 2>&1 \
    || printf '  could not write that one\n'
done

sleep 1
tix task mv "$EPIC" doing >/dev/null 2>&1

printf '%s\n' "0" > "${SCENE_PLANNER_RC:-/tmp/planner.rc}"
printf '\n  %s: the backlog is written; anyone can take it from here\n' "$AGENT"
