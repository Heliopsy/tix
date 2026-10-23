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

: "${SCENE_WIDTH:=1600}"
: "${SCENE_HEIGHT:=900}"
: "${SCENE_FPS:=12}"
: "${SCENE_PROJECT:=web}"
: "${SCENE_TASKS:=4}"
: "${SCENE_BEAT_TIMEOUT:=180}"

OUT=/out
DB_DIR=/home/demo/data
DB="$DB_DIR/demo.db"
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
wait_event() {
  local type=$1 count=${2:-1} secs=${3:-$SCENE_BEAT_TIMEOUT}
  say "waiting for ${count}x ${type} (<= ${secs}s)"
  if timeout "$secs" tix --db "$DB" --no-discovery \
      watch --type "$type" --limit "$count" -o ndjson >> "$LOG" 2>&1; then
    say "got ${count}x ${type}"
  else
    die "timed out waiting for ${count}x ${type}"
  fi
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
chromium \
  --no-sandbox --disable-gpu --disable-dev-shm-usage \
  --no-first-run --no-default-browser-check --disable-infobars \
  --disable-features=TranslateUI,AutofillServerCommunication \
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

say "beat 3: agent"
xterm -geometry 100x44+$((SCENE_WIDTH * 2 / 3))+0 -e \
  "tmux new-session -s demo -n work '$SCENE/agent.sh; bash'" \
  >> "$LOG" 2>&1 &
hold 3
wait_event 'task.claimed' 1

# ── beat 4: a second surface on the same tenant ──────────────────────────────

say "beat 4: ssh pane"
hold 4
tmux split-window -t demo:work -v "$SCENE/board.sh" >> "$LOG" 2>&1 || die "splitting the pane"
hold 6

# ── beat 5: watch the work land ──────────────────────────────────────────────

say "beat 5: work"
wait_event 'task.updated' "$SCENE_TASKS" $((SCENE_BEAT_TIMEOUT * 3))
hold 6

# ── beat 6: the frame the poster comes from ──────────────────────────────────

say "beat 6: hold"
hold 4

say "done"
