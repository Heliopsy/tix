#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# The event stream, as it happens.
#
# Everything else in the frame is a consequence of this: the board repaints
# because of these lines, the terminal board redraws because of these lines,
# and a webhook would fire from them too. Showing the stream itself is what
# turns "the screens update" into "there is one log and every surface reads it".
#
# It follows rather than polls, and it is the same command any operator or
# script would run.
#
# The stream carries no lease token of its own: no event or audit payload holds
# one. What it does carry is agent-authored free text, because a release writes
# a comment, and the agent that writes it was handed a token in its prompt. The
# agent panes have been filtered since the first take; this pane was not, so a
# token pasted into a comment reached the camera with nothing in the way.
#
# Two rules, the same pair the agent panes use. The first catches the flag form.
# The second catches a bare one: a lease token is 32 bytes of crypto/rand as raw
# base64url, so 43 characters of that alphabet in a row is a credential and not
# prose. Task refs and ULIDs are far shorter, so nothing in this scene trips it.

set -uo pipefail

export TIX_DATABASE_DSN=${DB:-/home/demo/data/demo.db}
export TIX_OUTPUT_COLOR=always

clear
tix --no-discovery watch --type 'task.*' \
  | sed -u -E -e 's/(--(lease-)?token)[ =]+[^[:space:]]+/\1 ********/g' \
               -e 's/[A-Za-z0-9_-]{43,}/********/g'
