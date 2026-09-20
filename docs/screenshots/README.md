# Screenshots

Captured against a seeded demo database on the commit that added them.

| File | Surface |
| --- | --- |
| [cli-task-list.png](cli-task-list.png) | `tix task ls` |
| [cli-task-show.png](cli-task-show.png) | `tix task show` |
| [cli-agent.png](cli-agent.png) | `tix claim next -o json` and NDJSON piped through `jq`, the agent path |
| [tui-board.png](tui-board.png) | `tix tui`, the workflow board |
| [web-task-list.png](web-task-list.png) | Browser task list |
| [web-task-detail.png](web-task-detail.png) | Browser task detail |
| [web-bundles-dark.png](web-bundles-dark.png) | Component sharing, dark scheme |

Regenerating them needs a seeded database and a running `tix serve`; the
terminal images were produced with `termshot`, the browser images with Chrome
at a 1400x900 viewport and a device pixel ratio of 2.
