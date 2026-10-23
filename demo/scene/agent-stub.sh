#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# A stand-in for the real agent, so the scene can be rehearsed without paying
# for a Claude run. It drives the same commands the agent is asked to drive, so
# every beat downstream of it sees exactly the events it would see.
#
# This is a debugging tool and is never what gets filmed for publication: the
# whole point of the recording is that the agent is real.
#
# Note the lease token. A claimed task cannot be transitioned without it, which
# is the guard that stops a zombie worker writing over the task that was taken
# from it. Any agent, real or rehearsed, has to carry it, and the first version
# of this file did not, which is how the omission was found.

set -uo pipefail

export TIX_DATABASE_DSN=${DB:-/home/demo/data/demo.db}
export TIX_OUTPUT_COLOR=always
PROJECT=${SCENE_PROJECT:-web}
TASKS=${SCENE_TASKS:-4}

clear
printf '  a stand-in agent (rehearsal, not a real one)\n\n'

for _ in $(seq "$TASKS"); do
  sleep 2
  claimed=$(tix claim next -p "$PROJECT" -s todo -o json 2>/dev/null) || {
    printf '  nothing left to claim\n'; break; }

  ref=$(printf '%s' "$claimed" | jq -r '.task.ref')
  title=$(printf '%s' "$claimed" | jq -r '.task.title')
  token=$(printf '%s' "$claimed" | jq -r '.lease_token')
  printf '  claimed %s  %s\n' "$ref" "$title"

  # todo -> doing -> done. The default workflow has no transition straight to
  # done, which is the point of having a workflow, and a release that tries it
  # is refused. The first version of this file did try it, and the scene sat
  # waiting for completions that were never going to arrive.
  tix task mv "$ref" doing --lease-token "$token" >/dev/null 2>&1
  sleep 3
  if tix claim release "$ref" --token "$token" --status done \
       --comment "done by the rehearsal stand-in" >/dev/null 2>&1; then
    printf '  done    %s\n\n' "$ref"
  else
    printf '  could not release %s\n\n' "$ref"
  fi
done

printf '  backlog cleared\n'
