#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# The scene. Runs inside the recording container and nowhere else.
#
# Every beat ends on a real committed event rather than on a sleep. `tix watch
# --limit N` blocks until N matching events commit and then exits, so the scene
# advances when the thing it is waiting for has actually happened. A sleep would
# either cut the agent off mid-thought or leave dead air, and which one you got
# would depend on the machine.
#
# The agent is a real Claude Code process, so the content of each beat differs
# every run. The beats do not. That is the whole of what "reproducible" can mean
# with a real agent in the frame.

set -euo pipefail

# Exported, not just defaulted. The agent and the board run in an xterm the
# scene starts, and anything not exported does not reach them: the first take
# with a project of its own had the agent working the fixture's backlog
# instead, because the default in its own script was still `web`.
export SCENE_PROJECT SCENE_TASKS DB

: "${SCENE_WIDTH:=1920}"
: "${SCENE_HEIGHT:=1080}"
: "${SCENE_FPS:=12}"
: "${SCENE_PROJECT:=links}"
: "${SCENE_TASKS:=6}"
# The lower bound on finished work, not the end condition. The scene ends when
# the backlog drains (see beat 5); this only guards the opposite failure, a
# take that reaches an empty board because the agents did nothing.
: "${SCENE_MIN_DONE:=2}"
export SCENE_MIN_DONE
: "${SCENE_BEAT_TIMEOUT:=180}"
: "${DEMO_PASSWORD:=tix-demo-password}"
: "${SCENE_AGENTS:=atlas scout}"
export SCENE_AGENTS
export DEMO_PASSWORD

OUT=/out
DB_DIR=/home/demo/data
DB="$DB_DIR/demo.db"
export DB
SCENE=/home/demo/scene
LOG=$OUT/scene.log

mkdir -p "$DB_DIR" "$OUT"
: > "$LOG"

# say writes to the scene log, never to the terminal being filmed. Anything
# printed on the recorded display is part of the product demo; progress
# chatter is not.
say() { printf '[scene] %s\n' "$*" >> "$LOG"; }

die() { say "FAILED: $*"; echo "scene failed: $*" >&2; exit 1; }

# wait_event blocks until COUNT events of TYPE commit, or gives up. It is the
# replacement for every sleep in this file. A timeout is a failed take, not a
# shorter one, because a take that silently skipped a beat looks like the
# product not working.
# The work tap. `tix watch` delivers events from the moment it subscribes, so a
# watcher started in beat 5 cannot see what the agent finished during beat 4,
# and the scene would sit waiting for completions that already happened. One
# subscriber is therefore opened before the agent starts and left running; the
# beats read its file rather than opening a subscription each.
tap_start() {
  TAP=$OUT/events.ndjson
  : > "$TAP"
  tix --db "$DB" --no-discovery watch \
    --type task.created --type task.claimed --type task.released \
    --type task.transitioned \
    -o ndjson >> "$TAP" 2>>"$LOG" &
  TAP_PID=$!
  sleep 1
  say "event tap open"
}

# tap_wait blocks until COUNT events of the given kinds have appeared in the
# tap, or gives up. A timeout is a failed take rather than a shorter one: a
# take that quietly skipped a beat looks like the product not working.
tap_wait() {
  local pattern=$1 count=${2:-1} secs=${3:-$SCENE_BEAT_TIMEOUT}
  local waited=0 seen=0
  say "waiting for ${count}x ${pattern} (<= ${secs}s)"
  while [ "$waited" -lt "$secs" ]; do
    seen=$(grep -cE "\"type\":\"(${pattern})\"" "$TAP" 2>/dev/null || true)
    [ "${seen:-0}" -ge "$count" ] && { say "got ${seen}x ${pattern}"; return 0; }
    sleep 1
    waited=$((waited + 1))
  done
  die "timed out with ${seen:-0}/${count} ${pattern}"
}

# hold is the one legitimate sleep: dwell time on a frame a viewer needs to
# read. It is never used to guess how long work takes.
hold() { sleep "$1"; }

# ── display ──────────────────────────────────────────────────────────────────

say "starting Xvfb ${SCENE_WIDTH}x${SCENE_HEIGHT}"
Xvfb :99 -screen 0 "${SCENE_WIDTH}x${SCENE_HEIGHT}x24" -nolisten tcp &
XVFB_PID=$!
for _ in $(seq 50); do xdpyinfo -display :99 >/dev/null 2>&1 && break; sleep 0.2; done
xdpyinfo -display :99 >/dev/null 2>&1 || die "Xvfb never came up"

# Not optional, and not silenced. The first takes had a white terminal beside
# a dark board because xrdb was missing from the image and the failure was
# swallowed by a `|| true`.
xrdb -merge /home/demo/.Xresources >> "$LOG" 2>&1 || die "xrdb could not load the terminal colours"

# The root window is what shows through the gaps between windows, and X leaves
# it a grey stipple. A flat colour a shade darker than the window chrome makes
# the gaps read as space between panels rather than as a hole in the capture.
xsetroot -solid '#070a0e' >> "$LOG" 2>&1 || die "could not set the root window colour"

# SCENE_VNC turns on a viewer for the display being filmed, so a take can be
# watched while it runs instead of only after it is encoded. It is off by
# default: the recording environment should expose nothing it does not need,
# and a rehearsal that nobody is watching has no reason to open a port.
if [ -n "${SCENE_VNC:-}" ]; then
  x11vnc -display :99 -forever -shared -nopw -quiet -rfbport 5900 >> "$LOG" 2>&1 &
  say "vnc on 5900"
fi
i3 >> "$LOG" 2>&1 &
sleep 1

cleanup() {
  say "cleanup"
  [ -n "${BROWSER_PID:-}" ] && kill "$BROWSER_PID" 2>/dev/null || true
  [ -n "${TAP_PID:-}" ] && kill "$TAP_PID" 2>/dev/null || true
  [ -n "${FFMPEG_PID:-}" ] && kill -INT "$FFMPEG_PID" 2>/dev/null || true
  [ -n "${FFMPEG_PID:-}" ] && wait "$FFMPEG_PID" 2>/dev/null || true
  [ -n "${SERVE_PID:-}" ] && kill "$SERVE_PID" 2>/dev/null || true
  [ -n "${XVFB_PID:-}" ] && kill "$XVFB_PID" 2>/dev/null || true
}
trap cleanup EXIT

# ── the product ──────────────────────────────────────────────────────────────

# A fresh installation, not a seeded one.
#
# `tix demo seed` writes six projects and sixty-six tasks of history. That is
# the right fixture for exploring the product and the wrong one for this
# recording: the scene says "here is a new installation, watch us set it up",
# and a backlog that predates the installation contradicts the first sentence.
# It also buried the four tasks the demo is about among sixty-six it is not.
#
# What remains is what a real `tix` creates for itself on first run.
say "creating the first account"
tix --db "$DB" --no-discovery user create nadia@demo.invalid \
  --handle nadia --display-name "Nadia Petrova" \
  --password "$DEMO_PASSWORD" --role admin >> "$LOG" 2>&1 \
  || die "creating the first account"

# An account and a scoped token per agent.
#
# Two agents, not one, because one agent working a list demonstrates nothing a
# shell loop could not do. Two agents on one backlog is the claim: the claim is
# a compare-and-swap, so they cannot take the same task, and nothing coordinates
# them. Each gets its own identity so the board shows who holds what.
#
# They run against the server with a token rather than against the database
# file, which is how an agent is actually deployed, and it exercises the token
# scopes at the same time.
for agent in $SCENE_AGENTS; do
  tix --db "$DB" --no-discovery user create "$agent@demo.invalid" \
    --handle "$agent" --display-name "$agent" \
    --password "$DEMO_PASSWORD" --role member >> "$LOG" 2>&1 \
    || die "creating the account for $agent"
done

# An SSH key for the pane that appears in beat 4. Generated here and thrown
# away with the container, so the recording never involves a key that exists
# anywhere else.
ssh-keygen -t ed25519 -N '' -q -f /home/demo/.ssh/demo_key <<< y >/dev/null 2>&1 || true
tix --db "$DB" --no-discovery user key add \
  --file /home/demo/.ssh/demo_key.pub --actor nadia --label recording \
  >> "$LOG" 2>&1 || die "enrolling the ssh key"

say "serving"
# The background workers are off. Each of them emits events on a timer, and an
# event the scene did not cause is an event wait_event might consume instead of
# the one it is waiting for.
tix --db "$DB" --no-discovery serve \
  --listen 127.0.0.1:8080 \
  --ssh-listen 127.0.0.1:2222 \
  --ssh-host-key "$DB_DIR/ssh_host_key" \
  --no-registry --no-webhook-dispatcher --no-retention-pruner \
  >> "$LOG" 2>&1 &
SERVE_PID=$!
for _ in $(seq 60); do
  curl -fsS http://127.0.0.1:8080/healthz >/dev/null 2>&1 && break; sleep 0.25
done
curl -fsS http://127.0.0.1:8080/healthz >/dev/null 2>&1 || die "server never became healthy"

# ── camera ───────────────────────────────────────────────────────────────────


# ── beat 1: the browser ──────────────────────────────────────────────────────

say "beat 1: browser"
# The three backgrounding flags are the reason the board updates at all. When
# the terminal window takes focus, chromium treats the occluded window as
# backgrounded and throttles its renderer: the socket stays open, the events
# arrive, and nothing is drawn. In the first takes the board sat on its
# first paint for the whole run while the agent emptied the backlog beside it,
# which made the one thing this recording exists to show the one thing it did
# not show.
#
# The command-line flags did not stop the "Save password?" bubble, which opened
# over the board in two takes. The profile preference does. It has to exist
# before chromium first reads the profile, so it is written here.
mkdir -p /home/demo/chrome/Default
cat > /home/demo/chrome/Default/Preferences <<'PREFS'
{"credentials_enable_service":false,
 "profile":{"password_manager_enabled":false,
            "password_manager_leak_detection":false,
            "default_content_setting_values":{"notifications":2}}}
PREFS

# --test-type is what actually removes the "unsupported command-line flag"
# infobar; --disable-infobars has not covered it for several versions, and that
# yellow strip sat across the top of the first take.
#
# The password manager is disabled three ways because one is not enough: a
# "Save password?" bubble opened over the board in the first take, right at the
# moment the board was the thing to look at.
chromium \
  --no-sandbox --disable-gpu --disable-dev-shm-usage \
  --no-first-run --no-default-browser-check --disable-infobars --test-type \
  --disable-features=TranslateUI,AutofillServerCommunication,PasswordManagerEnableAccountStore,PasswordLeakDetection \
  --password-store=basic --disable-save-password-bubble \
  --disable-backgrounding-occluded-windows \
  --disable-renderer-backgrounding \
  --disable-background-timer-throttling \
  --force-dark-mode \
  --remote-debugging-port=9222 \
  --user-data-dir=/home/demo/chrome \
  --window-position=0,0 \
  --window-size=$((SCENE_WIDTH * 2 / 3)),"$SCENE_HEIGHT" \
  --app=http://127.0.0.1:8080/login \
  >> "$LOG" 2>&1 &
for _ in $(seq 60); do
  curl -fsS http://127.0.0.1:9222/json/version >/dev/null 2>&1 && break; sleep 0.25
done
curl -fsS http://127.0.0.1:9222/json/version >/dev/null 2>&1 || die "chromium never exposed CDP"
hold 2

# ── beat 2: plan the work in the browser ─────────────────────────────────────

say "beat 2: planning"
# Started in the background and left running. It holds the CDP connection for
# the rest of the scene, because dropping it stops the page: see the note at
# the end of browser.mjs. It reports that the planning beat is finished by
# touching PLAN_DONE rather than by exiting.
PLAN_DONE=/tmp/plan.done
READY=/tmp/signed-in
GO=/tmp/rolling
EPIC_FILE=/tmp/epic.ref
DRAINED=/tmp/drained
rm -f "$PLAN_DONE" "$READY" "$GO" "$EPIC_FILE" "$DRAINED"
SCENE_PLAN_DONE="$PLAN_DONE" SCENE_READY="$READY" SCENE_GO="$GO" \
SCENE_EPIC_FILE="$EPIC_FILE" SCENE_DRAINED="$DRAINED" \
  node "$SCENE/browser.mjs" plan >> "$LOG" 2>&1 &
BROWSER_PID=$!

# Wait until it is signed in and parked, then roll, then release it. The
# recording therefore opens on the product rather than on somebody typing a
# password, which is not a feature and not worth eight seconds.
for _ in $(seq "$SCENE_BEAT_TIMEOUT"); do
  [ -f "$READY" ] && break
  kill -0 "$BROWSER_PID" 2>/dev/null || die "the browser died before signing in; see the log"
  sleep 1
done
[ -f "$READY" ] || die "the browser never signed in"

say "recording to $OUT/raw.mkv"
# Lossless-ish capture. Every encode decision belongs in encode.sh against a
# finished take, not here against a take that cannot be redone without paying
# for the agents again.
# -draw_mouse 0 because the pointer is not part of the product. Playwright
# drives chromium over CDP, which synthesises events without moving the X
# pointer, so it never goes anywhere: it sits where the server left it, in the
# middle of the screen, and earlier takes have an arrow resting on the board
# for their whole length.
ffmpeg -nostdin -loglevel error -y \
  -f x11grab -draw_mouse 0 -framerate "$SCENE_FPS" -video_size "${SCENE_WIDTH}x${SCENE_HEIGHT}" -i :99 \
  -c:v libx264 -preset ultrafast -qp 0 -pix_fmt yuv444p \
  "$OUT/raw.mkv" &
FFMPEG_PID=$!
sleep 2
touch "$GO"
for _ in $(seq $((SCENE_BEAT_TIMEOUT * 2))); do
  [ -f "$PLAN_DONE" ] && break
  kill -0 "$BROWSER_PID" 2>/dev/null || die "the planning beat died; see the log"
  sleep 1
done
[ -f "$PLAN_DONE" ] || die "the planning beat never finished"
say "planning done"

# ── beat 3: the agent ────────────────────────────────────────────────────────

# One scoped token each, written to a file the pane reads. The secret is shown
# exactly once by design, so it is captured here rather than re-derived.
for agent in $SCENE_AGENTS; do
  tix --db "$DB" --no-discovery token create "$agent-runner" --actor "$agent" \
      --expires never -o json \
      --scope task:read --scope task:write --scope task:transition \
      --scope task:claim --scope comment:write --scope project:read \
      --scope workflow:read --scope event:subscribe \
    > "/home/demo/.$agent.token.json" 2>>"$LOG" || die "minting a token for $agent"
done

tap_start

EPIC=$(cat "$EPIC_FILE" 2>/dev/null || true)
[ -n "$EPIC" ] || die "the planning beat never reported which task to break down"
export SCENE_EPIC="$EPIC"
say "the task to break down is $EPIC"

say "beat 2b: the event log"
# A window of its own, under the browser, rather than a pane stacked on the
# right. Four equal strips in one column is a wall of text; two columns of two
# reads as four things, and the log belongs beside the board it explains.
#
# It opens BEFORE the planner runs. tix watch delivers from the moment it
# subscribes, so opening it after the breakdown had been written meant the log
# began at sequence thirty-two and the most interesting thing in the take, six
# tasks appearing because an agent decided they should, had already scrolled
# past before the pane existed.
#
# The split is asked for on the browser, so i3 puts the new window below it and
# leaves the terminal column alone.
i3-msg '[class="Chromium"] focus' >> "$LOG" 2>&1 || true
i3-msg 'split v' >> "$LOG" 2>&1 || true
xterm \
  -bg '#0b0f14' -fg '#d7dce3' -cr '#2dd4bf' \
  -fa 'DejaVu Sans Mono' -fs 10 -b 10 +sb -bc \
  -T 'tix watch  (the event log)' \
  -e "$SCENE/watch.sh" >> "$LOG" 2>&1 &
hold 3
# The log needs a third of the column at most; the board is what is watched.
i3-msg '[title="tix watch.*"] focus' >> "$LOG" 2>&1 || true
i3-msg 'resize shrink height 180 px' >> "$LOG" 2>&1 || true
hold 1


# Put the terminal beside the browser-and-log column, not inside it.
#
# i3 places a new window against whatever is focused, and the log was focused
# because it had just been resized. The terminal therefore joined that column
# and three stacked panes came out four rows tall each, which is below the
# minimum the board needs to draw at all: it rendered "terminal is 206x4; tix
# tui needs at least 24x8" instead of a board. Focusing the browser and then
# its parent selects the column as a whole, so the split that follows puts the
# terminal next to it.
i3-msg '[class="Chromium"] focus' >> "$LOG" 2>&1 || true
i3-msg 'focus parent' >> "$LOG" 2>&1 || true
i3-msg 'split h' >> "$LOG" 2>&1 || true

say "beat 3: the planner"
# SCENE_AGENT exists so the scene can be debugged without paying for an agent
# run. agent-stub.sh drives the same tix commands on a timer, so every beat
# after this one is exercised exactly as it would be with the real thing.
AGENT=${SCENE_AGENT:-$SCENE/agent.sh}
# The planner writes its exit status here when it stops. See the wait below.
PLANNER_RC=/tmp/planner.rc
rm -f "$PLANNER_RC"
# Deliberately unquoted: SCENE_AGENTS is a space separated list of agent names
# and the split into positional parameters is the point.
# shellcheck disable=SC2086
set -- $SCENE_AGENTS
FIRST=$1
SECOND=${2:-}
# Colours on the command line, not from .Xresources. xrdb loads cleanly and
# xterm still came up white against a dark board, so the resource path is not
# worth another take to debug: -bg/-fg/-fa are unambiguous, and the sixteen
# palette entries come through -xrm, which is the same mechanism without the
# dependency on the resource database being consulted.
xterm \
  -bg '#0b0f14' -fg '#d7dce3' -cr '#2dd4bf' \
  -fa 'DejaVu Sans Mono' -fs 10 -b 8 +sb -bc \
  -xrm 'XTerm*color0:  #11161d' -xrm 'XTerm*color8:  #39414d' \
  -xrm 'XTerm*color1:  #f87171' -xrm 'XTerm*color9:  #fca5a5' \
  -xrm 'XTerm*color2:  #34d399' -xrm 'XTerm*color10: #6ee7b7' \
  -xrm 'XTerm*color3:  #fbbf24' -xrm 'XTerm*color11: #fcd34d' \
  -xrm 'XTerm*color4:  #60a5fa' -xrm 'XTerm*color12: #93c5fd' \
  -xrm 'XTerm*color5:  #a78bfa' -xrm 'XTerm*color13: #c4b5fd' \
  -xrm 'XTerm*color6:  #2dd4bf' -xrm 'XTerm*color14: #5eead4' \
  -xrm 'XTerm*color7:  #d7dce3' -xrm 'XTerm*color15: #f3f5f8' \
  -geometry 110x62+$((SCENE_WIDTH * 55 / 100))+0 \
  -T 'two agents, and the board over ssh' \
  -e "tmux -f /home/demo/scene/tmux.conf new-session -s demo -n work 'TIX_AGENT=$FIRST SCENE_EPIC=$EPIC SCENE_PLANNER_RC=$PLANNER_RC $SCENE/planner.sh; TIX_AGENT=$FIRST $AGENT; bash'" \
  >> "$LOG" 2>&1 &
hold 1
# i3 splits evenly, which left the board in half a frame with columns too
# narrow to read. The browser is the thing being watched, so it gets the room.
i3-msg '[class="Chromium"] focus' >> "$LOG" 2>&1 || true
# Explicit, not relative. Shrinking took the browser under 900px, which is
# where its own sidebar stacks into a row of wrapping links and spends the top
# of the frame on navigation. The terminal still gets enough width for two
# agents side by side.
i3-msg 'resize set width 54 ppt' >> "$LOG" 2>&1 || true

# The planner writes the backlog before anybody works it. Waiting on the
# subtasks appearing, rather than on a clock, is what keeps the next beat
# honest: the second agent arrives to a queue that exists.
#
# The planner stopping is also an answer, and the faster one. A take died with
# an expired OAuth session: claude printed its refusal to a discarded stderr
# and exited in under a second, and this wait then sat for six minutes for
# subtasks that nothing was going to write. The planner now records that it
# stopped, so the scene can fail at the moment the planner failed and say why.
planner_wait() {
  local secs=$1 waited=0 seen=0 rc
  say "waiting for 4x task.created (<= ${secs}s)"
  while [ "$waited" -lt "$secs" ]; do
    seen=$(grep -cE '"type":"task\.created"' "$TAP" 2>/dev/null || true)
    [ "${seen:-0}" -ge 4 ] && { say "got ${seen}x task.created"; return 0; }
    if [ -f "$PLANNER_RC" ]; then
      rc=$(cat "$PLANNER_RC" 2>/dev/null || echo "?")
      [ "$rc" = "0" ] \
        && die "the planner finished with only ${seen:-0} tasks written"
      die "the planner exited ${rc} with ${seen:-0} tasks written; see $OUT/agent-*.err"
    fi
    sleep 1
    waited=$((waited + 1))
  done
  die "timed out with ${seen:-0}/4 task.created"
}
planner_wait $((SCENE_BEAT_TIMEOUT * 2))
hold 3

# ── beat 4: a second surface on the same tenant ──────────────────────────────

say "beat 4: a second agent joins the queue"
# The second agent arrives after the first has already taken something, so the
# recording shows a backlog being shared rather than two processes starting
# together and racing once.
hold 3
if [ -n "$SECOND" ]; then
  # Side by side, not stacked. Two agents above one another read as one long
  # transcript; beside each other they read as two workers, which is the thing
  # being demonstrated, and each still gets its own labelled border.
  tmux split-window -t demo:work -h "TIX_AGENT=$SECOND $AGENT; bash" >> "$LOG" 2>&1 \
    || die "splitting for the second agent"
  hold 4
fi

say "beat 4b: the board over ssh"
# -f makes the new pane span the whole window rather than splitting whichever
# pane happens to be active, so the board gets the full width under both
# agents instead of half of one of them.
tmux split-window -f -v -t demo:work "$SCENE/board.sh" >> "$LOG" 2>&1 || die "splitting for the board"
# The board is the last pane split, so its index depends on whether the second
# agent took one. Hardcoding 2 filmed a one-agent take with the board neither
# sized nor labelled, because both commands addressed a pane that did not
# exist and both end in || true.
if [ -n "$SECOND" ]; then
  BOARD_PANE=2
else
  BOARD_PANE=1
fi
# The agents get the upper two thirds between them; the board needs less,
# because it is a list of five projects and not a transcript.
tmux resize-pane -t "demo:work.$BOARD_PANE" -y 30% >> "$LOG" 2>&1 || true

# Label every pane. Four unlabelled terminals stacked is four terminals; named,
# it is two agents, the event log and a board.
tmux select-pane -t demo:work.0 -T "agent: $FIRST" >> "$LOG" 2>&1 || true
if [ -n "$SECOND" ]; then
  tmux select-pane -t demo:work.1 -T "agent: $SECOND" >> "$LOG" 2>&1 || true
fi
tmux select-pane -t "demo:work.$BOARD_PANE" -T "ssh tix@demo  (the board)" >> "$LOG" 2>&1 || true
hold 5

# ── beat 5: watch the work land ──────────────────────────────────────────────

say "beat 5: work"
# The scene ends when the backlog drains, not after a fixed number of releases.
# A count cannot be the end condition here: the planner decides how many tasks
# to write, so a take that produced six tasks ended after four of them while
# two were still in Doing, and the board never scrolled to Done because its own
# condition is that To do and Doing are empty. The end of the scene and the
# payoff of the scene were two different states, and the scene reported success
# for a take that never reached the second one.
#
# So there is one state now, and the board is what reports it. Blocked tasks do
# not hold the scene open: a parked task is a thing worth filming, and it is
# neither To do nor Doing.
drain_wait() {
  local secs=$1 waited=0
  say "waiting for the backlog to drain (<= ${secs}s)"
  while [ "$waited" -lt "$secs" ]; do
    if [ -f "$DRAINED" ]; then
      say "backlog drained after ${waited}s"
      return 0
    fi
    kill -0 "$BROWSER_PID" 2>/dev/null || die "the board died before the backlog drained"
    sleep 1
    waited=$((waited + 1))
  done
  die "the backlog never drained; see the board heartbeats in this log"
}
drain_wait $((SCENE_BEAT_TIMEOUT * 5))

# The opposite failure: an empty backlog that nobody worked. The board scrolls
# on To do and Doing being empty and at least one card in Done, which one
# finished task satisfies, so the floor is checked here rather than there.
released=$(grep -cE '"type":"task\.released"' "$TAP" 2>/dev/null || true)
[ "${released:-0}" -ge "$SCENE_MIN_DONE" ] \
  || die "backlog drained with only ${released:-0} releases, under the floor of $SCENE_MIN_DONE"

# The scroll is smooth and the board is still repainting behind it. This is
# dwell on the end state, not a guess at how long work takes.
hold 8

# ── beat 6: the frame the poster comes from ──────────────────────────────────

say "beat 6: hold"
hold 4

say "done"
