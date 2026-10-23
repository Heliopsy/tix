#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Turns one take into the four artifacts that get published.
#
# Encoding is separate from recording on purpose. A take costs an agent run and
# cannot be repeated for free, so the capture is near-lossless and every
# quality decision is made here, against a file that can be re-encoded as many
# times as it takes.

set -euo pipefail

OUT=${1:-demo/out}
RAW="$OUT/raw.mkv"
[ -f "$RAW" ] || { echo "no take at $RAW; run just demo-record first" >&2; exit 1; }

# The GIF is a trimmed cut, not the whole thing. A full-length GIF of this
# scene is tens of megabytes, which is a bad trade on a page whose own
# screenshots were taken from 818KB to 30KB to avoid exactly that.
: "${GIF_START:=0}"
: "${GIF_SECONDS:=15}"
: "${GIF_WIDTH:=800}"
: "${POSTER_AT:=}"
# SPEED compresses the whole take. Real agents think, and thinking is not
# interesting to watch; 1.5 to 2 keeps every beat legible while cutting the
# dead air. It is applied here rather than to the capture, because a take costs
# an agent run and the recording should stay faithful to what happened.
: "${SPEED:=1}"

SPEED_FILTER=""
if [ "$SPEED" != "1" ]; then
  SPEED_FILTER="setpts=PTS/${SPEED},"
  echo "speeding up ${SPEED}x"
fi

DUR=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$RAW" | cut -d. -f1)
echo "take is ${DUR}s"

# Poster: the last held frame by default, which beat 6 exists to provide. It is
# the frame behind the play button, so it should show the finished board rather
# than an empty one.
if [ -z "$POSTER_AT" ]; then POSTER_AT=$((DUR > 3 ? DUR - 3 : 0)); fi
echo "poster from ${POSTER_AT}s"
ffmpeg -nostdin -loglevel error -y -ss "$POSTER_AT" -i "$RAW" -frames:v 1 "$OUT/demo-poster.png"

echo "mp4"
ffmpeg -nostdin -loglevel error -y -i "$RAW" \
  -vf "${SPEED_FILTER}null" -c:v libx264 -preset slow -crf 24 -pix_fmt yuv420p \
  -movflags +faststart -an "$OUT/demo.mp4"

echo "webm"
ffmpeg -nostdin -loglevel error -y -i "$RAW" \
  -vf "${SPEED_FILTER}null" -c:v libvpx-vp9 -crf 34 -b:v 0 -row-mt 1 -pix_fmt yuv420p -an "$OUT/demo.webm"

echo "gif (${GIF_SECONDS}s from ${GIF_START}s, ${GIF_WIDTH}px)"
# Two passes. One global palette for the whole cut rather than per-frame
# palettes: a terminal is mostly flat colour, and a per-frame palette makes the
# background shimmer between frames.
ffmpeg -nostdin -loglevel error -y -ss "$GIF_START" -t "$GIF_SECONDS" -i "$RAW" \
  -vf "${SPEED_FILTER}fps=10,scale=${GIF_WIDTH}:-1:flags=lanczos,palettegen=max_colors=128:stats_mode=diff" \
  "$OUT/palette.png"
ffmpeg -nostdin -loglevel error -y -ss "$GIF_START" -t "$GIF_SECONDS" -i "$RAW" -i "$OUT/palette.png" \
  -lavfi "${SPEED_FILTER}fps=10,scale=${GIF_WIDTH}:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=3" \
  "$OUT/demo.gif"
rm -f "$OUT/palette.png"

printf '\n'
for f in demo.mp4 demo.webm demo.gif demo-poster.png; do
  [ -f "$OUT/$f" ] && printf '  %-18s %s\n' "$f" "$(du -h "$OUT/$f" | cut -f1)"
done
