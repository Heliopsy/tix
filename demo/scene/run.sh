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

: "${SCENE_WIDTH:=1600}"
: "${SCENE_HEIGHT:=900}"
: "${SCENE_FPS:=12}"
: "${SCENE_PROJECT:=toolbelt}"
: "${SCENE_TASKS:=4}"
: "${SCENE_BEAT_TIMEOUT:=180}"

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
    --type task.claimed --type task.released --type task.transitioned \
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

xrdb -merge /home/demo/.Xresources 2>/dev/null || true
i3 >> "$LOG" 2>&1 &
sleep 1

cleanup() {
  say "cleanup"
  [ -n "${TAP_PID:-}" ] && kill "$TAP_PID" 2>/dev/null || true
  [ -n "${FFMPEG_PID:-}" ] && kill -INT "$FFMPEG_PID" 2>/dev/null || true
  [ -n "${FFMPEG_PID:-}" ] && wait "$FFMPEG_PID" 2>/dev/null || true
  [ -n "${SERVE_PID:-}" ] && kill "$SERVE_PID" 2>/dev/null || true
  [ -n "${XVFB_PID:-}" ] && kill "$XVFB_PID" 2>/dev/null || true
}
trap cleanup EXIT

# ── the product ──────────────────────────────────────────────────────────────

say "seeding"
tix --db "$DB" --no-discovery demo seed --days 30 >> "$LOG" 2>&1 \
  || die "demo seed"

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

say "recording to $OUT/raw.mkv"
# Lossless-ish capture. Every encode decision belongs in encode.sh against a
# finished take, not here against a take that cannot be redone without paying
# for the agent again.
ffmpeg -nostdin -loglevel error -y \
  -f x11grab -framerate "$SCENE_FPS" -video_size "${SCENE_WIDTH}x${SCENE_HEIGHT}" -i :99 \
  -c:v libx264 -preset ultrafast -qp 0 -pix_fmt yuv444p \
  "$OUT/raw.mkv" &
FFMPEG_PID=$!
sleep 1

# ── beat 1: the browser ──────────────────────────────────────────────────────

say "beat 1: browser"
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
SCENE_PROJECT="$SCENE_PROJECT" SCENE_TASKS="$SCENE_TASKS" \
  node "$SCENE/browser.mjs" plan >> "$LOG" 2>&1 || die "browser planning"

# ── beat 3: the agent ────────────────────────────────────────────────────────

tap_start

say "beat 3: agent"
# SCENE_AGENT exists so the scene can be debugged without paying for an agent
# run. agent-stub.sh drives the same tix commands on a timer, so every beat
# after this one is exercised exactly as it would be with the real thing.
AGENT=${SCENE_AGENT:-$SCENE/agent.sh}
xterm -geometry 100x44+$((SCENE_WIDTH * 2 / 3))+0 -e \
  "tmux -f /home/demo/scene/tmux.conf new-session -s demo -n work '$AGENT; bash'" \
  >> "$LOG" 2>&1 &
hold 1
# i3 splits evenly, which left the board in half a frame with three columns too
# narrow to read. The browser is the thing being watched, so it gets the room.
i3-msg '[class="Chromium"] focus' >> "$LOG" 2>&1 || true
i3-msg 'resize grow width 120 px' >> "$LOG" 2>&1 || true
hold 2
tap_wait 'task\.claimed' 1

# ── beat 4: a second surface on the same tenant ──────────────────────────────

say "beat 4: ssh pane"
hold 4
tmux split-window -t demo:work -v "$SCENE/board.sh" >> "$LOG" 2>&1 || die "splitting the pane"
hold 6

# ── beat 5: watch the work land ──────────────────────────────────────────────

say "beat 5: work"
tap_wait 'task\.released|task\.transitioned' "$SCENE_TASKS" $((SCENE_BEAT_TIMEOUT * 3))
hold 6

# ── beat 6: the frame the poster comes from ──────────────────────────────────

say "beat 6: hold"
hold 4

say "done"
