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

# Lease tokens are secrets, and the agents print them on every transition
# because the CLI takes one. This one is narrow in blast radius, scoped to a
# single task in a database thrown away with the container, but a video
# published on a marketing page should not carry a credential of any size, and
# a reader cannot tell a harmless one from a real one by looking.
#
# Every path that puts model-controlled text in the pane goes through here.
# The first version guarded only the tool calls, and the narration beside them
# is the same text written by the same agent, which was told the token in its
# prompt and asked to say what it was doing.
#
# A token split across two text deltas still arrives in the clear: the filter
# sees one delta at a time and holds no state between them. Narrowing the gap
# further means buffering the stream, which is the thing this file exists not
# to do.
def redact:
  gsub("(?<f>--(lease-)?token)[ =]+(?<v>[^\\s]+)"; .f + " ********");

if .type == "stream_event"
   and .event.type == "content_block_delta"
   and .event.delta.type == "text_delta"
then
  # Deltas only, never truncated: the pane is fed this prose a few characters
  # at a time, and cutting a delta at 96 characters cuts a word in half.
  .event.delta.text | redact

elif .type == "assistant" then
  ( .message.content[]?
    | select(.type == "tool_use")
    | ( .input.command // .input.file_path // .name | tostring )
    # One line, and a short one. A file written through a heredoc carries the
    # whole file in its command, and an agent pane that prints forty lines of
    # Python is a pane nobody reads. The first clause is the part worth seeing:
    # which command was run, not what was in it.
    | gsub("\n"; " ") | gsub(" +"; " ")
    | redact
    | if length > 96 then .[0:96] + " ..." else . end
    | "\n  $ " + . + "\n"
  )

else empty
end
