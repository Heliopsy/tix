#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# The agent pane. A real Claude Code process, given the tix skill and a slice
# of the backlog, with no special pleading: it reads the same skill a user
# would install and drives the same binary.
#
# It runs against the SQLite file directly rather than through the server. That
# is not a shortcut for the recording, it is the point being demonstrated: a
# direct-database write by a CLI shows up on every connected browser, because
# events are a persisted outbox rather than an in-memory bus.

set -uo pipefail

DB=${DB:-/home/demo/data/demo.db}
PROJECT=${SCENE_PROJECT:-web}
TASKS=${SCENE_TASKS:-4}

export TIX_DATABASE_DSN="$DB"
export TIX_OUTPUT_COLOR=always

# Keep the agent's own working files out of the data directory, so nothing it
# writes can be mistaken for something tix wrote.
mkdir -p /home/demo/work
cd /home/demo/work

clear

cat <<'BANNER'
  an agent, with the tix skill, working the backlog

BANNER

read -r -d '' PROMPT <<PROMPT_END || true
You are working in a tix task tracker. The CLI is \`tix\` and it is already
pointed at the right database, so you never need --db.

Work the backlog of project "$PROJECT", $TASKS tasks, like this:

1. \`tix claim next -p $PROJECT -o json\` to take one task.
2. Read what it asks for. Say in one short line what you are doing.
3. Do the smallest honest version of it in /home/demo/work.
4. \`tix task mv <ref> done --comment "<one line on what you did>"\`.
5. Repeat until you have completed $TASKS tasks.

Keep your narration to one or two lines per task. Do not print long file
contents. Do not ask questions; if a task is ambiguous, pick the obvious
reading and say which you picked.
PROMPT_END

claude -p "$PROMPT" \
  --allowedTools 'Bash(tix:*)' 'Bash(ls:*)' 'Bash(cat:*)' Write Edit Read \
  --permission-mode acceptEdits

printf '\n  backlog cleared\n'
