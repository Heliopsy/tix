# Design

## Profile

The Profile section is the browser's binding of `WhoAmI`. It sits on Settings rather than on a screen of
its own because Settings is already where the reader looks after their own account, and the account menu
reaches it. It reads only the actor `WhoAmI` returns plus `GetTenant` for the tenant's key and name, so it
shows nothing the API does not already tell the same reader. The sign-in method is what the request
carries: a browser session, or an API token by name. PR #237 adds the email, the session's method and the
memberships to whoami; the section takes them as rows when that lands.

## Actor screen

`GET /actors/{id}` resolves a handle or an identifier through `GetActor`, which returns naming fields only,
so the screen shows no authority of the actor it is about. Recent activity is the last twenty audit
entries by that actor through the same grouping the activity feed uses (`buildActivityView` with an
explicit filter and limit), drawn with the shared `activity-rows` block. A reader without `audit:read` is
told so in place of the list rather than refused the page.

## Subtask tree

`TaskTree` is asked for four levels. The first three are drawn; the fourth is never drawn and only tells
the screen which third-level nodes have children. Such a node carries a Show subtasks control that fetches
`GET /tasks/{ref}/tree`, the same block for that node, three levels deep again. Without scripting the
control is a link to that subtask's own screen. A tree too large to walk falls back to one level, which is
what the screen drew before. Collapsing is a button per node with `aria-expanded`, drawn hidden and shown
by `assets/tasktree.js`, so without scripting the tree is simply open; it is not a `<details>`, which
`TestNoPageActionOrSectionSitsInAnExpander` keeps for the listed toggles.

## Credential filter

The token and SSH key screens read `?actor=` and pass it to `ListTokens` and `ListSSHKeys` as given; the
service resolves a handle or an identifier and decides whether the reader may. The filter is a GET form
with the directory's handles as suggestions. An empty filter keeps today's listing.
