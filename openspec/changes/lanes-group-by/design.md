# Design: group lanes by

## The choice lives in the address

`lanes=project|assignee|priority|tag|due` sits beside `view=`. The view switch keeps every parameter it
does not drop, so it keeps `lanes` with no change; the filter form carries it as a hidden field the way
it carries `view`; Clear keeps it with the sort and the view; the pager names it among the parameters it
keeps. The live fragment and every row action's `next` are built from the request's own query, so they
keep it already.

Resolution: a valid `lanes` in the address wins. On the task list the remembered choice comes next,
then Project. On a project page the answer is Assignee: the page does not read the remembered view
either (#276), and a remembered Project would draw the one lane this change exists to remove. A
`lanes=project` on a project page is not a choice the page offers, so it falls back to Assignee.

The control is a GET form inside the view switch partial, so feel.js redraws it with the switch after
an in-place filter and its hidden fields carry what was just typed. Without scripting it has an Apply
button; with scripting a change submits it.

## What a lane is

| Grouping | Lane key | Order | Empty lane drawn when |
| --- | --- | --- | --- |
| project | project key | the screen's project order | a `project:` term names it |
| assignee | handle, `-` for Unassigned | named first, then A to Z, Unassigned last | an `assignee:` term names the person |
| priority | `highest` to `lowest` | highest first | a `priority:` term names it |
| tag | tag name, `-` for No tag | named first, then A to Z, No tag last | a `tag:` term names it |
| due | the My work buckets | overdue, today, week, later, none | never |

## Tags: one card, leftmost lane

A card is drawn once, in the leftmost drawn lane among its tags. Drawing it in every tag's lane would
repeat each card's element ids (the card, its four popovers, its fields), which the per-card htmx swap,
the focus return and the popovers all address by id, and would make the lane counts add up to more
than the page. Leftmost is decided after the reader's arrangement, so starring a tag's lane pulls its
tasks into it and hiding one moves them to their next tag. Tags a filter names come first, so
`tag:ops` puts every card in the ops lane. The card still lists all its tags, and a note over the lanes
says the rule. A tag lane that ends up holding nothing is not drawn unless the filter names it.

## Status is not a grouping

Lanes by status are the board. On one project they are its columns exactly; across projects they need
the board's workflow agreement and refusal, and the board already moves cards between states by drag.
Offering it would be a second, weaker board.

## Arrangement per grouping

The project lanes keep `lanes`, so nobody's arrangement moves. The others are `lanes:BY` views. The
service resolves their groups: an assignee is stored by actor id and shown as the handle, so renaming a
handle keeps the arrangement; a priority by name; a tag by name; a due bucket by its key; `-` is the
Unassigned or No tag lane. One arrangement serves the task list and every project page, because a
person or a priority is the same group on both.

## Drag and drop

Lane cards are not draggable today, so no drop is added. Reassigning stays in the card's More menu.
