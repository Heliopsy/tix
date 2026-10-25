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

    tix task add "<title>" -p $PROJECT --parent $EPIC --priority <high|normal|low> --tag <area>

Give every subtask a priority and one tag naming the area it touches, such as
api, storage, docs or health. They are what the board is read by: a column of
titles is a list, and the same column with a priority and an area on each card
is a plan somebody can pick from.

Say each title in one short line as you add it.

Most of the subtasks should be independent of each other, so that several
agents can work on them at the same time. Add a dependency only where one
subtask truly cannot begin until another is finished, and add at most two in
total. A chain where everything waits on the first task leaves every agent but
one with nothing to do.

Where a dependency is genuinely needed, record it:

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

# Wrapped at spaces rather than at the column. The pane is about fifty columns
# once two agents share the width, and letting the terminal hard-wrap split
# words down the middle: "moving it to d / oing", "a minimal versio / n."
# fold -s breaks at the last blank instead, and stdbuf keeps it flushing as the
# text arrives rather than at the end, which is the whole point of streaming it.
#
# The width is fixed rather than read from the terminal. tput cols answers for
# the pane as it is when this starts, and the pane is split in half a moment
# later, so the folded lines came out wider than the pane and the terminal
# hard-wrapped them mid-word again: "stora / ge", "wi / th". The scene sets
# SCENE_FOLD from the layout it built, because the number is a property of the
# pane and not of this script: 50 was half of a 110 column window, and the
# agents now get a third of a full width one.
#
# The comment belongs here and not inside the command below. A comment between
# a line ending in a backslash and the line it continues splices into the
# command: `--verbose \` + `# ...` becomes `--verbose # ...`, which ends the
# command, and `2>/dev/null | jq ...` then starts a new one reading from the
# terminal. bash -n accepts all of it. That shipped once, and the cost was not
# the broken pipe but what the pipe was doing: redacting the lease tokens.
#
# stderr goes to a file, not to /dev/null. It cannot go to the pane, because
# the pane is the product demo, but dropping it cost a whole take: the OAuth
# session had expired, claude printed "Failed to authenticate" and exited in
# under a second, and the scene then waited six minutes for subtasks that
# could never arrive. The pane showed nothing, the log showed nothing, and the
# only record of why was in the stream that was being discarded.
ERRLOG=${SCENE_AGENT_ERR:-/out/agent-$AGENT.err}
claude -p "$PROMPT" \
  --allowedTools 'Bash(tix:*)' Read \
  --permission-mode acceptEdits \
  --output-format stream-json --include-partial-messages --verbose \
  2>>"$ERRLOG" | jq -j --unbuffered -f "$(dirname "$0")/stream.jq" \
  | stdbuf -o0 fold -s -w "${SCENE_FOLD:-50}"

# The status of claude, not of fold. The scene waits on this: a planner that
# died is a failed take now rather than in six minutes.
printf '%s\n' "${PIPESTATUS[0]}" > "${SCENE_PLANNER_RC:-/tmp/planner.rc}"

printf '\n  %s: the backlog is written; anyone can take it from here\n' "$AGENT"
