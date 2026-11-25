# Waiting on other tasks, whole-selection counts, unpaged board columns

## Why

The task screen's heading said "5 blocked", counting tasks that depend on an unfinished task, right
next to a board column "Blocked (0)", which is a workflow state. People read the pair as a bug. The
cards that were waiting showed nothing at all, so there was no way to find them from the board.

The heading also counted only the page on screen, so "12 to go" on page 1 of 437 tasks was wrong, and
the board and lanes were cut by the same pager as the list: page 2 could show "Doing (0)" while every
doing task sat on page 1.

## What changes

- The heading's count reads "N waiting on other tasks" (one/other plural, Bulgarian too) and links to
  the same view filtered with `is:waiting-on`. `is:waiting-on` is a new alias of `is:blocked`, which
  stays accepted.
- Every board card, lane card and list row of a waiting task carries an hourglass mark, "waiting on
  AICP-12" or "waiting on 2 tasks", with all open dependencies in the tooltip and a link to the first.
- Tasks gain `waiting_on`, the references of their unfinished dependencies. `blocked` stays in JSON.
- The heading counts the whole filtered selection with a grouped count in the store, not the page.
- The board reads each column on its own, 50 cards a step, with the column's true total in its heading
  and a Show N more at its foot. The lanes are no longer paged: 200 a step with Show N more. The list
  keeps its pager.
- The TUI reads its board one column at a time, so a page never empties a column; the `!` marker means
  waiting on another task.
- The CLI table's BLOCKED column becomes WAITING and names the references.
