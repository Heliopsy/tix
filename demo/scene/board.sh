#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# The pane that appears partway through: a terminal board, reached over SSH,
# on the same tenant the browser is showing and the agent is working.
#
# The username is `tix`, not a person. On this listener the username selects
# the tenant, and `tix` is the neutral one meaning "the only tenant this key is
# enrolled in". A handle there is read as a tenant key, does not resolve, and
# the connection is refused with the same sentence every other failure gets.
#
# It connects over SSH rather than running `tix tui` locally on purpose. The
# claim is that the terminal interface is a real remote surface, not a local
# program with a shared file, and an ssh line on camera says that in a way a
# local invocation cannot.
#
# The project is named as the command, which is what `tix tui -p` is locally.
# Without it the session lands on the project list, correctly: the scene's
# tenant holds the project the browser created and the two a new tenant comes
# with, and the listener will not guess between them. Early takes filmed this
# pane sitting on that list for their whole length, which is how the listener
# came to take the argument at all.

set -uo pipefail

# The selection pulse animates, which makes every captured frame differ from
# the last even when nothing changed. That costs the encoder a great deal and
# buys the viewer nothing.
export TIX_TUI_MOTION=off
export TIX_OUTPUT_COLOR=always

# -t is not optional once there is a command. ssh allocates a pseudo-terminal
# for an interactive session and does not for a command, and a session with no
# terminal is refused by the listener, which has nothing to draw on. The pane
# this runs in is a tty, so one -t is enough.
exec ssh \
  -t \
  -p 2222 \
  -i /home/demo/.ssh/demo_key \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o LogLevel=ERROR \
  tix@127.0.0.1 "${SCENE_PROJECT:?the board pane needs the project the scene created}"
