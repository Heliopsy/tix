# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Renders Claude Code's stream-json into something worth filming.
#
# Without this an agent pane sits blank for the whole run and prints its
# narration in one block at the end: `claude -p` buffers its text output. In
# the first side-by-side take one agent had finished and showed a paragraph
# while the other, which had done most of the work, showed nothing at all. A
# pane that is blank while the event log beside it fills up reads as an agent
# that is broken, not one that is thinking.
#
# Text arrives as deltas, so it is printed as it comes. Tool calls are printed
# from the assistant message instead, because the arguments arrive as partial
# JSON that cannot be parsed until it is complete. Showing the command matters:
# a viewer should see `tix claim next` being run, not just its consequences.

if .type == "stream_event"
   and .event.type == "content_block_delta"
   and .event.delta.type == "text_delta"
then
  .event.delta.text

elif .type == "assistant" then
  ( .message.content[]?
    | select(.type == "tool_use")
    | ( .input.command // .input.file_path // .name | tostring )
    # One line, and a short one. A file written through a heredoc carries the
    # whole file in its command, and an agent pane that prints forty lines of
    # Python is a pane nobody reads. The first clause is the part worth seeing:
    # which command was run, not what was in it.
    | gsub("\n"; " ") | gsub(" +"; " ")
    # Lease tokens are secrets, and the agents print them on every transition
    # because the CLI takes one. This one is narrow in blast radius, scoped to
    # a single task in a database thrown away with the container, but a video
    # published on a marketing page should not carry a credential of any size,
    # and a reader cannot tell a harmless one from a real one by looking.
    | gsub("(?<f>--(lease-)?token) +(?<v>[^ ]+)"; .f + " ********")
    | if length > 96 then .[0:96] + " ..." else . end
    | "\n  $ " + . + "\n"
  )

else empty
end
