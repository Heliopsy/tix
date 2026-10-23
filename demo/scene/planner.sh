#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# The planner. One agent, given one task too big to do in a sitting, that
# breaks it into tasks small enough for other agents to take.
#
# This is the argument for tix rather than a long agent loop. A loop holds its
# plan in a context window: nothing else can see it, nothing else can help with
# it, and when the loop ends the plan is gone. Written into tix the plan is
# rows, so a second agent can start on step three while the first is still on
# step one, a human can reorder it in the browser, and a crash loses nothing
# because the work was never in the process to begin with.
#
# It writes the breakdown and stops. It does not do any of the work: the whole
# point is that the work is now available to whoever takes it next.

set -uo pipefail

AGENT=${TIX_AGENT:?an agent name is required}
TOKEN_FILE="/home/demo/.$AGENT.token.json"
PROJECT=${SCENE_PROJECT:-links}
EPIC=${SCENE_EPIC:?the ref of the task to break down is required}

export TIX_SERVER_URL="http://127.0.0.1:8080"
TIX_TOKEN=$(jq -r '.secret // .token // empty' "$TOKEN_FILE" 2>/dev/null)
export TIX_TOKEN
export TIX_OUTPUT_COLOR=always

mkdir -p "/home/demo/work/$AGENT"
cd "/home/demo/work/$AGENT" || exit 1

clear
printf '  %s  breaking the work down\n\n' "$AGENT"

read -r -d '' PROMPT <<PROMPT_END || true
You are an agent called $AGENT working in a tix task tracker. The \`tix\` CLI is
already pointed at the right server and carries your own token.

Task $EPIC in project $PROJECT describes a piece of work that is too big to do
in one go. Read it:

    tix task show $EPIC -o json

Break it into 5 or 6 subtasks that are each small enough for one agent to
finish on its own, and write each one into tix as a task in project $PROJECT:

    tix task add "<title>" -p $PROJECT --parent $EPIC

Say each title in one short line as you add it. Where one subtask genuinely
cannot start until another is finished, record that:

    tix dep add <ref> <the-ref-it-depends-on>

Do not do any of the work yourself. When the breakdown is written, move the
parent along and stop:

    tix task mv $EPIC doing

If a task cannot be finished because it needs a decision that is not yours to
make, do not guess and do not sit on it. Put the question on the task and give
it back:

    tix comment add <ref> "Blocked: <the question, and why it changes the work>"
    tix claim release <ref> --token <lease_token> --status blocked

Then carry on with the next task. A guessed answer is work that has to be
redone; a held lease is a task nobody else can take.

Keep your narration to one line per subtask. Do not ask questions.
PROMPT_END

claude -p "$PROMPT" \
  --allowedTools 'Bash(tix:*)' Read \
  --permission-mode acceptEdits

printf '\n  %s: the backlog is written; anyone can take it from here\n' "$AGENT"
