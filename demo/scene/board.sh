#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# The pane that appears partway through: a terminal board, reached over SSH,
# on the same tenant the browser is showing and the agent is working.
#
# It connects over SSH rather than running `tix tui` locally on purpose. The
# claim is that the terminal interface is a real remote surface, not a local
# program with a shared file, and an ssh line on camera says that in a way a
# local invocation cannot.

set -uo pipefail

# The selection pulse animates, which makes every captured frame differ from
# the last even when nothing changed. That costs the encoder a great deal and
# buys the viewer nothing.
export TIX_TUI_MOTION=off
export TIX_OUTPUT_COLOR=always

exec ssh \
  -p 2222 \
  -i /home/demo/.ssh/demo_key \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o LogLevel=ERROR \
  nadia@127.0.0.1
