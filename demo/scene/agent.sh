#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# One agent pane. A real Claude Code process, given the tix skill and a share
# of one backlog, with no special pleading: it reads the same skill a user
# would install and drives the same binary.
#
# Two of these run side by side against the same backlog, and nothing
# coordinates them. That is the thing worth filming. A claim is a
# compare-and-swap, so two agents cannot take the same task however closely
# they race, and neither needs to know the other exists. A lease means that if
# one dies holding a task, the task comes back by itself.
#
# It talks to the server with a scoped token rather than opening the database,
# because that is how an agent is actually deployed, and because the token
# scopes are then part of what the recording shows.

set -uo pipefail

AGENT=${TIX_AGENT:?an agent name is required}
TOKEN_FILE="/home/demo/.$AGENT.token.json"
PROJECT=${SCENE_PROJECT:-links}

export TIX_SERVER_URL="http://127.0.0.1:8080"
TIX_TOKEN=$(jq -r '.secret // .token // empty' "$TOKEN_FILE" 2>/dev/null)
export TIX_TOKEN
export TIX_OUTPUT_COLOR=always

# Its own directory, so two agents working one backlog cannot tread on each
# other's files and leave the viewer wondering which one broke it.
mkdir -p "/home/demo/work/$AGENT"
cd "/home/demo/work/$AGENT" || exit 1

clear
printf '  %s  working the shared backlog\n\n' "$AGENT"

if [ -z "$TIX_TOKEN" ]; then
  printf '  no token for %s; see the scene log\n' "$AGENT"
  exit 1
fi

read -r -d '' PROMPT <<PROMPT_END || true
You are an agent called $AGENT working in a tix task tracker. The \`tix\` CLI is
already pointed at the right server and carries your own token, so you never
need --db, --server or --token.

Another agent is working the same backlog at the same time. You do not
coordinate with it and you do not need to: a claim is atomic, so if a task is
already taken your claim simply returns nothing and you move on.

Loop until the backlog is empty:

1. \`tix claim next -p $PROJECT -s todo -o json\`. If it exits non-zero or says
   no task is available, the backlog is done: say so in one line and stop.
   The answer carries the task and a \`lease_token\`. Keep that token: a claimed
   task cannot be transitioned without it.
2. Say in one short line which task you took and what you are about to do.
3. \`tix task mv <ref> doing --lease-token <lease_token>\`.
4. Do the smallest honest version of the work, here in this directory.
5. \`tix claim release <ref> --token <lease_token> --status done --comment "<one line>"\`.

If a task cannot be finished because it needs a decision that is not yours to
make, do not guess and do not sit on it. Put the question on the task and give
it back:

    tix comment add <ref> "Blocked: <the question, and why it changes the work>"
    tix claim release <ref> --token <lease_token> --status blocked

Then carry on with the next task. A guessed answer is work that has to be
redone; a held lease is a task nobody else can take.

Keep your narration to one or two lines per task. Do not print file contents.
Do not ask questions; if a task is ambiguous, pick the obvious reading and say
which you picked.
PROMPT_END

# Streamed, not buffered. See demo/scene/stream.jq for why: `claude -p` holds
# its narration until the run ends, which leaves the pane blank while the
# agent works and prints a paragraph after it has stopped.
claude -p "$PROMPT" \
  --allowedTools 'Bash(tix:*)' 'Bash(ls:*)' 'Bash(cat:*)' 'Bash(python3:*)' Write Edit Read \
  --permission-mode acceptEdits \
  --output-format stream-json --include-partial-messages --verbose \
  2>/dev/null | jq -j --unbuffered -f "$(dirname "$0")/stream.jq"

printf '\n  %s: nothing left to take\n' "$AGENT"
