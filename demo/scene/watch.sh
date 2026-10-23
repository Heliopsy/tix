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

set -uo pipefail

export TIX_DATABASE_DSN=${DB:-/home/demo/data/demo.db}
export TIX_OUTPUT_COLOR=always

clear
exec tix --no-discovery watch --type 'task.*'
